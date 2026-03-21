-- Create cost_recommendations table for FinOps optimization recommendations
CREATE TABLE IF NOT EXISTS cost_recommendations (
    id VARCHAR(36) PRIMARY KEY,
    cloud_account_id VARCHAR(36) NOT NULL,
    recommendation_type VARCHAR(50) NOT NULL, -- idle_resource, oversized_resource, unattached_storage
    resource_id VARCHAR(255) NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    potential_monthly_savings DECIMAL(15, 2) NOT NULL DEFAULT 0.00,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cloud_account_id) REFERENCES cloud_accounts(id) ON DELETE CASCADE,
    INDEX idx_cloud_account_id (cloud_account_id),
    INDEX idx_recommendation_type (recommendation_type),
    INDEX idx_potential_savings (potential_monthly_savings)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
