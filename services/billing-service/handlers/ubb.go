package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	stripe "github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/invoice"
	"github.com/stripe/stripe-go/v76/invoiceitem"
	"github.com/stripe/stripe-go/v76/price"
	"github.com/stripe/stripe-go/v76/subscriptionitem"
	"github.com/stripe/stripe-go/v76/usagerecord"
	"github.com/stripe/stripe-go/v76/usagerecordsummary"
)

// UBBHandler handles Usage-Based Billing streams and meter events.
type UBBHandler struct {
	DB *sql.DB
}

// ── DB bootstrap ──────────────────────────────────────────────────────────────

// EnsureUBBTable creates all UBB tables at startup (not inline on every request).
func EnsureUBBTable(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ubb_streams (
			id                    VARCHAR(36)  PRIMARY KEY,
			account_id            VARCHAR(36)  NOT NULL,
			stream_name           VARCHAR(255) NOT NULL,
			resolver_id           VARCHAR(255) NOT NULL,
			api_key               VARCHAR(64)  NOT NULL UNIQUE,
			stripe_sub_item_id    VARCHAR(255) NOT NULL DEFAULT '',
			stripe_customer_id    VARCHAR(255) NOT NULL DEFAULT '',
			plan_name             VARCHAR(64)  NOT NULL DEFAULT '',
			included_units        INT          NOT NULL DEFAULT 1000,
			overage_price_cents   INT          NOT NULL DEFAULT 4,
			sub_item_price_cents  INT          NOT NULL DEFAULT 0,
			status                VARCHAR(32)  NOT NULL DEFAULT 'active',
			created_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at            DATETIME     NULL,
			INDEX idx_ubb_account (account_id),
			INDEX idx_ubb_api_key (api_key)
		)`,
		// Ensure sub_item_price_cents column exists on pre-existing tables.
		// MySQL doesn't support ADD COLUMN IF NOT EXISTS, so we check information_schema first.
		`SET @_col = (SELECT COUNT(*) FROM information_schema.COLUMNS
		  WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='ubb_streams' AND COLUMN_NAME='sub_item_price_cents')`,
		`SET @_sql = IF(@_col=0,
		  'ALTER TABLE ubb_streams ADD COLUMN sub_item_price_cents INT NOT NULL DEFAULT 0',
		  'SELECT 1')`,
		`PREPARE _stmt FROM @_sql`,
		`EXECUTE _stmt`,
		`DEALLOCATE PREPARE _stmt`,
		`CREATE TABLE IF NOT EXISTS ubb_usage_events (
			id              VARCHAR(36)  PRIMARY KEY,
			stream_id       VARCHAR(36)  NOT NULL,
			account_id      VARCHAR(36)  NOT NULL,
			quantity        BIGINT       NOT NULL,
			action          VARCHAR(16)  NOT NULL DEFAULT 'increment',
			idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
			event_ts        DATETIME     NOT NULL,
			created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_stream_ts (stream_id, event_ts),
			UNIQUE KEY uq_idempotency (stream_id, idempotency_key)
		)`,
		// Permanent revenue ledger — survives stream deletion.
		// Every time a stream is deleted, its accrued overage is snapshotted here.
		// This ensures billed amounts are never lost and are included in future invoices.
		`CREATE TABLE IF NOT EXISTS ubb_billed_revenue (
			id                  VARCHAR(36)   PRIMARY KEY,
			account_id          VARCHAR(36)   NOT NULL,
			stream_id           VARCHAR(36)   NOT NULL,
			stream_name         VARCHAR(255)  NOT NULL,
			total_units         BIGINT        NOT NULL DEFAULT 0,
			included_units      INT           NOT NULL DEFAULT 0,
			overage_units       BIGINT        NOT NULL DEFAULT 0,
			overage_price_cents INT           NOT NULL DEFAULT 0,
			overage_cents       BIGINT        NOT NULL DEFAULT 0,
			billing_period      VARCHAR(32)   NOT NULL DEFAULT '',
			stripe_invoiced     TINYINT(1)    NOT NULL DEFAULT 0,
			stripe_invoice_id   VARCHAR(255)  NOT NULL DEFAULT '',
			created_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_rev_account (account_id),
			INDEX idx_rev_period  (account_id, billing_period),
			UNIQUE KEY uq_stream_period (stream_id, billing_period)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// ── Request / Response types ──────────────────────────────────────────────────

// SyncSubItemPrices backfills sub_item_price_cents for existing streams that have
// sub_item_price_cents=0 but a real Stripe sub item. Called once at startup.
// This fixes streams created before the sub_item_price_cents column was added.
func SyncSubItemPrices(db *sql.DB) {
	rows, err := db.Query(
		`SELECT id, stripe_sub_item_id FROM ubb_streams
		 WHERE deleted_at IS NULL AND stripe_sub_item_id != '' AND sub_item_price_cents = 0`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	type row struct{ id, subItemID string }
	var toSync []row
	for rows.Next() {
		var r row
		if rows.Scan(&r.id, &r.subItemID) == nil {
			toSync = append(toSync, r)
		}
	}

	for _, r := range toSync {
		si, err := subscriptionitem.Get(r.subItemID, nil)
		if err != nil || si.Price == nil {
			continue
		}
		unitAmount := si.Price.UnitAmount
		if unitAmount > 0 {
			_, _ = db.Exec(
				`UPDATE ubb_streams SET sub_item_price_cents=? WHERE id=?`,
				unitAmount, r.id,
			)
		}
	}
}

type createStreamRequest struct {
	StreamName        string `json:"stream_name" binding:"required"`
	ResolverID        string `json:"resolver_id" binding:"required"`
	IncludedUnits     int    `json:"included_units"`
	OveragePriceCents int    `json:"overage_price_cents"`
}

type postUsageRequest struct {
	Quantity       int64  `json:"quantity" binding:"required"`
	Timestamp      int64  `json:"timestamp"`       // unix; 0 = now
	Action         string `json:"action"`          // "increment" (default) or "set"
	IdempotencyKey string `json:"idempotency_key"` // client-supplied dedup key
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// CreateStream POST /billing/ubb/streams
// Resolves a dedicated Stripe metered sub item for this stream (not shared).
func (h *UBBHandler) CreateStream(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var req createStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.IncludedUnits == 0 {
		req.IncludedUnits = 1000
	}
	if req.OveragePriceCents == 0 {
		req.OveragePriceCents = 4
	}

	// Resolve stripe customer + a DEDICATED metered sub item for this stream.
	// sub_item_price_cents is stored so we never need to call Stripe API to check it later.
	stripeCustomerID, stripeSubItemID, planName, subItemPriceCents := h.resolveStripeContextForStream(accountID, int64(req.OveragePriceCents))

	apiKey, err := generateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate api key"})
		return
	}

	id := uuid.New().String()
	_, err = h.DB.Exec(`
		INSERT INTO ubb_streams
		  (id, account_id, stream_name, resolver_id, api_key,
		   stripe_sub_item_id, stripe_customer_id, plan_name,
		   included_units, overage_price_cents, sub_item_price_cents, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', NOW())`,
		id, accountID, req.StreamName, req.ResolverID, apiKey,
		stripeSubItemID, stripeCustomerID, planName,
		req.IncludedUnits, req.OveragePriceCents, subItemPriceCents,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create stream: " + err.Error()})
		return
	}

	logAuditEvent(h.DB, "", accountID, "create", "ubb_stream", id, c.ClientIP(), c.Request.UserAgent())

	c.JSON(http.StatusCreated, gin.H{
		"id":                  id,
		"stream_name":         req.StreamName,
		"resolver_id":         req.ResolverID,
		"api_key":             apiKey,
		"included_units":      req.IncludedUnits,
		"overage_price_cents": req.OveragePriceCents,
		"plan_name":           planName,
		"stripe_customer_id":  stripeCustomerID,
		"stripe_sub_item_id":  stripeSubItemID,
		"status":              "active",
	})
}

// ListStreams GET /billing/ubb/streams
func (h *UBBHandler) ListStreams(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	rows, err := h.DB.Query(`
		SELECT id, stream_name, resolver_id, api_key,
		       stripe_sub_item_id, stripe_customer_id, plan_name,
		       included_units, overage_price_cents, status, created_at
		FROM ubb_streams
		WHERE account_id=? AND deleted_at IS NULL
		ORDER BY created_at DESC`, accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer rows.Close()

	type streamRow struct {
		ID                string `json:"id"`
		StreamName        string `json:"stream_name"`
		ResolverID        string `json:"resolver_id"`
		APIKey            string `json:"api_key"`
		StripeSubItemID   string `json:"stripe_sub_item_id"`
		StripeCustomerID  string `json:"stripe_customer_id"`
		PlanName          string `json:"plan_name"`
		IncludedUnits     int    `json:"included_units"`
		OveragePriceCents int    `json:"overage_price_cents"`
		Status            string `json:"status"`
		CreatedAt         string `json:"created_at"`
	}

	var streams []streamRow
	for rows.Next() {
		var s streamRow
		if err := rows.Scan(&s.ID, &s.StreamName, &s.ResolverID, &s.APIKey,
			&s.StripeSubItemID, &s.StripeCustomerID, &s.PlanName,
			&s.IncludedUnits, &s.OveragePriceCents, &s.Status, &s.CreatedAt); err == nil {
			streams = append(streams, s)
		}
	}
	if streams == nil {
		streams = []streamRow{}
	}
	c.JSON(http.StatusOK, gin.H{"streams": streams})
}

// DeleteStream DELETE /billing/ubb/streams/:id
// Snapshots accrued overage into ubb_billed_revenue before soft-deleting the stream.
func (h *UBBHandler) DeleteStream(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	streamID := c.Param("id")

	// Read stream details before deleting
	var streamName, subItemID string
	var includedUnits, overagePriceCents, subItemPriceCents int64
	err := h.DB.QueryRow(
		`SELECT stream_name, stripe_sub_item_id, included_units, overage_price_cents, sub_item_price_cents
		 FROM ubb_streams WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		streamID, accountID,
	).Scan(&streamName, &subItemID, &includedUnits, &overagePriceCents, &subItemPriceCents)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// Calculate total usage at time of deletion using the same authoritative logic
	// (only trust Stripe summaries when sub item has a non-zero unit price)
	periodStart, periodEnd := h.currentBillingPeriod(accountID)
	totalUnits, _ := h.resolvedUsage(subItemID, streamID, subItemPriceCents, periodStart, periodEnd)

	// Snapshot billed amount into permanent revenue ledger (new model: no free-tier deduction)
	var billedCents int64
	if subItemID != "" && subItemPriceCents > 0 {
		// Stripe stream: all units billed at sub_item_price_cents
		billedCents = totalUnits * subItemPriceCents
	} else {
		// Local stream: all units billed at overage_price_cents (no free-tier)
		billedCents = totalUnits * overagePriceCents
	}
	now := time.Now()
	billingPeriod := fmt.Sprintf("%d-%02d", now.Year(), int(now.Month()))

	_, _ = h.DB.Exec(
		`INSERT INTO ubb_billed_revenue
		 (id, account_id, stream_id, stream_name, total_units, included_units,
		  overage_units, overage_price_cents, overage_cents, billing_period)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   total_units=VALUES(total_units),
		   overage_units=VALUES(overage_units),
		   overage_cents=VALUES(overage_cents)`,
		uuid.New().String(), accountID, streamID, streamName,
		totalUnits, includedUnits, totalUnits, overagePriceCents, billedCents, billingPeriod,
	)

	// Soft-delete the stream
	res, err := h.DB.Exec(
		`UPDATE ubb_streams SET deleted_at=NOW(), status='deleted' WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		streamID, accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      "stream deleted",
		"snapshotted":  true,
		"billed_cents": billedCents,
	})
}

// PostUsage POST /billing/ubb/streams/:id/usage
// Auth: accepts either X-Account-ID (internal) OR X-API-Key (external SDK calls).
// Idempotent: duplicate idempotency_key for the same stream is silently ignored.
func (h *UBBHandler) PostUsage(c *gin.Context) {
	streamID := c.Param("id")

	// ── Auth: API key takes priority, falls back to account header ──
	accountID, authErr := h.resolveUsageAuth(c, streamID)
	if authErr != "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": authErr})
		return
	}

	var req postUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Action == "" {
		req.Action = "increment"
	}

	// Derive idempotency key: client-supplied or auto-generated (UUID — always unique)
	idemKey := req.IdempotencyKey
	if idemKey == "" {
		idemKey = uuid.New().String()
	}

	// Fetch stream
	var subItemID, streamName string
	var includedUnits int
	err := h.DB.QueryRow(
		`SELECT stripe_sub_item_id, stream_name, included_units
		 FROM ubb_streams WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		streamID, accountID,
	).Scan(&subItemID, &streamName, &includedUnits)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	ts := req.Timestamp
	if ts == 0 {
		ts = time.Now().Unix()
	}

	// ── Idempotency check — skip if already recorded ──
	if h.usageAlreadyRecorded(streamID, idemKey) {
		c.JSON(http.StatusOK, gin.H{
			"recorded":        false,
			"idempotent_skip": true,
			"stream_name":     streamName,
		})
		return
	}

	// ── No Stripe sub item: local only ──
	if subItemID == "" {
		h.saveLocalUsage(streamID, accountID, req.Quantity, ts, req.Action, idemKey)
		c.JSON(http.StatusOK, gin.H{
			"recorded":    true,
			"quantity":    req.Quantity,
			"timestamp":   ts,
			"billed_via":  "local",
			"stream_name": streamName,
		})
		return
	}

	// ── Post to Stripe ──
	action := stripe.UsageRecordActionIncrement
	if req.Action == "set" {
		action = stripe.UsageRecordActionSet
	}
	stripeParams := &stripe.UsageRecordParams{
		SubscriptionItem: stripe.String(subItemID),
		Quantity:         stripe.Int64(req.Quantity),
		Timestamp:        stripe.Int64(ts),
		Action:           stripe.String(string(action)),
	}
	ur, err := usagerecord.New(stripeParams)
	if err != nil {
		// Fallback: save locally so usage isn't lost
		h.saveLocalUsage(streamID, accountID, req.Quantity, ts, req.Action, idemKey)
		c.JSON(http.StatusOK, gin.H{
			"recorded":     true,
			"quantity":     req.Quantity,
			"timestamp":    ts,
			"billed_via":   "local_fallback",
			"stripe_error": err.Error(),
		})
		return
	}

	h.saveLocalUsage(streamID, accountID, req.Quantity, ts, req.Action, idemKey)

	c.JSON(http.StatusOK, gin.H{
		"recorded":         true,
		"stripe_record_id": ur.ID,
		"quantity":         ur.Quantity,
		"timestamp":        ur.Timestamp,
		"billed_via":       "stripe",
		"stream_name":      streamName,
	})
}

// GetUsageSummary GET /billing/ubb/streams/:id/usage
// Returns usage for the CURRENT billing period only (not all-time).
func (h *UBBHandler) GetUsageSummary(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	streamID := c.Param("id")

	var subItemID, streamName string
	var includedUnits, overagePriceCents, subItemPriceCents int
	err := h.DB.QueryRow(
		`SELECT stripe_sub_item_id, stream_name, included_units, overage_price_cents, sub_item_price_cents
		 FROM ubb_streams WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		streamID, accountID,
	).Scan(&subItemID, &streamName, &includedUnits, &overagePriceCents, &subItemPriceCents)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		return
	}

	// Current billing period boundaries from the active subscription
	periodStart, periodEnd := h.currentBillingPeriod(accountID)

	// resolvedUsage: Stripe is authoritative when sub item has real price
	total, source := h.resolvedUsage(subItemID, streamID, int64(subItemPriceCents), periodStart, periodEnd)

	// Expose raw counts for the debug row in the UI
	localTotal := h.localUsageForPeriod(streamID, periodStart, periodEnd)
	stripeTotal := int64(0)
	if source == "stripe" {
		stripeTotal = total
	} else if subItemID != "" && int64(subItemPriceCents) > 0 {
		// source=local means Stripe returned 0, show that explicitly
		stripeTotal = stripeUsageForPeriod(subItemID, periodStart, periodEnd)
	}

	overage := int64(0)
	if total > int64(includedUnits) {
		overage = total - int64(includedUnits)
	}
	overageCost := float64(overage) * float64(overagePriceCents) / 100.0

	c.JSON(http.StatusOK, gin.H{
		"stream_id":           streamID,
		"stream_name":         streamName,
		"total_usage":         total,
		"included_units":      includedUnits,
		"overage_units":       overage,
		"overage_cost_usd":    fmt.Sprintf("%.2f", overageCost),
		"overage_price_cents": overagePriceCents,
		"period_start":        periodStart,
		"period_end":          periodEnd,
		"local_total":         localTotal,
		"stripe_total":        stripeTotal,
	})
}

// PreviewInvoice GET /billing/ubb/invoice/preview
// Returns the upcoming Stripe invoice. For metered lines priced at $0 (legacy sub items
// created before per-unit pricing was configured), the amount is patched with the
// locally-calculated overage so the UI always shows the correct charge.
func (h *UBBHandler) PreviewInvoice(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var stripeSubID string
	_ = h.DB.QueryRow(
		`SELECT stripe_subscription_id FROM stripe_subscriptions
		 WHERE account_id=? AND status IN ('active','trialing') AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		accountID,
	).Scan(&stripeSubID)

	if stripeSubID == "" || isLocalSubID(stripeSubID) {
		c.JSON(http.StatusOK, gin.H{"preview": nil, "message": "no active Stripe subscription"})
		return
	}

	params := &stripe.InvoiceUpcomingParams{
		Subscription: stripe.String(stripeSubID),
	}
	inv, err := invoice.Upcoming(params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"preview": nil, "message": err.Error()})
		return
	}

	// Build a map of stripe_sub_item_id → stream info for patching $0 metered lines
	type streamMeta struct {
		ID                string
		Name              string
		IncludedUnits     int64
		OveragePriceCents int64
		SubItemPriceCents int64
	}
	subItemToStream := map[string]streamMeta{}
	sRows, _ := h.DB.Query(
		`SELECT id, stream_name, stripe_sub_item_id, included_units, overage_price_cents, sub_item_price_cents
		 FROM ubb_streams WHERE account_id=? AND deleted_at IS NULL AND stripe_sub_item_id != ''`,
		accountID,
	)
	if sRows != nil {
		defer sRows.Close()
		for sRows.Next() {
			var m streamMeta
			var subItemID string
			if sRows.Scan(&m.ID, &m.Name, &subItemID, &m.IncludedUnits, &m.OveragePriceCents, &m.SubItemPriceCents) == nil {
				subItemToStream[subItemID] = m
			}
		}
	}

	periodStart, periodEnd := h.currentBillingPeriod(accountID)

	type lineItem struct {
		Description    string  `json:"description"`
		Amount         float64 `json:"amount_usd"`
		Quantity       int64   `json:"quantity"`
		UnitAmountZero bool    `json:"unit_amount_zero"` // true when sub item has $0/unit price (legacy)
	}

	normaliseDesc := func(desc string) string {
		if idx := strings.LastIndex(desc, " ("); idx > 0 {
			if strings.HasSuffix(desc, " units)") || strings.HasSuffix(desc, "units)") {
				return strings.TrimSpace(desc[:idx])
			}
		}
		return desc
	}

	lineMap := make(map[string]*lineItem)
	var lineOrder []string
	var totalAmountDue float64 = float64(inv.AmountDue) / 100.0

	for _, l := range inv.Lines.Data {
		// Determine if this is a metered line with $0 unit price
		subItemID := ""
		if l.SubscriptionItem != nil {
			subItemID = l.SubscriptionItem.ID
		}
		isMetered := l.Price != nil && l.Price.Recurring != nil &&
			l.Price.Recurring.UsageType == stripe.PriceRecurringUsageTypeMetered

		// A sub item is "legacy" (stale) if our DB says sub_item_price_cents=0
		// OR if Stripe reports unit_amount=0. Either way, don't trust Stripe's quantity.
		meta, isMapped := subItemToStream[subItemID]
		isLegacySubItem := isMetered && (l.Price == nil || l.Price.UnitAmount == 0 || (isMapped && meta.SubItemPriceCents == 0))

		// Skip stale $0-priced metered lines that are NOT mapped to an active stream.
		if isLegacySubItem && l.Amount == 0 && !isMapped {
			totalAmountDue -= float64(l.Amount) / 100.0
			continue
		}

		key := normaliseDesc(l.Description)
		amountUSD := float64(l.Amount) / 100.0

		unitAmountZero := false
		if isLegacySubItem && subItemID != "" && isMapped {
			// Always use local DB usage — Stripe's quantity on a legacy sub item is stale/unreliable
			var localUsage int64
			_ = h.DB.QueryRow(
				`SELECT COALESCE(SUM(quantity),0) FROM ubb_usage_events
				 WHERE stream_id=? AND event_ts >= FROM_UNIXTIME(?) AND event_ts < FROM_UNIXTIME(?)`,
				meta.ID, periodStart, periodEnd,
			).Scan(&localUsage)

			// Skip this line entirely if there's no local usage
			if localUsage == 0 {
				continue
			}

			unitAmountZero = true
			overage := int64(0)
			if localUsage > meta.IncludedUnits {
				overage = localUsage - meta.IncludedUnits
			}
			patchedAmount := float64(overage*meta.OveragePriceCents) / 100.0
			if patchedAmount > amountUSD {
				totalAmountDue += patchedAmount - amountUSD
				amountUSD = patchedAmount
			}
			key = fmt.Sprintf("%s — usage overage (%d units)", meta.Name, localUsage)
			l.Quantity = localUsage
		}

		if existing, ok := lineMap[key]; ok {
			existing.Amount += amountUSD
			existing.Quantity += l.Quantity
		} else {
			lineMap[key] = &lineItem{
				Description:    key,
				Amount:         amountUSD,
				Quantity:       l.Quantity,
				UnitAmountZero: unitAmountZero,
			}
			lineOrder = append(lineOrder, key)
		}
	}

	var lines []lineItem
	for _, key := range lineOrder {
		lines = append(lines, *lineMap[key])
	}

	c.JSON(http.StatusOK, gin.H{
		"preview": gin.H{
			"amount_due":   totalAmountDue,
			"currency":     string(inv.Currency),
			"period_start": inv.PeriodStart,
			"period_end":   inv.PeriodEnd,
			"lines":        lines,
		},
	})
}

// GetSubscriptionItems GET /billing/ubb/subscription-items
func (h *UBBHandler) GetSubscriptionItems(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var stripeSubID string
	_ = h.DB.QueryRow(
		`SELECT stripe_subscription_id FROM stripe_subscriptions
		 WHERE account_id=? AND status IN ('active','trialing') AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		accountID,
	).Scan(&stripeSubID)

	if stripeSubID == "" || isLocalSubID(stripeSubID) {
		c.JSON(http.StatusOK, gin.H{"items": []any{}})
		return
	}

	params := &stripe.SubscriptionItemListParams{
		Subscription: stripe.String(stripeSubID),
	}
	iter := subscriptionitem.List(params)

	type item struct {
		ID        string `json:"id"`
		PriceID   string `json:"price_id"`
		PriceName string `json:"price_name"`
		UsageType string `json:"usage_type"`
	}
	var items []item
	for iter.Next() {
		si := iter.SubscriptionItem()
		ut := ""
		if si.Price != nil && si.Price.Recurring != nil {
			ut = string(si.Price.Recurring.UsageType)
		}
		priceName := ""
		if si.Price != nil && si.Price.Product != nil {
			priceName = si.Price.Product.Name
		}
		items = append(items, item{
			ID:        si.ID,
			PriceID:   si.Price.ID,
			PriceName: priceName,
			UsageType: ut,
		})
	}
	if items == nil {
		items = []item{}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// RefreshStreamSubItem POST /billing/ubb/streams/:id/refresh-sub-item
// Replaces the existing Stripe sub item (which may have $0/unit price) with a
// new one priced at the stream's actual overage_price_cents.
func (h *UBBHandler) RefreshStreamSubItem(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	streamID := c.Param("id")

	// Read the stream's current sub item and overage price
	var oldSubItemID string
	var overagePriceCents int64
	err := h.DB.QueryRow(
		`SELECT stripe_sub_item_id, overage_price_cents FROM ubb_streams
		 WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		streamID, accountID,
	).Scan(&oldSubItemID, &overagePriceCents)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not found"})
		return
	}
	if overagePriceCents == 0 {
		overagePriceCents = 4
	}

	// Get the active Stripe subscription ID and plan name
	var stripeSubID, planName string
	_ = h.DB.QueryRow(
		`SELECT ss.stripe_subscription_id, sp.name
		 FROM stripe_subscriptions ss
		 JOIN subscription_plans sp ON sp.id=ss.plan_id
		 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC LIMIT 1`,
		accountID,
	).Scan(&stripeSubID, &planName)

	if stripeSubID == "" || isLocalSubID(stripeSubID) {
		// Local plan — just update plan_name, no Stripe sub item needed
		_, _ = h.DB.Exec(
			`UPDATE ubb_streams SET stripe_sub_item_id='', plan_name=? WHERE id=? AND account_id=? AND deleted_at IS NULL`,
			planName, streamID, accountID,
		)
		c.JSON(http.StatusOK, gin.H{"stream_id": streamID, "stripe_sub_item_id": "", "plan_name": planName})
		return
	}

	// Delete the old sub item from Stripe (so we can create a correctly-priced one)
	if oldSubItemID != "" {
		_, _ = subscriptionitem.Del(oldSubItemID, &stripe.SubscriptionItemParams{
			ClearUsage: stripe.Bool(false),
		})
	}

	// Create a new metered sub item with the correct per-unit price
	newSubItemID, newSubItemPriceCents := h.createMeteredSubItem(stripeSubID, planName, overagePriceCents)

	_, dbErr := h.DB.Exec(
		`UPDATE ubb_streams SET stripe_sub_item_id=?, plan_name=?, sub_item_price_cents=? WHERE id=? AND account_id=? AND deleted_at IS NULL`,
		newSubItemID, planName, newSubItemPriceCents, streamID, accountID,
	)
	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"stream_id":          streamID,
		"stripe_sub_item_id": newSubItemID,
		"plan_name":          planName,
	})
}

// DryRunInvoice GET /billing/ubb/invoice/dryrun
// Builds a full invoice breakdown locally.
//
// Billing model:
//   - Stripe-billed streams (sub_item_price_cents > 0, source="stripe"):
//     Stripe charges ALL metered units at the per-unit price — no free-tier
//     deduction. Amount = usage × unit_price_cents. included_units is shown
//     for reference only.
//   - Local-billed streams (no sub item or legacy $0 sub item, source="local"):
//     Apply the local free tier. Amount = max(0, usage - included_units) × overage_price_cents.
func (h *UBBHandler) DryRunInvoice(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var planName string
	var flatFeeCents int64
	_ = h.DB.QueryRow(
		`SELECT sp.name, sp.price_cents
		 FROM stripe_subscriptions ss
		 JOIN subscription_plans sp ON sp.id = ss.plan_id
		 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC LIMIT 1`,
		accountID,
	).Scan(&planName, &flatFeeCents)

	rows, err := h.DB.Query(
		`SELECT id, stream_name, stripe_sub_item_id, included_units, overage_price_cents, sub_item_price_cents
		 FROM ubb_streams WHERE account_id=? AND deleted_at IS NULL`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer rows.Close()

	type streamInfo struct {
		ID                string
		Name              string
		SubItemID         string
		IncludedUnits     int64
		OveragePriceCents int64
		SubItemPriceCents int64
	}
	var streams []streamInfo
	for rows.Next() {
		var s streamInfo
		if err := rows.Scan(&s.ID, &s.Name, &s.SubItemID, &s.IncludedUnits, &s.OveragePriceCents, &s.SubItemPriceCents); err == nil {
			streams = append(streams, s)
		}
	}

	periodStart, periodEnd := h.currentBillingPeriod(accountID)

	type lineItem struct {
		Description   string  `json:"description"`
		AmountUSD     float64 `json:"amount_usd"`
		Units         int64   `json:"units"`
		IncludedUnits int64   `json:"included_units,omitempty"`
		OverageUnits  int64   `json:"overage_units,omitempty"`
		IsOverage     bool    `json:"is_overage"`
		Source        string  `json:"source"` // "stripe" | "local"
	}

	var lines []lineItem
	var totalOverageCents int64

	if flatFeeCents > 0 {
		lines = append(lines, lineItem{
			Description: fmt.Sprintf("%s plan — monthly flat fee", planName),
			AmountUSD:   float64(flatFeeCents) / 100.0,
		})
	}

	for _, s := range streams {
		usage, source := h.resolvedUsage(s.SubItemID, s.ID, s.SubItemPriceCents, periodStart, periodEnd)

		var billedCents int64
		var overageUnits int64

		if source == "stripe" {
			// Stripe: all units billed at sub_item_price_cents (no free-tier deduction)
			billedCents = usage * s.SubItemPriceCents
			overageUnits = usage
		} else {
			// Local: all units billed at overage_price_cents (no free-tier deduction)
			billedCents = usage * s.OveragePriceCents
			overageUnits = usage
		}
		totalOverageCents += billedCents

		lines = append(lines, lineItem{
			Description:   fmt.Sprintf("%s — usage this period", s.Name),
			AmountUSD:     float64(billedCents) / 100.0,
			Units:         usage,
			IncludedUnits: s.IncludedUnits,
			OverageUnits:  overageUnits,
			IsOverage:     billedCents > 0,
			Source:        source,
		})
	}

	now := time.Now()
	totalCents := flatFeeCents + totalOverageCents

	// Add deleted stream revenue for this billing period (not yet invoiced).
	// Only include rows where the stream has actually been deleted — active stream
	// rows in ubb_billed_revenue are already counted in the per-stream loop above.
	billingPeriod := fmt.Sprintf("%d-%02d", now.Year(), int(now.Month()))
	var deletedRevenueCents int64
	_ = h.DB.QueryRow(
		`SELECT COALESCE(SUM(r.overage_cents),0)
		 FROM ubb_billed_revenue r
		 JOIN ubb_streams s ON s.id = r.stream_id
		 WHERE r.account_id=? AND r.billing_period=? AND r.stripe_invoiced=0
		   AND s.deleted_at IS NOT NULL`,
		accountID, billingPeriod,
	).Scan(&deletedRevenueCents)

	if deletedRevenueCents > 0 {
		lines = append(lines, lineItem{
			Description: "Deleted streams — usage charges (this period)",
			AmountUSD:   float64(deletedRevenueCents) / 100.0,
			IsOverage:   true,
			Source:      "local",
		})
		totalCents += deletedRevenueCents
	}

	c.JSON(http.StatusOK, gin.H{
		"dry_run":      true,
		"plan_name":    planName,
		"period":       fmt.Sprintf("%s %d", now.Month().String(), now.Year()),
		"flat_fee_usd": float64(flatFeeCents) / 100.0,
		"overage_usd":  float64(totalOverageCents) / 100.0,
		"total_usd":    float64(totalCents) / 100.0,
		"currency":     "usd",
		"lines":        lines,
		"stream_count": len(streams),
	})
}

// GetNextBillSummary GET /billing/ubb/next-bill
// Returns the projected next bill: flat fee + active stream overages + deleted stream revenue.
func (h *UBBHandler) GetNextBillSummary(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var planName string
	var flatFeeCents int64
	// Try active/trialing subscription first, then fall back to any subscription (covers local subs)
	_ = h.DB.QueryRow(
		`SELECT sp.name, sp.price_cents
		 FROM stripe_subscriptions ss
		 JOIN subscription_plans sp ON sp.id = ss.plan_id
		 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC LIMIT 1`,
		accountID,
	).Scan(&planName, &flatFeeCents)
	if planName == "" {
		_ = h.DB.QueryRow(
			`SELECT sp.name, sp.price_cents
			 FROM stripe_subscriptions ss
			 JOIN subscription_plans sp ON sp.id = ss.plan_id
			 WHERE ss.account_id=? AND ss.deleted_at IS NULL
			 ORDER BY ss.created_at DESC LIMIT 1`,
			accountID,
		).Scan(&planName, &flatFeeCents)
	}

	now := time.Now()
	billingPeriod := fmt.Sprintf("%d-%02d", now.Year(), int(now.Month()))
	periodStart, periodEnd := h.currentBillingPeriod(accountID)

	// Active stream overages — mirror DryRunInvoice billing model:
	// Stripe streams: ALL units × sub_item_price_cents (no free-tier deduction)
	// Local streams:  max(0, usage - included_units) × overage_price_cents
	var activeOverageCents int64
	rows, _ := h.DB.Query(
		`SELECT id, stripe_sub_item_id, included_units, overage_price_cents, sub_item_price_cents
		 FROM ubb_streams WHERE account_id=? AND deleted_at IS NULL`,
		accountID,
	)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var sid, subItemID string
			var included, priceCents, subItemPriceCents int64
			if rows.Scan(&sid, &subItemID, &included, &priceCents, &subItemPriceCents) != nil {
				continue
			}
			usage, source := h.resolvedUsage(subItemID, sid, subItemPriceCents, periodStart, periodEnd)
			if source == "stripe" {
				activeOverageCents += usage * subItemPriceCents
			} else {
				// Local: all units billed at overage_price_cents (no free-tier deduction)
				activeOverageCents += usage * priceCents
			}
		}
	}

	// Deleted stream revenue for this billing period (not yet invoiced).
	// Only rows where the stream is actually deleted — active stream rows are already
	// counted in the activeOverageCents loop above.
	var deletedRevenueCents int64
	_ = h.DB.QueryRow(
		`SELECT COALESCE(SUM(r.overage_cents),0)
		 FROM ubb_billed_revenue r
		 JOIN ubb_streams s ON s.id = r.stream_id
		 WHERE r.account_id=? AND r.billing_period=? AND r.stripe_invoiced=0
		   AND s.deleted_at IS NOT NULL`,
		accountID, billingPeriod,
	).Scan(&deletedRevenueCents)

	totalCents := flatFeeCents + activeOverageCents + deletedRevenueCents

	c.JSON(http.StatusOK, gin.H{
		"plan_name":             planName,
		"billing_period":        billingPeriod,
		"flat_fee_cents":        flatFeeCents,
		"active_overage_cents":  activeOverageCents,
		"deleted_revenue_cents": deletedRevenueCents,
		"total_cents":           totalCents,
		"flat_fee_usd":          float64(flatFeeCents) / 100.0,
		"active_overage_usd":    float64(activeOverageCents) / 100.0,
		"deleted_revenue_usd":   float64(deletedRevenueCents) / 100.0,
		"total_usd":             float64(totalCents) / 100.0,
	})
}

// PayUBBInvoice POST /billing/ubb/invoice/pay
// Only charges streams that have NO Stripe sub item (local-only fallback).
// Streams with a valid sub item are already billed automatically by Stripe's
// subscription at period end — creating a second invoice for them would double-charge.
func (h *UBBHandler) PayUBBInvoice(c *gin.Context) {
	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id required"})
		return
	}

	var stripeCustomerID string
	_ = h.DB.QueryRow(
		`SELECT stripe_customer_id FROM stripe_customers WHERE account_id=?`, accountID,
	).Scan(&stripeCustomerID)
	if stripeCustomerID == "" {
		c.JSON(http.StatusOK, gin.H{
			"paid":      false,
			"total_usd": 0.0,
			"message":   "No Stripe customer found — subscribe to a plan first",
		})
		return
	}

	// Only fetch streams WITHOUT a Stripe sub item (local-only billing).
	// Streams with a sub item are handled by Stripe's subscription automatically.
	rows, err := h.DB.Query(
		`SELECT id, stream_name, included_units, overage_price_cents
		 FROM ubb_streams
		 WHERE account_id=? AND deleted_at IS NULL
		   AND (stripe_sub_item_id IS NULL OR stripe_sub_item_id = '')`,
		accountID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	defer rows.Close()

	type streamInfo struct {
		ID                string
		Name              string
		IncludedUnits     int64
		OveragePriceCents int64
	}
	var streams []streamInfo
	for rows.Next() {
		var s streamInfo
		if err := rows.Scan(&s.ID, &s.Name, &s.IncludedUnits, &s.OveragePriceCents); err == nil {
			streams = append(streams, s)
		}
	}

	if len(streams) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"paid":      false,
			"message":   "All streams are billed via Stripe subscription — no manual payment needed",
			"total_usd": 0.0,
		})
		return
	}

	periodStart, periodEnd := h.currentBillingPeriod(accountID)
	now := time.Now()

	var totalOverageCents int64
	type overageLine struct {
		streamName   string
		overageUnits int64
		cents        int64
	}
	var overageLines []overageLine

	for _, s := range streams {
		var usage int64
		_ = h.DB.QueryRow(
			`SELECT COALESCE(SUM(quantity),0) FROM ubb_usage_events
			 WHERE stream_id=? AND event_ts >= FROM_UNIXTIME(?) AND event_ts < FROM_UNIXTIME(?)`,
			s.ID, periodStart, periodEnd,
		).Scan(&usage)

		if usage > s.IncludedUnits {
			ov := usage - s.IncludedUnits
			cents := ov * s.OveragePriceCents
			totalOverageCents += cents
			overageLines = append(overageLines, overageLine{s.Name, ov, cents})
		}
	}

	if totalOverageCents == 0 {
		c.JSON(http.StatusOK, gin.H{
			"paid":      false,
			"message":   "No overage charges — nothing to pay",
			"total_usd": 0.0,
		})
		return
	}

	// Create invoice items for each overage stream
	for _, ol := range overageLines {
		_, err := invoiceitem.New(&stripe.InvoiceItemParams{
			Customer:    stripe.String(stripeCustomerID),
			Amount:      stripe.Int64(ol.cents),
			Currency:    stripe.String("usd"),
			Description: stripe.String(fmt.Sprintf("UBB overage — %s (%d units)", ol.streamName, ol.overageUnits)),
		})
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"paid":      false,
				"total_usd": float64(totalOverageCents) / 100.0,
				"message":   "Failed to create invoice item for " + ol.streamName + ": " + err.Error(),
			})
			return
		}
	}

	inv, err := invoice.New(&stripe.InvoiceParams{
		Customer:         stripe.String(stripeCustomerID),
		AutoAdvance:      stripe.Bool(false),
		CollectionMethod: stripe.String("charge_automatically"),
		Description:      stripe.String(fmt.Sprintf("UBB overage charges — %s %d", now.Month().String(), now.Year())),
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"paid":      false,
			"total_usd": float64(totalOverageCents) / 100.0,
			"message":   "Failed to create Stripe invoice: " + err.Error(),
		})
		return
	}

	finalInv, err := invoice.FinalizeInvoice(inv.ID, &stripe.InvoiceFinalizeInvoiceParams{})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"paid":       false,
			"invoice_id": inv.ID,
			"total_usd":  float64(totalOverageCents) / 100.0,
			"message":    "Failed to finalize invoice: " + err.Error(),
		})
		return
	}

	paidInv, err := invoice.Pay(finalInv.ID, &stripe.InvoicePayParams{})
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "invoice_already_paid") || strings.Contains(errStr, "Invoice is already paid") {
			fetchedInv, fetchErr := invoice.Get(finalInv.ID, nil)
			if fetchErr == nil && fetchedInv.AmountPaid > 0 {
				c.JSON(http.StatusOK, gin.H{
					"paid":        true,
					"invoice_id":  fetchedInv.ID,
					"invoice_url": fetchedInv.HostedInvoiceURL,
					"pdf_url":     fetchedInv.InvoicePDF,
					"total_usd":   float64(fetchedInv.AmountPaid) / 100.0,
					"status":      string(fetchedInv.Status),
					"message":     "Invoice was already paid",
				})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"paid":        false,
			"invoice_id":  finalInv.ID,
			"invoice_url": finalInv.HostedInvoiceURL,
			"total_usd":   float64(totalOverageCents) / 100.0,
			"message":     extractStripeMessage(err),
		})
		return
	}

	_, _ = h.DB.Exec(
		`INSERT INTO stripe_invoices
		 (id, account_id, stripe_invoice_id, amount_cents, currency, status, invoice_pdf_url)
		 VALUES (UUID(), ?, ?, ?, 'usd', ?, ?)
		 ON DUPLICATE KEY UPDATE status=VALUES(status), invoice_pdf_url=VALUES(invoice_pdf_url)`,
		accountID, paidInv.ID, paidInv.AmountPaid, string(paidInv.Status), paidInv.InvoicePDF,
	)

	logAuditEvent(h.DB, "", accountID, "pay", "ubb_invoice", paidInv.ID, c.ClientIP(), c.Request.UserAgent())

	c.JSON(http.StatusOK, gin.H{
		"paid":        true,
		"invoice_id":  paidInv.ID,
		"invoice_url": paidInv.HostedInvoiceURL,
		"pdf_url":     paidInv.InvoicePDF,
		"total_usd":   float64(paidInv.AmountPaid) / 100.0,
		"status":      string(paidInv.Status),
		"message":     "Payment successful",
	})
}

// SnapshotBilledRevenue upserts the current-period usage for all active streams
// into ubb_billed_revenue. Called periodically (daily goroutine) and on demand
// via POST /billing/ubb/revenue/snapshot. This ensures the revenue ledger is
// always up-to-date even for streams that haven't been deleted yet.
func SnapshotBilledRevenue(db *sql.DB) {
	// Fetch all active streams grouped by account
	rows, err := db.Query(
		`SELECT id, account_id, stream_name, stripe_sub_item_id,
		        included_units, overage_price_cents, sub_item_price_cents
		 FROM ubb_streams WHERE deleted_at IS NULL`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	h := &UBBHandler{DB: db}

	type streamRow struct {
		id, accountID, name, subItemID          string
		included, priceCents, subItemPriceCents int64
	}
	var streams []streamRow
	for rows.Next() {
		var s streamRow
		if rows.Scan(&s.id, &s.accountID, &s.name, &s.subItemID,
			&s.included, &s.priceCents, &s.subItemPriceCents) == nil {
			streams = append(streams, s)
		}
	}

	now := time.Now()
	billingPeriod := fmt.Sprintf("%d-%02d", now.Year(), int(now.Month()))

	for _, s := range streams {
		periodStart, periodEnd := h.currentBillingPeriod(s.accountID)
		usage, _ := h.resolvedUsage(s.subItemID, s.id, s.subItemPriceCents, periodStart, periodEnd)

		var overageUnits, overageCents int64
		if s.subItemID != "" && s.subItemPriceCents > 0 {
			// Stripe stream: all units billed at sub_item_price_cents (no free-tier)
			overageUnits = usage
			overageCents = usage * s.subItemPriceCents
		} else {
			// Local stream: all units billed at overage_price_cents (no free-tier deduction)
			overageUnits = usage
			overageCents = usage * s.priceCents
		}

		// Upsert: one row per (stream_id, billing_period). Use stream_id+period as natural key.
		_, _ = db.Exec(
			`INSERT INTO ubb_billed_revenue
			 (id, account_id, stream_id, stream_name, total_units, included_units,
			  overage_units, overage_price_cents, overage_cents, billing_period)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE
			   total_units=VALUES(total_units),
			   overage_units=VALUES(overage_units),
			   overage_cents=VALUES(overage_cents),
			   stream_name=VALUES(stream_name)`,
			uuid.New().String(), s.accountID, s.id, s.name,
			usage, s.included, overageUnits, s.priceCents, overageCents, billingPeriod,
		)
	}
}

// SnapshotRevenue POST /billing/ubb/revenue/snapshot
// Manually triggers SnapshotBilledRevenue for the calling account (or all accounts for admins).
func (h *UBBHandler) SnapshotRevenue(c *gin.Context) {
	go SnapshotBilledRevenue(h.DB)
	c.JSON(http.StatusOK, gin.H{"message": "snapshot triggered"})
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// resolveStripeContextForStream finds the Stripe customer and picks an UNUSED
// metered sub item for the new stream. Each stream must have its own sub item.
// If all metered sub items are already assigned, subItemID is returned empty
// (stream will fall back to local billing).
func (h *UBBHandler) resolveStripeContextForStream(accountID string, overagePriceCents int64) (customerID, subItemID, planName string, subItemPriceCents int64) {
	_ = h.DB.QueryRow(
		`SELECT stripe_customer_id FROM stripe_customers WHERE account_id=?`, accountID,
	).Scan(&customerID)

	var stripeSubID string
	_ = h.DB.QueryRow(
		`SELECT ss.stripe_subscription_id, sp.name
		 FROM stripe_subscriptions ss
		 JOIN subscription_plans sp ON sp.id=ss.plan_id
		 WHERE ss.account_id=? AND ss.status IN ('active','trialing') AND ss.deleted_at IS NULL
		 ORDER BY ss.created_at DESC LIMIT 1`,
		accountID,
	).Scan(&stripeSubID, &planName)

	if stripeSubID == "" || isLocalSubID(stripeSubID) {
		return
	}

	// Collect sub item IDs already assigned to existing streams
	usedIDs := map[string]bool{}
	rows, _ := h.DB.Query(
		`SELECT stripe_sub_item_id FROM ubb_streams
		 WHERE account_id=? AND deleted_at IS NULL AND stripe_sub_item_id != ''`,
		accountID,
	)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				usedIDs[id] = true
			}
		}
	}

	// Pick the first metered sub item not yet assigned to any stream.
	// Only reuse if it has the correct unit price — otherwise create a new one.
	params := &stripe.SubscriptionItemListParams{
		Subscription: stripe.String(stripeSubID),
	}
	iter := subscriptionitem.List(params)
	for iter.Next() {
		si := iter.SubscriptionItem()
		if si.Price == nil || si.Price.Recurring == nil {
			continue
		}
		if si.Price.Recurring.UsageType != stripe.PriceRecurringUsageTypeMetered {
			continue
		}
		if usedIDs[si.ID] {
			continue
		}
		// Only reuse if the unit price matches what we want
		if si.Price.UnitAmount == overagePriceCents {
			subItemID = si.ID
			subItemPriceCents = si.Price.UnitAmount
			return
		}
	}

	// No matching sub item — create one with the stream's overage price
	subItemID, subItemPriceCents = h.createMeteredSubItem(stripeSubID, planName, overagePriceCents)
	return
}

// createMeteredSubItem creates a metered price on-the-fly and adds it to the subscription.
// overagePriceCents is the per-unit charge in cents (e.g. 4 = $0.04/unit).
// Returns the new sub item ID and the unit price cents that was set.
func (h *UBBHandler) createMeteredSubItem(stripeSubID, planName string, overagePriceCents int64) (string, int64) {
	priceParams := &stripe.PriceParams{
		Currency:   stripe.String("usd"),
		UnitAmount: stripe.Int64(overagePriceCents),
		Recurring: &stripe.PriceRecurringParams{
			Interval:  stripe.String("month"),
			UsageType: stripe.String("metered"),
		},
		ProductData: &stripe.PriceProductDataParams{
			Name: stripe.String("UBB Usage — " + planName),
		},
	}
	newPrice, err := stripePrice(priceParams)
	if err != nil {
		return "", 0
	}

	siParams := &stripe.SubscriptionItemParams{
		Subscription: stripe.String(stripeSubID),
		Price:        stripe.String(newPrice),
	}
	si, err := subscriptionitem.New(siParams)
	if err != nil {
		return "", 0
	}
	return si.ID, overagePriceCents
}

// resolveUsageAuth checks X-API-Key first, then falls back to X-Account-ID.
// Returns the accountID or an error string.
func (h *UBBHandler) resolveUsageAuth(c *gin.Context, streamID string) (string, string) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey != "" {
		var accountID string
		err := h.DB.QueryRow(
			`SELECT account_id FROM ubb_streams WHERE id=? AND api_key=? AND deleted_at IS NULL`,
			streamID, apiKey,
		).Scan(&accountID)
		if err == sql.ErrNoRows {
			return "", "invalid API key for this stream"
		}
		if err != nil {
			return "", "db error"
		}
		return accountID, ""
	}

	accountID := c.GetHeader("X-Account-ID")
	if accountID == "" {
		return "", "X-Account-ID or X-API-Key required"
	}
	return accountID, ""
}

// usageAlreadyRecorded returns true if the idempotency key was already used for this stream.
func (h *UBBHandler) usageAlreadyRecorded(streamID, idemKey string) bool {
	if idemKey == "" {
		return false
	}
	var count int
	_ = h.DB.QueryRow(
		`SELECT COUNT(*) FROM ubb_usage_events WHERE stream_id=? AND idempotency_key=?`,
		streamID, idemKey,
	).Scan(&count)
	return count > 0
}

// currentBillingPeriod returns unix timestamps for the start and end of the
// account's active Stripe subscription period. Falls back to calendar month.
//
// IMPORTANT: We use UNIX_TIMESTAMP() in SQL to avoid Go's timezone
// interpretation of MySQL DATETIME columns. sql.NullTime scans DATETIME as
// local time, causing .Unix() to be off by the server's UTC offset (e.g.
// +5:30 IST = 19800 s), which shifts the period window and excludes events
// that were legitimately recorded within the period.
func (h *UBBHandler) currentBillingPeriod(accountID string) (start, end int64) {
	var periodStart, periodEnd sql.NullInt64
	_ = h.DB.QueryRow(
		`SELECT UNIX_TIMESTAMP(current_period_start), UNIX_TIMESTAMP(current_period_end)
		 FROM stripe_subscriptions
		 WHERE account_id=? AND status IN ('active','trialing') AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT 1`,
		accountID,
	).Scan(&periodStart, &periodEnd)

	if periodStart.Valid && periodEnd.Valid && periodStart.Int64 > 0 {
		return periodStart.Int64, periodEnd.Int64
	}

	// Fallback: calendar month in UTC
	now := time.Now().UTC()
	s := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	e := s.AddDate(0, 1, 0)
	return s.Unix(), e.Unix()
}

// saveLocalUsage inserts a usage event. Table is guaranteed to exist (created at startup).
func (h *UBBHandler) saveLocalUsage(streamID, accountID string, qty, ts int64, action, idemKey string) {
	_, _ = h.DB.Exec(
		`INSERT IGNORE INTO ubb_usage_events
		 (id, stream_id, account_id, quantity, action, idempotency_key, event_ts)
		 VALUES (?, ?, ?, ?, ?, ?, FROM_UNIXTIME(?))`,
		uuid.New().String(), streamID, accountID, qty, action, idemKey, ts,
	)
}

func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ubb_" + hex.EncodeToString(b), nil
}

// extractStripeMessage returns a clean, user-friendly error message from a Stripe error.
func extractStripeMessage(err error) string {
	if err == nil {
		return ""
	}
	stripeErr, ok := err.(*stripe.Error)
	if ok {
		switch stripeErr.Code {
		case "invoice_already_paid":
			return "This invoice has already been paid."
		case "card_declined":
			return "Payment declined — please update your payment method in billing settings."
		case "expired_card":
			return "Your card has expired — please update your payment method."
		case "insufficient_funds":
			return "Insufficient funds — please use a different payment method."
		}
		if stripeErr.Msg != "" {
			return stripeErr.Msg
		}
	}
	msg := err.Error()
	if idx := strings.Index(msg, `"message":"`); idx != -1 {
		start := idx + len(`"message":"`)
		end := strings.Index(msg[start:], `"`)
		if end > 0 {
			return msg[start : start+end]
		}
	}
	if idx := strings.Index(msg, "{"); idx > 0 {
		return strings.TrimSpace(msg[:idx])
	}
	return "Payment failed — use the invoice link to pay manually."
}

// stripePrice wraps price.New to keep the import used and allow easy mocking.
func stripePrice(params *stripe.PriceParams) (string, error) {
	p, err := price.New(params)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// stripeUsageForPeriod queries Stripe usage record summaries for a sub item within a period.
// Returns 0 if no sub item or Stripe returns nothing.
func stripeUsageForPeriod(subItemID string, periodStart, periodEnd int64) int64 {
	if subItemID == "" {
		return 0
	}
	params := &stripe.UsageRecordSummaryListParams{
		SubscriptionItem: stripe.String(subItemID),
	}
	var total int64
	iter := usagerecordsummary.List(params)
	for iter.Next() {
		s := iter.UsageRecordSummary()
		if s.Period.Start >= periodStart && s.Period.Start < periodEnd {
			total += s.TotalUsage
		}
	}
	return total
}

// localUsageForPeriod returns the sum of usage events for a stream within a period.
func (h *UBBHandler) localUsageForPeriod(streamID string, periodStart, periodEnd int64) int64 {
	var total int64
	_ = h.DB.QueryRow(
		`SELECT COALESCE(SUM(quantity),0) FROM ubb_usage_events
		 WHERE stream_id=? AND event_ts >= FROM_UNIXTIME(?) AND event_ts < FROM_UNIXTIME(?)`,
		streamID, periodStart, periodEnd,
	).Scan(&total)
	return total
}

// resolvedUsage returns the authoritative usage for a stream in the current period.
//
// Rules:
//   - If sub item has a real price (sub_item_price_cents > 0): Stripe is the
//     billing source of truth. Use Stripe summaries. Fall back to local only
//     if Stripe returns 0 (e.g. new sub item with no records yet).
//   - If sub item has $0/unit price (legacy): local DB is the only source.
//   - If no sub item: local DB only.
func (h *UBBHandler) resolvedUsage(subItemID, streamID string, subItemPriceCents int64, periodStart, periodEnd int64) (usage int64, source string) {
	if subItemID != "" && subItemPriceCents > 0 {
		// Properly-priced sub item — Stripe is billing source of truth
		stripeUsage := stripeUsageForPeriod(subItemID, periodStart, periodEnd)
		if stripeUsage > 0 {
			return stripeUsage, "stripe"
		}
		// Stripe returned 0 — fall back to local (new sub item, usage not yet aggregated)
		local := h.localUsageForPeriod(streamID, periodStart, periodEnd)
		return local, "local"
	}

	// Legacy $0/unit sub item or no sub item — local DB only
	local := h.localUsageForPeriod(streamID, periodStart, periodEnd)
	return local, "local"
}
