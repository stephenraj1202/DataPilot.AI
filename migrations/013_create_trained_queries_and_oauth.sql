-- Trained queries: admin-defined NL question → SQL mappings
CREATE TABLE IF NOT EXISTS trained_queries (
    id VARCHAR(36) PRIMARY KEY,
    account_id VARCHAR(36) NOT NULL,
    connection_id VARCHAR(36) NOT NULL,
    question TEXT NOT NULL,
    sql_query TEXT NOT NULL,
    description TEXT,
    created_by VARCHAR(36) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    match_count INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (connection_id) REFERENCES database_connections(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_tq_account_id (account_id),
    INDEX idx_tq_connection_id (connection_id),
    INDEX idx_tq_is_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE users ADD COLUMN oauth_provider VARCHAR(50) DEFAULT NULL;
ALTER TABLE users ADD COLUMN oauth_provider_id VARCHAR(255) DEFAULT NULL;
ALTER TABLE users ADD COLUMN avatar_url VARCHAR(500) DEFAULT NULL;
ALTER TABLE users ADD COLUMN full_name VARCHAR(255) DEFAULT NULL;
ALTER TABLE users ADD COLUMN terms_accepted BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN terms_accepted_at DATETIME DEFAULT NULL;

CREATE INDEX idx_oauth ON users (oauth_provider, oauth_provider_id);
