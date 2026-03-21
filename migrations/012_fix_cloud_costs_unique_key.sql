-- Remove duplicate cloud_costs rows, keeping the most recent per (cloud_account_id, date, service_name, resource_id)
DELETE cc1 FROM cloud_costs cc1
INNER JOIN cloud_costs cc2
  ON cc1.cloud_account_id = cc2.cloud_account_id
  AND cc1.date = cc2.date
  AND cc1.service_name = cc2.service_name
  AND cc1.resource_id = cc2.resource_id
  AND cc1.created_at < cc2.created_at;

-- Add unique constraint only if it doesn't already exist
SET @exists = (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'cloud_costs'
    AND index_name = 'uq_cost_entry'
);

SET @sql = IF(@exists = 0,
  'ALTER TABLE cloud_costs ADD UNIQUE KEY uq_cost_entry (cloud_account_id, date, service_name, resource_id)',
  'SELECT ''uq_cost_entry already exists, skipping'' as info'
);

PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
