package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// CostEntry represents a single daily cost record from a cloud provider.
type CostEntry struct {
	Date       time.Time
	Service    string
	ResourceID string
	Region     string
	Amount     float64
	Currency   string
}

// CloudProvider is the interface all provider implementations must satisfy.
type CloudProvider interface {
	ValidateCredentials(creds map[string]string) error
	FetchCosts(creds map[string]string, start, end time.Time) ([]CostEntry, error)
}

// ── AWS ───────────────────────────────────────────────────────────────────────

type awsProvider struct{}

func (a *awsProvider) ValidateCredentials(creds map[string]string) error {
	if creds["access_key_id"] == "" || creds["secret_access_key"] == "" {
		return fmt.Errorf("AWS credentials must include access_key_id and secret_access_key")
	}
	return nil
}

func (a *awsProvider) FetchCosts(creds map[string]string, start, end time.Time) ([]CostEntry, error) {
	// Cost Explorer is a global service — always use us-east-1 as the API endpoint.
	// It returns costs across ALL regions regardless of this setting.
	cfg := aws.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			creds["access_key_id"],
			creds["secret_access_key"],
			creds["session_token"],
		),
	}
	ce := costexplorer.NewFromConfig(cfg)

	// Cost Explorer end date is exclusive — add 1 day so today's costs are included.
	endExclusive := end.AddDate(0, 0, 1)

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &cetypes.DateInterval{Start: aws.String(start.Format("2006-01-02")), End: aws.String(endExclusive.Format("2006-01-02"))},
		Granularity: cetypes.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		// Group by SERVICE + REGION — max 2 GroupBy dimensions supported by Cost Explorer.
		GroupBy: []cetypes.GroupDefinition{
			{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
			{Type: cetypes.GroupDefinitionTypeDimension, Key: aws.String("REGION")},
		},
	}

	var entries []CostEntry

	// Paginate — Cost Explorer returns NextPageToken when there are more results.
	for {
		out, err := ce.GetCostAndUsage(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("AWS Cost Explorer: %w", err)
		}

		for _, result := range out.ResultsByTime {
			date, err := time.Parse("2006-01-02", *result.TimePeriod.Start)
			if err != nil {
				continue
			}
			for _, group := range result.Groups {
				// Keys[0] = SERVICE, Keys[1] = REGION
				svc, region := "Unknown", "global"
				if len(group.Keys) > 0 && group.Keys[0] != "" {
					svc = group.Keys[0]
				}
				if len(group.Keys) > 1 && group.Keys[1] != "" {
					region = group.Keys[1]
				}
				amountStr := "0"
				if m, ok := group.Metrics["UnblendedCost"]; ok && m.Amount != nil {
					amountStr = *m.Amount
				}
				amount, _ := strconv.ParseFloat(amountStr, 64)
				// Skip only exact zero — keep everything >= $0.0000000001
				if amount < 0.0000000001 {
					continue
				}
				entries = append(entries, CostEntry{
					Date:       date,
					Service:    svc,
					ResourceID: fmt.Sprintf("aws-%s-%s", svc, region),
					Region:     region,
					Amount:     amount,
					Currency:   "USD",
				})
			}
		}

		// No more pages
		if out.NextPageToken == nil || *out.NextPageToken == "" {
			break
		}
		input.NextPageToken = out.NextPageToken
	}

	log.Printf("[aws] FetchCosts %s→%s: %d entries fetched", start.Format("2006-01-02"), end.Format("2006-01-02"), len(entries))
	return entries, nil
}

// ── Azure (real Cost Management REST API) ─────────────────────────────────────

type azureProvider struct{}

func (a *azureProvider) ValidateCredentials(creds map[string]string) error {
	if creds["subscription_id"] == "" || creds["tenant_id"] == "" ||
		creds["client_id"] == "" || creds["client_secret"] == "" {
		return fmt.Errorf("Azure credentials must include subscription_id, tenant_id, client_id, and client_secret")
	}
	return nil
}

func (a *azureProvider) FetchCosts(creds map[string]string, start, end time.Time) ([]CostEntry, error) {
	token, err := azureGetToken(creds["tenant_id"], creds["client_id"], creds["client_secret"])
	if err != nil {
		return nil, fmt.Errorf("Azure auth: %w", err)
	}

	subID := creds["subscription_id"]

	// Try Cost Management API first, fall back to Consumption API on 403
	entries, err := azureFetchCostManagement(token, subID, start, end)
	if err != nil {
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "AuthorizationFailed") {
			log.Printf("[azure] Cost Management API unauthorized, falling back to Consumption API")
			return azureFetchConsumption(token, subID, start, end)
		}
		return nil, err
	}
	return entries, nil
}

func azureFetchCostManagement(token, subID string, start, end time.Time) ([]CostEntry, error) {
	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.CostManagement/query?api-version=2023-11-01",
		subID,
	)

	doQuery := func(payload []byte) ([][]interface{}, []struct{ Name string }, error) {
		req, _ := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		var result struct {
			Properties struct {
				Columns []struct{ Name string } `json:"columns"`
				Rows    [][]interface{}         `json:"rows"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, nil, fmt.Errorf("parse: %w", err)
		}
		return result.Properties.Rows, result.Properties.Columns, nil
	}

	// Query 1: total per service
	svcPayload, _ := json.Marshal(map[string]interface{}{
		"type":      "Usage",
		"timeframe": "Custom",
		"timePeriod": map[string]string{
			"from": start.Format("2006-01-02") + "T00:00:00Z",
			"to":   end.Format("2006-01-02") + "T00:00:00Z",
		},
		"dataset": map[string]interface{}{
			"granularity": "None",
			"aggregation": map[string]interface{}{
				"totalCost": map[string]string{"name": "PreTaxCost", "function": "Sum"},
			},
			"grouping": []map[string]string{
				{"type": "Dimension", "name": "ServiceName"},
			},
		},
	})
	svcRows, svcCols, err := doQuery(svcPayload)
	if err != nil {
		return nil, fmt.Errorf("Azure Cost API (services): %w", err)
	}

	colIdx := map[string]int{}
	for i, col := range svcCols {
		colIdx[col.Name] = i
	}
	costIdx := colIdx["PreTaxCost"]
	svcIdx := colIdx["ServiceName"]

	type svcCost struct {
		name string
		cost float64
	}
	var services []svcCost
	var totalCost float64
	for _, row := range svcRows {
		if len(row) < 2 {
			continue
		}
		amount, _ := strconv.ParseFloat(fmt.Sprintf("%v", row[costIdx]), 64)
		if amount < 0.0000000001 {
			continue
		}
		svc := fmt.Sprintf("%v", row[svcIdx])
		services = append(services, svcCost{name: svc, cost: amount})
		totalCost += amount
	}

	if len(services) == 0 {
		log.Printf("[azure] no cost data returned for %s→%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
		return nil, nil
	}

	// Query 2: daily totals
	dailyPayload, _ := json.Marshal(map[string]interface{}{
		"type":      "Usage",
		"timeframe": "Custom",
		"timePeriod": map[string]string{
			"from": start.Format("2006-01-02") + "T00:00:00Z",
			"to":   end.Format("2006-01-02") + "T00:00:00Z",
		},
		"dataset": map[string]interface{}{
			"granularity": "Daily",
			"aggregation": map[string]interface{}{
				"totalCost": map[string]string{"name": "PreTaxCost", "function": "Sum"},
			},
		},
	})
	dailyRows, dailyCols, err := doQuery(dailyPayload)
	if err != nil {
		log.Printf("[azure] daily query failed, spreading evenly: %v", err)
		dailyRows = nil
	}

	type dayTotal struct {
		date time.Time
		cost float64
	}
	var days []dayTotal
	if len(dailyRows) > 0 {
		dColIdx := map[string]int{}
		for i, col := range dailyCols {
			dColIdx[col.Name] = i
		}
		dCostIdx := dColIdx["PreTaxCost"]
		dDateIdx := dColIdx["UsageDate"]
		for _, row := range dailyRows {
			if len(row) < 2 {
				continue
			}
			amt, _ := strconv.ParseFloat(fmt.Sprintf("%v", row[dCostIdx]), 64)
			dateRaw := strings.Split(fmt.Sprintf("%v", row[dDateIdx]), ".")[0]
			var date time.Time
			switch {
			case len(dateRaw) == 8:
				date, _ = time.Parse("20060102", dateRaw)
			case len(dateRaw) >= 10:
				date, _ = time.Parse("2006-01-02", dateRaw[:10])
			default:
				continue
			}
			if date.IsZero() {
				continue
			}
			days = append(days, dayTotal{date: date, cost: amt})
		}
	}

	if len(days) == 0 {
		numDays := int(end.Sub(start).Hours()/24) + 1
		if numDays < 1 {
			numDays = 1
		}
		dailyAmt := totalCost / float64(numDays)
		for d := 0; d < numDays; d++ {
			days = append(days, dayTotal{date: start.AddDate(0, 0, d), cost: dailyAmt})
		}
	}

	var dailySum float64
	for _, d := range days {
		dailySum += d.cost
	}

	var entries []CostEntry
	for _, svc := range services {
		for _, day := range days {
			var dayAmt float64
			if dailySum > 0 {
				dayAmt = svc.cost * (day.cost / dailySum)
			} else {
				dayAmt = svc.cost / float64(len(days))
			}
			if dayAmt < 0.0000000001 {
				continue
			}
			entries = append(entries, CostEntry{
				Date:       day.date,
				Service:    svc.name,
				ResourceID: fmt.Sprintf("azure-%s", svc.name),
				Region:     "azure",
				Amount:     dayAmt,
				Currency:   "USD",
			})
		}
	}

	log.Printf("[azure] CostManagement %s→%s: %d services, %.2f total, %d entries",
		start.Format("2006-01-02"), end.Format("2006-01-02"), len(services), totalCost, len(entries))
	return entries, nil
}

// azureFetchConsumption uses the Consumption UsageDetails API (requires Billing Reader role).
func azureFetchConsumption(token, subID string, start, end time.Time) ([]CostEntry, error) {
	// Consumption API — paginate through all usage details
	apiURL := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Consumption/usageDetails?api-version=2023-03-01&$filter=properties/usageStart ge '%s' and properties/usageEnd le '%s'&$top=1000",
		subID,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)

	client := &http.Client{Timeout: 60 * time.Second}

	// service → day → cost
	type key struct {
		svc string
		day string
	}
	costMap := map[key]float64{}

	for apiURL != "" {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Azure Consumption API: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Azure Consumption API: HTTP %d: %s", resp.StatusCode, string(body))
		}

		var page struct {
			Value []struct {
				Properties struct {
					UsageStart      string  `json:"usageStart"`
					ConsumedService string  `json:"consumedService"`
					PretaxCost      float64 `json:"pretaxCost"`
					BillingCurrency string  `json:"billingCurrency"`
				} `json:"properties"`
			} `json:"value"`
			NextLink string `json:"nextLink"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("Azure Consumption parse: %w", err)
		}

		for _, item := range page.Value {
			if item.Properties.PretaxCost < 0.0000000001 {
				continue
			}
			day := ""
			if len(item.Properties.UsageStart) >= 10 {
				day = item.Properties.UsageStart[:10]
			}
			svc := item.Properties.ConsumedService
			if svc == "" {
				svc = "Unknown"
			}
			costMap[key{svc: svc, day: day}] += item.Properties.PretaxCost
		}

		apiURL = page.NextLink
	}

	var entries []CostEntry
	for k, amount := range costMap {
		if amount < 0.0000000001 {
			continue
		}
		date, err := time.Parse("2006-01-02", k.day)
		if err != nil {
			continue
		}
		entries = append(entries, CostEntry{
			Date:       date,
			Service:    k.svc,
			ResourceID: fmt.Sprintf("azure-%s", k.svc),
			Region:     "azure",
			Amount:     amount,
			Currency:   "USD",
		})
	}

	log.Printf("[azure] Consumption API %s→%s: %d entries", start.Format("2006-01-02"), end.Format("2006-01-02"), len(entries))
	return entries, nil
}

func azureGetToken(tenantID, clientID, clientSecret string) (string, error) {
	url := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)
	body := strings.NewReader(fmt.Sprintf(
		"client_id=%s&client_secret=%s&scope=https://management.azure.com/.default&grant_type=client_credentials",
		clientID, clientSecret,
	))
	resp, err := http.Post(url, "application/x-www-form-urlencoded", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.Error != "" {
		return "", fmt.Errorf("%s: %s", tok.Error, tok.ErrorDesc)
	}
	return tok.AccessToken, nil
}

// ── GCP (BigQuery billing export) ─────────────────────────────────────────────

type gcpProvider struct{}

func (g *gcpProvider) ValidateCredentials(creds map[string]string) error {
	if creds["project_id"] == "" || creds["service_account_key"] == "" {
		return fmt.Errorf("GCP credentials must include project_id and service_account_key")
	}
	return nil
}

func (g *gcpProvider) FetchCosts(creds map[string]string, start, end time.Time) ([]CostEntry, error) {
	projectID := creds["project_id"]
	saKey := creds["service_account_key"]
	billingDataset := creds["billing_dataset"] // e.g. "my_billing_dataset"
	billingTable := creds["billing_table"]     // e.g. "gcp_billing_export_v1_XXXXXX"

	opts := []option.ClientOption{option.WithCredentialsJSON([]byte(saKey))}

	// If BigQuery billing export is configured, use it for accurate costs
	if billingDataset != "" && billingTable != "" {
		return gcpCostsFromBigQuery(projectID, billingDataset, billingTable, start, end, opts)
	}

	// Fallback: return empty (GCP billing API doesn't expose spend without BigQuery export)
	return nil, fmt.Errorf("GCP requires BigQuery billing export. Add billing_dataset and billing_table to credentials")
}

func gcpCostsFromBigQuery(projectID, dataset, table string, start, end time.Time, opts []option.ClientOption) ([]CostEntry, error) {
	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("BigQuery client: %w", err)
	}
	defer client.Close()

	query := fmt.Sprintf(`
		SELECT
			DATE(usage_start_time) as usage_date,
			service.description as service_name,
			SUM(cost) as total_cost
		FROM `+"`%s.%s`"+`
		WHERE DATE(usage_start_time) >= '%s'
		  AND DATE(usage_start_time) <= '%s'
		  AND cost > 0
		GROUP BY usage_date, service_name
		ORDER BY usage_date ASC`,
		dataset, table,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)

	q := client.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("BigQuery query: %w", err)
	}

	type bqRow struct {
		UsageDate   bigquery.NullDate   `bigquery:"usage_date"`
		ServiceName bigquery.NullString `bigquery:"service_name"`
		TotalCost   float64             `bigquery:"total_cost"`
	}

	var entries []CostEntry
	for {
		var row bqRow
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("BigQuery read: %w", err)
		}
		if row.TotalCost < 0.0000000001 {
			continue
		}
		svc := row.ServiceName.StringVal
		date := time.Date(int(row.UsageDate.Date.Year), row.UsageDate.Date.Month, int(row.UsageDate.Date.Day), 0, 0, 0, 0, time.UTC)
		entries = append(entries, CostEntry{
			Date: date, Service: svc,
			ResourceID: fmt.Sprintf("gcp-%s", svc),
			Region:     "gcp", Amount: row.TotalCost, Currency: "USD",
		})
	}
	return entries, nil
}

// ── Factory ───────────────────────────────────────────────────────────────────

func NewCloudProvider(provider string) (CloudProvider, error) {
	switch provider {
	case "aws":
		return &awsProvider{}, nil
	case "azure":
		return &azureProvider{}, nil
	case "gcp":
		return &gcpProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
	}
}
