-- 019: Add sub_item_price_cents to ubb_streams
-- Tracks the unit price (in cents) of the Stripe sub item linked to each stream.
-- 0 = legacy sub item created before per-unit pricing was configured (stale Stripe data).
-- >0 = properly-priced sub item — Stripe summaries can be trusted.
-- Add sub_item_price_cents if it doesn't already exist
-- (MySQL-compatible: uses stored procedure to check information_schema)
SET @col_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME   = 'ubb_streams'
      AND COLUMN_NAME  = 'sub_item_price_cents'
);
SET @sql = IF(@col_exists = 0,
    'ALTER TABLE ubb_streams ADD COLUMN sub_item_price_cents INT NOT NULL DEFAULT 0',
    'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
