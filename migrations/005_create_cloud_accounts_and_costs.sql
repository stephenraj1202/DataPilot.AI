-- Create cloud_accounts table
CREATE TABLE IF NOT EXISTS cloud_accounts (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    provider VARCHAR(20) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    encrypted_credentials TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    last_sync_at DATETIME,
    last_sync_status VARCHAR(50),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    INDEX idx_account_id (account_id),
    INDEX idx_provider (provider),
    INDEX idx_last_sync_at (last_sync_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create cloud_costs table
CREATE TABLE IF NOT EXISTS cloud_costs (
    id VARCHAR(36) PRIMARY KEY,
    cloud_account_id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    resource_id VARCHAR(255),
    cost_amount DECIMAL(15, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    region VARCHAR(50),
    tags JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cloud_account_id) REFERENCES cloud_accounts(id) ON DELETE CASCADE,
    INDEX idx_cloud_account_date (cloud_account_id, date),
    INDEX idx_date (date),
    INDEX idx_service_name (service_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create cost_anomalies table
CREATE TABLE IF NOT EXISTS cost_anomalies (
    id VARCHAR(36) PRIMARY KEY,
    cloud_account_id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    baseline_cost DECIMAL(15, 2) NOT NULL,
    actual_cost DECIMAL(15, 2) NOT NULL,
    deviation_percentage DECIMAL(5, 2) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    contributing_services JSON,
    acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_by VARCHAR(36),
    acknowledged_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cloud_account_id) REFERENCES cloud_accounts(id) ON DELETE CASCADE,
    INDEX idx_cloud_account_date (cloud_account_id, date),
    INDEX idx_severity (severity),
    INDEX idx_acknowledged (acknowledged)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
