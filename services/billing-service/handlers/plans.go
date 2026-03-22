package handlers

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/google/uuid"
)

// Plan represents a subscription plan.
type Plan struct {
	ID                     string
	Name                   string
	PriceCents             int
	StripePriceID          string
	MaxCloudAccounts       *int // nil = unlimited
	MaxDatabaseConnections *int // nil = unlimited
	RateLimitPerMinute     int
	Features               []string
}

var defaultPlans = []Plan{
	{
		Name:                   "free",
		PriceCents:             0,
		StripePriceID:          "",
		MaxCloudAccounts:       intPtr(1),
		MaxDatabaseConnections: intPtr(2),
		RateLimitPerMinute:     100,
		Features:               []string{"30-day trial", "1 cloud account", "2 databases", "100 req/min"},
	},
	{
		Name:                   "base",
		PriceCents:             99900, // ₹999 in paise
		StripePriceID:          "",
		MaxCloudAccounts:       intPtr(3),
		MaxDatabaseConnections: intPtr(5),
		RateLimitPerMinute:     500,
		Features:               []string{"3 cloud accounts", "5 databases", "500 req/min"},
	},
	{
		Name:                   "pro",
		PriceCents:             199900, // ₹1999 in paise
		StripePriceID:          "",
		MaxCloudAccounts:       intPtr(10),
		MaxDatabaseConnections: nil,
		RateLimitPerMinute:     2000,
		Features:               []string{"10 cloud accounts", "unlimited databases", "2000 req/min"},
	},
	{
		Name:                   "enterprise",
		PriceCents:             499900, // ₹4999 in paise
		StripePriceID:          "",
		MaxCloudAccounts:       nil,
		MaxDatabaseConnections: nil,
		RateLimitPerMinute:     10000,
		Features:               []string{"unlimited cloud accounts", "unlimited databases", "10000 req/min"},
	},
}

func intPtr(v int) *int { return &v }

// SeedPlans inserts the default subscription plans if they don't already exist,
// and updates stripe_price_id for existing plans when provided.
func SeedPlans(db *sql.DB, priceIDs ...map[string]string) error {
	prices := map[string]string{}
	if len(priceIDs) > 0 {
		prices = priceIDs[0]
	}

	for _, p := range defaultPlans {
		// Apply price ID from config if provided
		if pid, ok := prices[p.Name]; ok && pid != "" {
			p.StripePriceID = pid
		}

		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM subscription_plans WHERE name = ?`, p.Name).Scan(&count)
		if err != nil {
			return err
		}

		if count > 0 {
			// Update stripe_price_id if we now have one
			if p.StripePriceID != "" {
				_, _ = db.Exec(
					`UPDATE subscription_plans SET stripe_price_id=? WHERE name=? AND (stripe_price_id='' OR stripe_price_id IS NULL)`,
					p.StripePriceID, p.Name,
				)
			}
			// Always sync price_cents to keep INR amounts current
			_, _ = db.Exec(
				`UPDATE subscription_plans SET price_cents=? WHERE name=?`,
				p.PriceCents, p.Name,
			)
			log.Printf("Plan '%s' already exists, updated price_cents=%d", p.Name, p.PriceCents)
			continue
		}

		id := uuid.New().String()
		featuresJSON, _ := json.Marshal(p.Features)

		_, err = db.Exec(
			`INSERT INTO subscription_plans
			 (id, name, price_cents, stripe_price_id, max_cloud_accounts, max_database_connections, rate_limit_per_minute, features)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, p.Name, p.PriceCents, p.StripePriceID,
			p.MaxCloudAccounts, p.MaxDatabaseConnections,
			p.RateLimitPerMinute, string(featuresJSON),
		)
		if err != nil {
			return err
		}
		log.Printf("Seeded plan: %s", p.Name)
	}
	return nil
}

// GetFreePlanID returns the ID of the free plan from the database.
func GetFreePlanID(db *sql.DB) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM subscription_plans WHERE name = 'free' LIMIT 1`).Scan(&id)
	return id, err
}

// GetPlanByName returns a plan record by name.
func GetPlanByName(db *sql.DB, name string) (*Plan, error) {
	row := db.QueryRow(
		`SELECT id, name, price_cents, COALESCE(stripe_price_id,''), max_cloud_accounts, max_database_connections, rate_limit_per_minute
		 FROM subscription_plans WHERE name = ? LIMIT 1`,
		name,
	)
	var p Plan
	err := row.Scan(&p.ID, &p.Name, &p.PriceCents, &p.StripePriceID,
		&p.MaxCloudAccounts, &p.MaxDatabaseConnections, &p.RateLimitPerMinute)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
