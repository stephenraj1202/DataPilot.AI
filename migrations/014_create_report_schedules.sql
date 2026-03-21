-- Report schedules: users can schedule FinOps cost reports to be emailed
CREATE TABLE IF NOT EXISTS report_schedules (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    created_by VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    frequency ENUM('daily','weekly','monthly') NOT NULL DEFAULT 'weekly',
    -- day_of_week: 0=Sun..6=Sat (used for weekly), day_of_month: 1-28 (used for monthly)
    day_of_week TINYINT DEFAULT NULL,
    day_of_month TINYINT DEFAULT NULL,
    send_hour TINYINT NOT NULL DEFAULT 8,
    recipients JSON NOT NULL COMMENT 'array of email strings',
    report_type ENUM('cost_summary','anomalies','recommendations','full') NOT NULL DEFAULT 'full',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    last_sent_at DATETIME DEFAULT NULL,
    next_run_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_rs_account_id (account_id),
    INDEX idx_rs_next_run (next_run_at),
    INDEX idx_rs_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
