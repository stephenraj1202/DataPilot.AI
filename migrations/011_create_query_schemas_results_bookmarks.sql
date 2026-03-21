-- Cache extracted schemas per connection (replaces Redis schema caching)
CREATE TABLE IF NOT EXISTS query_schemas (
    id VARCHAR(36) PRIMARY KEY,
    connection_id VARCHAR(36) NOT NULL,
    schema_text LONGTEXT NOT NULL,
    extracted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_connection (connection_id),
    FOREIGN KEY (connection_id) REFERENCES database_connections(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Persist query result sets in MySQL (replaces Redis result caching)
CREATE TABLE IF NOT EXISTS query_results (
    id VARCHAR(36) PRIMARY KEY,
    query_log_id VARCHAR(36) NOT NULL,
    chart_type VARCHAR(50) NOT NULL,
    labels JSON NOT NULL,
    data JSON NOT NULL,
    raw_data LONGTEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (query_log_id) REFERENCES query_logs(id) ON DELETE CASCADE,
    INDEX idx_query_log_id (query_log_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Bookmarks: saved query + chart/table snapshots
CREATE TABLE IF NOT EXISTS query_bookmarks (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    connection_id VARCHAR(36) NOT NULL,
    title VARCHAR(255) NOT NULL,
    query_text TEXT NOT NULL,
    generated_sql TEXT NOT NULL,
    chart_type VARCHAR(50) NOT NULL,
    labels JSON NOT NULL,
    data JSON NOT NULL,
    raw_data LONGTEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (connection_id) REFERENCES database_connections(id) ON DELETE CASCADE,
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
