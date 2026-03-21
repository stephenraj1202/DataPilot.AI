-- 017: UBB idempotency key + indexes
-- Tables already created in 016 with idempotency_key and uq_idempotency.
-- These ALTER statements are kept for environments that may have pre-existing
-- tables created by EnsureUBBTable() before migrations were applied.
-- The migration runner ignores error 1060 (duplicate column) and 1061 (duplicate key).

-- Step 1: add the column (runner ignores 1060 if already exists)
ALTER TABLE ubb_usage_events
  ADD COLUMN idempotency_key VARCHAR(128) NOT NULL DEFAULT '';

-- Step 2: backfill existing rows so every (stream_id, idempotency_key) is unique.
UPDATE ubb_usage_events
  SET idempotency_key = id
  WHERE idempotency_key = '';

-- Step 3: add unique constraint (runner ignores 1061 if exists)
ALTER TABLE ubb_usage_events
  ADD UNIQUE KEY uq_idempotency (stream_id, idempotency_key);

-- Step 4: index on ubb_streams.account_id (runner ignores 1061 if exists)
ALTER TABLE ubb_streams
  ADD INDEX idx_ubb_account (account_id);
