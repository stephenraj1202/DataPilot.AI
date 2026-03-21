-- Increase cost_amount precision from DECIMAL(15,2) to DECIMAL(20,10)
-- This allows storing costs as small as $0.0000000001 (10 decimal places)
-- Required for accurate display of micro-costs from AWS/Azure/GCP

ALTER TABLE cloud_costs
    MODIFY COLUMN cost_amount DECIMAL(20, 10) NOT NULL;

ALTER TABLE cost_anomalies
    MODIFY COLUMN baseline_cost DECIMAL(20, 10) NOT NULL,
    MODIFY COLUMN actual_cost   DECIMAL(20, 10) NOT NULL;

ALTER TABLE cost_recommendations
    MODIFY COLUMN potential_monthly_savings DECIMAL(20, 10) NOT NULL DEFAULT 0;
