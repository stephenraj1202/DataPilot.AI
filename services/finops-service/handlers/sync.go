package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

// SyncScheduler manages the background cost sync job.
type SyncScheduler struct {
	DB     *sql.DB
	AESKey string
}

// Start launches the background goroutine that syncs costs every 6 hours.
func (s *SyncScheduler) Start() {
	go func() {
		// Run immediately on startup, then every 6 hours.
		s.runSync()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.runSync()
		}
	}()
}

// runSync iterates all active cloud accounts and syncs their costs.
func (s *SyncScheduler) runSync() {
	log.Println("[sync] Starting cloud cost sync")

	rows, err := s.DB.Query(
		`SELECT id, provider, encrypted_credentials FROM cloud_accounts WHERE deleted_at IS NULL AND status = 'active'`,
	)
	if err != nil {
		log.Printf("[sync] Failed to query cloud accounts: %v", err)
		return
	}
	defer rows.Close()

	type accountRow struct {
		ID                   string
		Provider             string
		EncryptedCredentials string
	}

	var accounts []accountRow
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(&a.ID, &a.Provider, &a.EncryptedCredentials); err != nil {
			log.Printf("[sync] Failed to scan account row: %v", err)
			continue
		}
		accounts = append(accounts, a)
	}

	for _, acct := range accounts {
		if err := s.syncAccount(acct.ID, acct.Provider, acct.EncryptedCredentials); err != nil {
			log.Printf("[sync] Failed to sync account %s: %v — scheduling retry in 30 minutes", acct.ID, err)
			s.scheduleRetry(acct.ID, acct.Provider, acct.EncryptedCredentials)
			s.updateSyncStatus(acct.ID, "failed")
		} else {
			s.updateSyncStatus(acct.ID, "success")
		}
	}

	log.Println("[sync] Cloud cost sync complete")
}

// scheduleRetry retries a failed account sync after 30 minutes.
func (s *SyncScheduler) scheduleRetry(id, provider, encCreds string) {
	go func() {
		time.Sleep(30 * time.Minute)
		log.Printf("[sync] Retrying sync for account %s", id)
		if err := s.syncAccount(id, provider, encCreds); err != nil {
			log.Printf("[sync] Retry failed for account %s: %v", id, err)
			s.updateSyncStatus(id, "failed")
		} else {
			s.updateSyncStatus(id, "success")
		}
	}()
}

// syncAccount fetches and stores cost data for a single cloud account.
func (s *SyncScheduler) syncAccount(id, provider, encCreds string) error {
	// Decrypt credentials
	credsJSON, err := decrypt(encCreds, s.AESKey)
	if err != nil {
		return err
	}

	var creds map[string]string
	if err := json.Unmarshal([]byte(credsJSON), &creds); err != nil {
		return err
	}

	cp, err := NewCloudProvider(provider)
	if err != nil {
		return err
	}

	// Fetch from start of current month through today — MTD only.
	// Use today's date (not truncated) so the sync always captures the latest partial-day costs.
	now := time.Now().UTC()
	end := now
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	entries, err := cp.FetchCosts(creds, start, end)
	if err != nil {
		return err
	}

	for _, e := range entries {
		costID := uuid.New().String()
		_, err := s.DB.Exec(
			`INSERT INTO cloud_costs (id, cloud_account_id, date, service_name, resource_id, cost_amount, currency, region, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())
			 ON DUPLICATE KEY UPDATE cost_amount = VALUES(cost_amount)`,
			costID, id, e.Date.Format("2006-01-02"), e.Service, e.ResourceID, e.Amount, e.Currency, e.Region,
		)
		if err != nil {
			log.Printf("[sync] Failed to insert cost entry for account %s: %v", id, err)
		}
	}

	return nil
}

// SyncOne triggers an immediate sync for a single cloud account by ID.
// Returns an error if the account is not found or sync fails.
func (s *SyncScheduler) SyncOne(id string) error {
	var provider, encCreds string
	err := s.DB.QueryRow(
		`SELECT provider, encrypted_credentials FROM cloud_accounts WHERE id = ? AND deleted_at IS NULL AND status = 'active'`,
		id,
	).Scan(&provider, &encCreds)
	if err != nil {
		return err
	}
	if err := s.syncAccount(id, provider, encCreds); err != nil {
		s.updateSyncStatus(id, "failed")
		return err
	}
	s.updateSyncStatus(id, "success")
	return nil
}

// updateSyncStatus updates last_sync_at and last_sync_status for a cloud account.
func (s *SyncScheduler) updateSyncStatus(id, status string) {
	_, err := s.DB.Exec(
		`UPDATE cloud_accounts SET last_sync_at = NOW(), last_sync_status = ?, updated_at = NOW() WHERE id = ?`,
		status, id,
	)
	if err != nil {
		log.Printf("[sync] Failed to update sync status for account %s: %v", id, err)
	}
}
