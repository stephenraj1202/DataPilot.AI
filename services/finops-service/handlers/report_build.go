package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/smtp"
	"strings"
	"time"
)

// ─── buildReport ─────────────────────────────────────────────────────────────

func (h *ReportHandler) buildReport(accountID, reportType string) (string, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	d := reportData{
		Now:        now,
		MonthLabel: now.Format("January 2006"),
	}

	// ── 0. Live sync — fetch latest data from AWS/Azure/GCP before building ──
	if h.Scheduler != nil {
		syncRows, err := h.DB.Query(
			`SELECT id FROM cloud_accounts WHERE account_id=? AND deleted_at IS NULL AND status='active'`,
			accountID,
		)
		if err == nil {
			var ids []string
			for syncRows.Next() {
				var id string
				if err2 := syncRows.Scan(&id); err2 == nil {
					ids = append(ids, id)
				}
			}
			_ = syncRows.Close()
			for _, id := range ids {
				if err := h.Scheduler.SyncOne(id); err != nil {
					log.Printf("[report] live sync failed for account %s: %v (using cached data)", id, err)
				}
			}
			// Refresh now after sync
			now = time.Now()
			monthStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			d.Now = now
		}
	}

	// ── 1. Load all cloud accounts for this tenant ──────────────────────────
	caRows, err := h.DB.Query(
		`SELECT id, provider, account_name, COALESCE(encrypted_credentials,''),
		        COALESCE(last_sync_at,'') as last_sync_at,
		        COALESCE(last_sync_status,'') as last_sync_status
		 FROM cloud_accounts
		 WHERE account_id=? AND deleted_at IS NULL
		 ORDER BY provider, account_name`,
		accountID,
	)
	if err != nil {
		return "", fmt.Errorf("cloud accounts: %w", err)
	}
	defer caRows.Close()

	type caRow struct {
		ID, Provider, Name, EncCreds, LastSync, LastSyncStatus string
	}
	var accounts []caRow
	for caRows.Next() {
		var r caRow
		if err := caRows.Scan(&r.ID, &r.Provider, &r.Name, &r.EncCreds, &r.LastSync, &r.LastSyncStatus); err == nil {
			accounts = append(accounts, r)
		}
	}
	_ = caRows.Close()

	// ── 2. Per-account data ─────────────────────────────────────────────────
	providerMap := map[string]*providerSummary{}
	totalDays := daysInMonth(now)
	elapsed := now.Day()

	for _, ca := range accounts {
		// MTD cost for this cloud account
		var mtd float64
		_ = h.DB.QueryRow(
			`SELECT COALESCE(SUM(cost_amount),0)
			 FROM cloud_costs
			 WHERE cloud_account_id=? AND date>=? AND date<=?`,
			ca.ID, monthStart.Format("2006-01-02"), now.Format("2006-01-02"),
		).Scan(&mtd)

		// Forecast
		forecast := mtd
		if elapsed > 0 && elapsed < totalDays {
			forecast = math.Round((mtd/float64(elapsed))*float64(totalDays)*100) / 100
		}

		// Per-service breakdown — fetch (service_name, region, resource_id) grouped by
		// (service_name, region) to get distinct billing resources per category.
		svcRows, err := h.DB.Query(
			`SELECT service_name, COALESCE(region,'global'), COALESCE(resource_id,''),
			        COALESCE(SUM(cost_amount),0) as total
			 FROM cloud_costs
			 WHERE cloud_account_id=? AND date>=? AND date<=?
			 GROUP BY service_name, region, resource_id
			 ORDER BY total DESC`,
			ca.ID, monthStart.Format("2006-01-02"), now.Format("2006-01-02"),
		)
		var svcRegionRows []svcRegionPair
		svcCostMap := map[string]float64{}
		if err == nil {
			for svcRows.Next() {
				var sr svcRegionPair
				if err2 := svcRows.Scan(&sr.Name, &sr.Region, &sr.ResourceID, &sr.Cost); err2 == nil {
					svcRegionRows = append(svcRegionRows, sr)
					svcCostMap[sr.Name] += sr.Cost
				}
			}
			_ = svcRows.Close()
		}
		var acctServices []reportService
		for name, cost := range svcCostMap {
			rs := reportService{Name: name, Provider: ca.Provider, Cost: cost}
			acctServices = append(acctServices, rs)
			d.Services = append(d.Services, rs)
		}

		// Region count — Azure stores "azure" as region, exclude it
		var regionCount int
		_ = h.DB.QueryRow(
			`SELECT COUNT(DISTINCT NULLIF(region,'azure'))
			 FROM cloud_costs
			 WHERE cloud_account_id=? AND date>=? AND date<=?`,
			ca.ID, monthStart.Format("2006-01-02"), now.Format("2006-01-02"),
		).Scan(&regionCount)

		syncOK := ca.LastSyncStatus == "success"

		// Decrypt credentials and fetch live resource counts for tiles
		var tiles []reportTile
		if ca.EncCreds != "" && h.AESKey != "" {
			if plainCreds, err := decrypt(ca.EncCreds, h.AESKey); err == nil {
				var credsMap map[string]string
				if err2 := json.Unmarshal([]byte(plainCreds), &credsMap); err2 == nil {
					tiles = buildTilesFromLiveCounts(ca.Provider, credsMap)
				} else {
					log.Printf("[report] creds unmarshal failed for %s: %v", ca.ID, err2)
				}
			} else {
				log.Printf("[report] creds decrypt failed for %s: %v", ca.ID, err)
			}
		}

		ag := reportAccountGroup{
			AccountName: ca.Name,
			Provider:    ca.Provider,
			LastSyncAt:  ca.LastSync,
			LastSyncOK:  syncOK,
			RegionCount: regionCount,
			MTDCost:     mtd,
			Forecast:    forecast,
			Tiles:       tiles,
			Services:    acctServices,
		}
		d.AccountGroups = append(d.AccountGroups, ag)

		// Accumulate provider summary
		ps, ok := providerMap[ca.Provider]
		if !ok {
			ps = &providerSummary{Name: ca.Provider}
			providerMap[ca.Provider] = ps
		}
		ps.Cost += mtd
		ps.Forecast += forecast
		d.TotalCost += mtd
	}

	// Flatten provider map to slice (ordered)
	for _, p := range []string{"aws", "azure", "gcp"} {
		if ps, ok := providerMap[p]; ok {
			d.Providers = append(d.Providers, *ps)
		}
	}
	// Forecast total
	if elapsed > 0 && elapsed < totalDays {
		d.Forecast = math.Round((d.TotalCost/float64(elapsed))*float64(totalDays)*100) / 100
	} else {
		d.Forecast = d.TotalCost
	}

	// ── 3. Daily costs (current month, all accounts) ────────────────────────
	dailyRows, err := h.DB.Query(
		`SELECT cc.date, COALESCE(SUM(cc.cost_amount),0)
		 FROM cloud_costs cc
		 JOIN cloud_accounts ca ON ca.id=cc.cloud_account_id
		 WHERE ca.account_id=? AND ca.deleted_at IS NULL
		   AND cc.date>=? AND cc.date<=?
		 GROUP BY cc.date ORDER BY cc.date ASC`,
		accountID, monthStart.Format("2006-01-02"), now.Format("2006-01-02"),
	)
	if err == nil {
		for dailyRows.Next() {
			var dc reportDailyCost
			if err2 := dailyRows.Scan(&dc.Date, &dc.Cost); err2 == nil {
				d.DailyCosts = append(d.DailyCosts, dc)
			}
		}
		_ = dailyRows.Close()
	}

	// ── 4. Anomalies (last 30 days) ─────────────────────────────────────────
	if reportType == "anomalies" || reportType == "full" {
		anomRows, err := h.DB.Query(
			`SELECT an.date, an.severity, an.actual_cost, an.baseline_cost, an.deviation_percentage,
			        ca.account_name
			 FROM cost_anomalies an
			 JOIN cloud_accounts ca ON ca.id=an.cloud_account_id
			 WHERE ca.account_id=? AND ca.deleted_at IS NULL
			   AND an.date>=DATE_SUB(CURDATE(), INTERVAL 30 DAY)
			 ORDER BY an.date DESC LIMIT 20`,
			accountID,
		)
		if err == nil {
			for anomRows.Next() {
				var a reportAnomaly
				if err2 := anomRows.Scan(&a.Date, &a.Severity, &a.Actual, &a.Baseline,
					&a.Deviation, &a.AccountName); err2 == nil {
					d.Anomalies = append(d.Anomalies, a)
				}
			}
			_ = anomRows.Close()
		}
	}

	// ── 5. Recommendations ──────────────────────────────────────────────────
	if reportType == "recommendations" || reportType == "full" {
		recRows, err := h.DB.Query(
			`SELECT r.recommendation_type, r.service_name,
			        COALESCE(r.description,''), COALESCE(r.potential_monthly_savings,0),
			        ca.account_name
			 FROM cost_recommendations r
			 JOIN cloud_accounts ca ON ca.id=r.cloud_account_id
			 WHERE ca.account_id=? AND ca.deleted_at IS NULL
			 ORDER BY r.potential_monthly_savings DESC LIMIT 20`,
			accountID,
		)
		if err == nil {
			for recRows.Next() {
				var r reportRec
				if err2 := recRows.Scan(&r.Type, &r.Service, &r.Desc, &r.Savings, &r.AccountName); err2 == nil {
					d.Recs = append(d.Recs, r)
					d.TotalSaves += r.Savings
				}
			}
			_ = recRows.Close()
		}
	}

	return buildHTMLReport(d, reportType), nil
}

// ─── svcRegionPair is used to pass (service_name, region, resource_id) rows to buildTiles ─

type svcRegionPair struct {
	Name, Region, ResourceID string
	Cost                     float64
}

// ─── buildTiles ───────────────────────────────────────────────────────────────
// Counts DISTINCT service names per category from the billing data.
// Each unique service name = 1 resource type being billed.

func buildTiles(provider string, rows []svcRegionPair, regionCount int) []reportTile {
	// Count distinct service names per category
	catServices := map[string]map[string]struct{}{}
	for _, r := range rows {
		cat := classifyServiceForTile(r.Name)
		if catServices[cat] == nil {
			catServices[cat] = map[string]struct{}{}
		}
		catServices[cat][r.Name] = struct{}{}
	}
	counts := map[string]int{}
	for cat, set := range catServices {
		counts[cat] = len(set)
	}

	// "running" = total distinct services with any cost (actively billed)
	totalActive := 0
	seen := map[string]struct{}{}
	for _, r := range rows {
		if r.Cost > 0 {
			seen[r.Name] = struct{}{}
		}
	}
	totalActive = len(seen)

	type tileDef struct {
		key, icon, label, color string
		useRunning              bool
	}

	var defs []tileDef
	switch provider {
	case "aws":
		defs = []tileDef{
			{"compute", "💻", "EC2", "#FF9900", false},
			{"running", "✅", "Running", "#22C55E", true},
			{"storage", "💾", "EBS / S3", "#3F8624", false},
			{"database", "🗃️", "RDS", "#8B5CF6", false},
			{"networking", "⚖️", "LBs", "#6366F1", false},
			{"containers", "📦", "EKS / ECS", "#06B6D4", false},
			{"serverless", "λ", "Lambda", "#EC4899", false},
			{"other", "📷", "Other", "#6B7280", false},
		}
	case "azure":
		defs = []tileDef{
			{"compute", "💻", "VMs", "#0078D4", false},
			{"running", "✅", "Running", "#22C55E", true},
			{"storage", "💾", "Blob / Disk", "#107C10", false},
			{"database", "🗃️", "SQL / Cosmos", "#8B5CF6", false},
			{"networking", "🌐", "VNet / LB", "#00B4D8", false},
			{"containers", "📦", "AKS / ACI", "#06B6D4", false},
			{"serverless", "λ", "Functions", "#EC4899", false},
			{"other", "📷", "Other", "#6B7280", false},
		}
	default: // gcp
		defs = []tileDef{
			{"compute", "💻", "Compute", "#4285F4", false},
			{"running", "✅", "Running", "#22C55E", true},
			{"storage", "💾", "Storage", "#34A853", false},
			{"database", "🗃️", "Database", "#8B5CF6", false},
			{"networking", "🌐", "Network", "#00B4D8", false},
			{"containers", "📦", "GKE", "#06B6D4", false},
			{"serverless", "λ", "Functions", "#EC4899", false},
			{"other", "📷", "Other", "#6B7280", false},
		}
	}

	var tiles []reportTile
	for _, def := range defs {
		count := 0
		if def.useRunning {
			count = totalActive
		} else {
			count = counts[def.key]
		}
		tiles = append(tiles, reportTile{Icon: def.icon, Label: def.label, Color: def.color, Count: count})
	}
	return tiles
}

// ─── classifyServiceForTile ──────────────────────────────────────────────────
// Mirrors the classify() closure in GetCloudAccountVMResources exactly.

func classifyServiceForTile(svc string) string {
	s := strings.ToLower(svc)
	switch {
	case strings.Contains(s, "ec2") || strings.Contains(s, "compute engine") ||
		strings.Contains(s, "virtual machine") || strings.Contains(s, "vm ") ||
		strings.Contains(s, "app service"):
		return "compute"
	case strings.Contains(s, "rds") || strings.Contains(s, "aurora") ||
		strings.Contains(s, "sql") || strings.Contains(s, "database") ||
		strings.Contains(s, "cloud sql") || strings.Contains(s, "cosmos"):
		return "database"
	case strings.Contains(s, "s3") || strings.Contains(s, "storage") ||
		strings.Contains(s, "blob") || strings.Contains(s, "gcs") ||
		strings.Contains(s, "glacier") || strings.Contains(s, "backup"):
		return "storage"
	case strings.Contains(s, "lambda") || strings.Contains(s, "function") ||
		strings.Contains(s, "cloud function") || strings.Contains(s, "cloud run"):
		return "serverless"
	case strings.Contains(s, "eks") || strings.Contains(s, "aks") ||
		strings.Contains(s, "gke") || strings.Contains(s, "kubernetes") ||
		strings.Contains(s, "container") || strings.Contains(s, "ecs"):
		return "containers"
	case strings.Contains(s, "cloudfront") || strings.Contains(s, "cdn") ||
		strings.Contains(s, "load balancer") || strings.Contains(s, "network") ||
		strings.Contains(s, "vpc") || strings.Contains(s, "dns") ||
		strings.Contains(s, "traffic manager"):
		return "networking"
	default:
		return "other"
	}
}

// ─── buildHTMLReport ─────────────────────────────────────────────────────────

func buildHTMLReport(d reportData, reportType string) string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html><html><head><meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>FinOps Report</title>
<style>
*{margin:0;padding:0;box-sizing:border-box;}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f0f4f8;color:#1a202c;}
.wrap{max-width:700px;margin:0 auto;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 24px rgba(0,0,0,.12);}
.header{background:linear-gradient(135deg,#1e3a5f 0%,#2563eb 60%,#3b82f6 100%);padding:36px 32px;color:#fff;}
.header h1{font-size:26px;font-weight:700;letter-spacing:-.5px;}
.header p{margin-top:6px;opacity:.85;font-size:14px;}
.header .meta{margin-top:16px;display:flex;gap:24px;flex-wrap:wrap;}
.header .meta span{background:rgba(255,255,255,.15);border-radius:20px;padding:4px 14px;font-size:13px;}
.section{padding:28px 32px;}
.section-title{font-size:13px;font-weight:700;text-transform:uppercase;letter-spacing:.8px;color:#64748b;margin-bottom:16px;}
.provider-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:14px;}
.provider-card{border-radius:10px;padding:18px 20px;color:#fff;}
.provider-card .name{font-size:12px;font-weight:600;text-transform:uppercase;opacity:.85;}
.provider-card .cost{font-size:26px;font-weight:700;margin:6px 0 2px;}
.provider-card .forecast{font-size:12px;opacity:.8;}
.aws-card{background:linear-gradient(135deg,#FF9900,#e67e00);}
.azure-card{background:linear-gradient(135deg,#0078D4,#005a9e);}
.gcp-card{background:linear-gradient(135deg,#34A853,#1e7e34);}
.other-card{background:linear-gradient(135deg,#6366f1,#4338ca);}
.total-banner{background:linear-gradient(135deg,#1e3a5f,#2563eb);color:#fff;border-radius:10px;padding:20px 24px;display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:12px;}
.total-banner .label{font-size:13px;opacity:.8;}
.total-banner .value{font-size:28px;font-weight:700;}
.total-banner .forecast-val{font-size:14px;opacity:.85;}
.account-block{border:1px solid #e2e8f0;border-radius:10px;margin-bottom:16px;overflow:hidden;}
.account-header{background:#f8fafc;padding:14px 18px;display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:8px;}
.account-header .acct-name{font-weight:600;font-size:15px;}
.account-header .badge{font-size:11px;padding:3px 10px;border-radius:12px;font-weight:600;}
.badge-aws{background:#fff3e0;color:#e65100;}
.badge-azure{background:#e3f2fd;color:#0d47a1;}
.badge-gcp{background:#e8f5e9;color:#1b5e20;}
.badge-ok{background:#dcfce7;color:#166534;}
.badge-err{background:#fee2e2;color:#991b1b;}
.tile-grid{display:flex;flex-wrap:wrap;gap:10px;padding:14px 18px;}
.tile{border-radius:8px;padding:10px 14px;min-width:90px;text-align:center;}
.tile .tile-icon{font-size:20px;}
.tile .tile-count{font-size:18px;font-weight:700;margin:2px 0;}
.tile .tile-label{font-size:11px;color:#64748b;}
.account-costs{padding:0 18px 14px;display:flex;gap:24px;flex-wrap:wrap;}
.account-costs .cost-item .label{font-size:11px;color:#64748b;}
.account-costs .cost-item .val{font-size:18px;font-weight:700;color:#1e3a5f;}
table{width:100%;border-collapse:collapse;font-size:13px;}
th{background:#f1f5f9;padding:10px 12px;text-align:left;font-weight:600;color:#475569;border-bottom:2px solid #e2e8f0;}
td{padding:9px 12px;border-bottom:1px solid #f1f5f9;color:#374151;}
tr:last-child td{border-bottom:none;}
.bar-wrap{background:#e2e8f0;border-radius:4px;height:6px;width:100px;display:inline-block;vertical-align:middle;}
.bar-fill{height:6px;border-radius:4px;background:linear-gradient(90deg,#2563eb,#3b82f6);}
.anomaly-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:12px;}
.anomaly-card{border-radius:8px;padding:14px 16px;border-left:4px solid;}
.sev-critical{border-color:#dc2626;background:#fef2f2;}
.sev-high{border-color:#ea580c;background:#fff7ed;}
.sev-medium{border-color:#d97706;background:#fffbeb;}
.sev-low{border-color:#65a30d;background:#f7fee7;}
.anomaly-card .sev-badge{font-size:11px;font-weight:700;text-transform:uppercase;padding:2px 8px;border-radius:10px;display:inline-block;margin-bottom:6px;}
.anomaly-card .anom-date{font-size:12px;color:#64748b;}
.anomaly-card .anom-cost{font-size:18px;font-weight:700;margin:4px 0;}
.anomaly-card .anom-dev{font-size:12px;}
.rec-list{display:flex;flex-direction:column;gap:10px;}
.rec-card{border:1px solid #e2e8f0;border-radius:8px;padding:14px 16px;display:flex;justify-content:space-between;align-items:flex-start;gap:12px;}
.rec-card .rec-info .rec-type{font-size:11px;font-weight:700;text-transform:uppercase;color:#64748b;}
.rec-card .rec-info .rec-svc{font-size:14px;font-weight:600;margin:2px 0;}
.rec-card .rec-info .rec-desc{font-size:12px;color:#64748b;}
.rec-card .rec-info .rec-acct{font-size:11px;color:#94a3b8;margin-top:4px;}
.rec-card .savings-badge{background:linear-gradient(135deg,#059669,#10b981);color:#fff;border-radius:8px;padding:8px 14px;text-align:center;white-space:nowrap;flex-shrink:0;}
.rec-card .savings-badge .save-label{font-size:10px;opacity:.85;}
.rec-card .savings-badge .save-val{font-size:16px;font-weight:700;}
.savings-total{background:linear-gradient(135deg,#059669,#10b981);color:#fff;border-radius:10px;padding:18px 24px;display:flex;justify-content:space-between;align-items:center;}
.savings-total .st-label{font-size:13px;opacity:.85;}
.savings-total .st-val{font-size:26px;font-weight:700;}
.footer{background:#1e293b;color:#94a3b8;padding:20px 32px;font-size:12px;text-align:center;}
.footer a{color:#60a5fa;text-decoration:none;}
.divider{height:1px;background:#e2e8f0;margin:0 32px;}
</style></head><body><div class="wrap">`)

	// ── Header ──
	fmt.Fprintf(&b, `<div class="header">
<h1>☁️ FinOps Report</h1>
<p>%s &nbsp;·&nbsp; %s</p>
<div class="meta">
  <span>📅 Generated %s</span>
  <span>📊 %s</span>
</div>
</div>`, d.MonthLabel, reportTypeLabel(reportType),
		d.Now.Format("Jan 2, 2006 15:04 UTC"),
		titleCase(reportType))

	// ── Total cost banner ──
	if reportType == "cost_summary" || reportType == "full" {
		b.WriteString(`<div class="section">`)
		fmt.Fprintf(&b, `<div class="total-banner">
<div><div class="label">Month-to-Date Spend</div><div class="value">$%.2f</div></div>
<div><div class="label">Projected Month-End</div><div class="forecast-val">$%.2f</div></div>
</div>`, d.TotalCost, d.Forecast)
		b.WriteString(`</div>`)
	}

	// ── Provider cards ──
	if len(d.Providers) > 0 && (reportType == "cost_summary" || reportType == "full") {
		b.WriteString(`<div class="section"><div class="section-title">By Cloud Provider</div><div class="provider-grid">`)
		for _, p := range d.Providers {
			cls := "other-card"
			switch p.Name {
			case "aws":
				cls = "aws-card"
			case "azure":
				cls = "azure-card"
			case "gcp":
				cls = "gcp-card"
			}
			fmt.Fprintf(&b, `<div class="provider-card %s">
<div class="name">%s</div>
<div class="cost">$%.2f</div>
<div class="forecast">Forecast: $%.2f</div>
</div>`, cls, strings.ToUpper(p.Name), p.Cost, p.Forecast)
		}
		b.WriteString(`</div></div><div class="divider"></div>`)
	}

	// ── Per-account resource tiles ──
	if len(d.AccountGroups) > 0 && (reportType == "cost_summary" || reportType == "full") {
		b.WriteString(`<div class="section"><div class="section-title">Cloud Accounts</div>`)
		for _, ag := range d.AccountGroups {
			syncBadge := `<span class="badge badge-ok">✓ Synced</span>`
			if !ag.LastSyncOK {
				syncBadge = `<span class="badge badge-err">✗ Sync Error</span>`
			}
			provBadge := fmt.Sprintf(`<span class="badge badge-%s">%s</span>`, ag.Provider, strings.ToUpper(ag.Provider))
			fmt.Fprintf(&b, `<div class="account-block">
<div class="account-header">
  <span class="acct-name">%s</span>
  <div style="display:flex;gap:6px;align-items:center;">%s %s</div>
</div>`, ag.AccountName, provBadge, syncBadge)

			// Cost row
			fmt.Fprintf(&b, `<div class="account-costs">
<div class="cost-item"><div class="label">MTD Cost</div><div class="val">$%.2f</div></div>
<div class="cost-item"><div class="label">Forecast</div><div class="val">$%.2f</div></div>
<div class="cost-item"><div class="label">Regions</div><div class="val">%d</div></div>
</div>`, ag.MTDCost, ag.Forecast, ag.RegionCount)

			// Tiles
			if len(ag.Tiles) > 0 {
				b.WriteString(`<div class="tile-grid">`)
				for _, t := range ag.Tiles {
					fmt.Fprintf(&b, `<div class="tile" style="background:%s22;border:1px solid %s44;">
<div class="tile-icon">%s</div>
<div class="tile-count" style="color:%s;">%d</div>
<div class="tile-label">%s</div>
</div>`, t.Color, t.Color, t.Icon, t.Color, t.Count, t.Label)
				}
				b.WriteString(`</div>`)
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div><div class="divider"></div>`)
	}

	// ── Top services table — ALL services grouped by provider ──
	if len(d.Services) > 0 && (reportType == "cost_summary" || reportType == "full") {
		// Aggregate across all accounts, keyed by provider+service
		type svcKey struct{ provider, name string }
		svcMap := map[svcKey]float64{}
		for _, s := range d.Services {
			svcMap[svcKey{s.Provider, s.Name}] += s.Cost
		}

		// Group by provider, sort each group by cost desc
		type svcRow struct {
			name, provider string
			cost           float64
		}
		providerServices := map[string][]svcRow{}
		providerOrder := []string{"aws", "azure", "gcp"}
		for k, cost := range svcMap {
			providerServices[k.provider] = append(providerServices[k.provider], svcRow{k.name, k.provider, cost})
		}
		// Sort each provider's services by cost desc
		for p := range providerServices {
			sl := providerServices[p]
			for i := 0; i < len(sl)-1; i++ {
				for j := i + 1; j < len(sl); j++ {
					if sl[j].cost > sl[i].cost {
						sl[i], sl[j] = sl[j], sl[i]
					}
				}
			}
			providerServices[p] = sl
		}
		// Also collect any providers not in the fixed order
		for p := range providerServices {
			found := false
			for _, op := range providerOrder {
				if op == p {
					found = true
					break
				}
			}
			if !found {
				providerOrder = append(providerOrder, p)
			}
		}

		providerColors := map[string]string{
			"aws": "#FF9900", "azure": "#0078D4", "gcp": "#34A853",
		}
		providerLabels := map[string]string{
			"aws": "Amazon Web Services", "azure": "Microsoft Azure", "gcp": "Google Cloud Platform",
		}

		b.WriteString(`<div class="section"><div class="section-title">All Services by Provider</div>`)
		for _, p := range providerOrder {
			svcs, ok := providerServices[p]
			if !ok || len(svcs) == 0 {
				continue
			}
			color := providerColors[p]
			if color == "" {
				color = "#6366f1"
			}
			label := providerLabels[p]
			if label == "" {
				label = strings.ToUpper(p)
			}
			maxCost := svcs[0].cost
			fmt.Fprintf(&b, `<div style="margin-bottom:20px;">
<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px;">
  <span style="background:%s;color:#fff;border-radius:6px;padding:3px 12px;font-size:12px;font-weight:700;">%s</span>
  <span style="font-size:13px;font-weight:600;color:#374151;">%s</span>
  <span style="margin-left:auto;background:#f1f5f9;border-radius:12px;padding:2px 10px;font-size:12px;font-weight:600;color:#64748b;">%d services</span>
</div>
<table><thead><tr><th>#</th><th>Service</th><th>MTD Cost</th><th>Share</th></tr></thead><tbody>`,
				color, strings.ToUpper(p), label, len(svcs))
			for i, s := range svcs {
				pct := 0.0
				if maxCost > 0 {
					pct = (s.cost / maxCost) * 100
				}
				fmt.Fprintf(&b, `<tr>
<td style="color:#94a3b8;font-size:12px;">%d</td>
<td>%s</td>
<td><strong>$%.2f</strong></td>
<td><div class="bar-wrap"><div class="bar-fill" style="width:%.0f%%;background:%s;"></div></div></td>
</tr>`, i+1, s.name, s.cost, pct, color)
			}
			b.WriteString(`</tbody></table></div>`)
		}
		b.WriteString(`</div><div class="divider"></div>`)
	}

	// ── Anomalies ──
	if len(d.Anomalies) > 0 && (reportType == "anomalies" || reportType == "full") {
		b.WriteString(`<div class="section"><div class="section-title">Cost Anomalies (Last 30 Days)</div><div class="anomaly-grid">`)
		for _, a := range d.Anomalies {
			sevClass := "sev-medium"
			sevColor := "#d97706"
			switch strings.ToLower(a.Severity) {
			case "critical":
				sevClass, sevColor = "sev-critical", "#dc2626"
			case "high":
				sevClass, sevColor = "sev-high", "#ea580c"
			case "low":
				sevClass, sevColor = "sev-low", "#65a30d"
			}
			fmt.Fprintf(&b, `<div class="anomaly-card %s">
<span class="sev-badge" style="background:%s22;color:%s;">%s</span>
<div class="anom-date">%s &nbsp;·&nbsp; %s</div>
<div class="anom-cost">$%.2f</div>
<div class="anom-dev">Baseline $%.2f &nbsp;·&nbsp; +%.1f%%</div>
</div>`, sevClass, sevColor, sevColor, strings.ToUpper(a.Severity),
				a.Date, a.AccountName, a.Actual, a.Baseline, a.Deviation)
		}
		b.WriteString(`</div></div><div class="divider"></div>`)
	}

	// ── Recommendations ──
	if len(d.Recs) > 0 && (reportType == "recommendations" || reportType == "full") {
		b.WriteString(`<div class="section"><div class="section-title">Cost Recommendations</div><div class="rec-list">`)
		for _, r := range d.Recs {
			fmt.Fprintf(&b, `<div class="rec-card">
<div class="rec-info">
  <div class="rec-type">%s</div>
  <div class="rec-svc">%s</div>
  <div class="rec-desc">%s</div>
  <div class="rec-acct">%s</div>
</div>
<div class="savings-badge">
  <div class="save-label">Save/mo</div>
  <div class="save-val">$%.0f</div>
</div>
</div>`, r.Type, r.Service, r.Desc, r.AccountName, r.Savings)
		}
		b.WriteString(`</div>`)

		if d.TotalSaves > 0 {
			fmt.Fprintf(&b, `<div style="margin-top:16px;"><div class="savings-total">
<div><div class="st-label">Total Potential Monthly Savings</div></div>
<div class="st-val">$%.0f / mo</div>
</div></div>`, d.TotalSaves)
		}
		b.WriteString(`</div><div class="divider"></div>`)
	}

	// ── Footer ──
	fmt.Fprintf(&b, `<div class="footer">
<p>This report was generated automatically by your FinOps Platform.</p>
<p style="margin-top:6px;">%s &nbsp;·&nbsp; <a href="#">Manage Schedules</a> &nbsp;·&nbsp; <a href="#">Unsubscribe</a></p>
</div>`, d.Now.Format("Jan 2, 2006 15:04 UTC"))

	b.WriteString(`</div></body></html>`)
	return b.String()
}

// ─── resolveAccountSMTP ───────────────────────────────────────────────────────

func (h *ReportHandler) resolveAccountSMTP(accountID string) smtpResolvedCfg {
	var host, port, username, password, fromEmail string
	var useTLS bool
	err := h.DB.QueryRow(
		`SELECT smtp_host, CAST(smtp_port AS CHAR), smtp_username, encrypted_password,
		        from_email, COALESCE(use_tls, 0)
		 FROM mail_settings
		 WHERE account_id=?
		 LIMIT 1`,
		accountID,
	).Scan(&host, &port, &username, &password, &fromEmail, &useTLS)

	if err == nil && host != "" {
		// Decrypt password if AES key is set
		if h.AESKey != "" && password != "" {
			if dec, err2 := decryptAES(password, h.AESKey); err2 == nil {
				password = dec
			}
		}
		return smtpResolvedCfg{
			Host: host, Port: port,
			Username: username, Password: password,
			From: fromEmail, UseTLS: useTLS,
		}
	}

	// Fall back to platform default from config
	return smtpResolvedCfg{
		Host:     h.EmailCfg.SMTPHost,
		Port:     h.EmailCfg.SMTPPort,
		Username: h.EmailCfg.SMTPUsername,
		Password: h.EmailCfg.SMTPPassword,
		From:     h.EmailCfg.FromEmail,
		UseTLS:   false,
	}
}

// ─── deliverEmail ─────────────────────────────────────────────────────────────

func (h *ReportHandler) deliverEmail(cfg smtpResolvedCfg, to []string, subject, htmlBody string) error {
	if cfg.Host == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	toHeader := strings.Join(to, ", ")
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", toHeader))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	raw := []byte(msg.String())

	if cfg.UseTLS {
		tlsCfg := &tls.Config{ServerName: cfg.Host}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("TLS dial: %w", err)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("SMTP client: %w", err)
		}
		defer client.Close()
		if cfg.Username != "" {
			if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
				return fmt.Errorf("SMTP auth: %w", err)
			}
		}
		if err := client.Mail(cfg.From); err != nil {
			return err
		}
		for _, r := range to {
			if err := client.Rcpt(r); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(raw); err != nil {
			return err
		}
		return w.Close()
	}

	// Plain SMTP with STARTTLS
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	err := smtp.SendMail(addr, auth, cfg.From, to, raw)
	if err != nil {
		// Some SMTP servers close the connection after DATA is accepted,
		// causing a benign EOF/connection-reset that doesn't mean delivery failed.
		errStr := err.Error()
		if strings.Contains(errStr, "EOF") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "use of closed network connection") {
			return nil
		}
		return err
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func calcNextRun(frequency string, dayOfWeek, dayOfMonth *int, sendHour int) time.Time {
	now := time.Now()
	switch frequency {
	case "daily":
		next := time.Date(now.Year(), now.Month(), now.Day(), sendHour, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case "weekly":
		dow := 0
		if dayOfWeek != nil {
			dow = *dayOfWeek
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), sendHour, 0, 0, 0, time.UTC)
		for int(next.Weekday()) != dow || !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case "monthly":
		dom := 1
		if dayOfMonth != nil {
			dom = *dayOfMonth
		}
		next := time.Date(now.Year(), now.Month(), dom, sendHour, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.AddDate(0, 1, 0)
		}
		return next
	}
	return now.Add(24 * time.Hour)
}

func reportTypeLabel(rt string) string {
	switch rt {
	case "cost_summary":
		return "Cost Summary"
	case "anomalies":
		return "Anomaly Report"
	case "recommendations":
		return "Recommendations"
	case "full":
		return "Full Report"
	}
	return titleCase(rt)
}

func titleCase(s string) string {
	words := strings.Fields(strings.ReplaceAll(s, "_", " "))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

func daysInMonth(t time.Time) int {
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	return int(first.AddDate(0, 1, 0).Sub(first).Hours() / 24)
}

// decryptAES wraps the package-level decrypt helper from crypto.go.
func decryptAES(ciphertext, key string) (string, error) {
	return decrypt(ciphertext, key)
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
