-- 016: Create UBB (usage-based billing) tables

CREATE TABLE IF NOT EXISTS ubb_streams (
    id                    VARCHAR(36)  PRIMARY KEY,
    account_id            VARCHAR(36)  NOT NULL,
    stream_name           VARCHAR(255) NOT NULL,
    resolver_id           VARCHAR(255) NOT NULL,
    api_key               VARCHAR(64)  NOT NULL UNIQUE,
    stripe_sub_item_id    VARCHAR(255) NOT NULL DEFAULT '',
    stripe_customer_id    VARCHAR(255) NOT NULL DEFAULT '',
    plan_name             VARCHAR(64)  NOT NULL DEFAULT '',
    included_units        INT          NOT NULL DEFAULT 1000,
    overage_price_cents   INT          NOT NULL DEFAULT 4,
    status                VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at            DATETIME     NULL,
    INDEX idx_ubb_account (account_id),
    INDEX idx_ubb_api_key (api_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ubb_usage_events (
    id              VARCHAR(36)  PRIMARY KEY,
    stream_id       VARCHAR(36)  NOT NULL,
    account_id      VARCHAR(36)  NOT NULL,
    quantity        BIGINT       NOT NULL,
    action          VARCHAR(16)  NOT NULL DEFAULT 'increment',
    idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
    event_ts        DATETIME     NOT NULL,
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_stream_ts (stream_id, event_ts),
    UNIQUE KEY uq_idempotency (stream_id, idempotency_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
