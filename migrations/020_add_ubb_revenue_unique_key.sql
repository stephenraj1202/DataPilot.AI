-- Migration 020: Add unique key on ubb_billed_revenue(stream_id, billing_period)
-- Required for ON DUPLICATE KEY UPDATE upserts in SnapshotBilledRevenue.

SET @_idx = (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'ubb_billed_revenue'
    AND INDEX_NAME = 'uq_stream_period'
);
SET @_sql = IF(@_idx = 0,
  'ALTER TABLE ubb_billed_revenue ADD UNIQUE KEY uq_stream_period (stream_id, billing_period)',
  'SELECT 1'
);
PREPARE _stmt FROM @_sql;
EXECUTE _stmt;
DEALLOCATE PREPARE _stmt;
