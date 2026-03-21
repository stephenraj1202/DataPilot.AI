-- Migration 018: Create ubb_billed_revenue table
-- Permanent revenue ledger for UBB streams.
-- Survives stream deletion — usage/overage is snapshotted here on delete.

CREATE TABLE IF NOT EXISTS ubb_billed_revenue (
    id                  VARCHAR(36)   NOT NULL PRIMARY KEY,
    account_id          VARCHAR(36)   NOT NULL,
    stream_id           VARCHAR(36)   NOT NULL,
    stream_name         VARCHAR(255)  NOT NULL,
    total_units         BIGINT        NOT NULL DEFAULT 0,
    included_units      INT           NOT NULL DEFAULT 0,
    overage_units       BIGINT        NOT NULL DEFAULT 0,
    overage_price_cents INT           NOT NULL DEFAULT 0,
    overage_cents       BIGINT        NOT NULL DEFAULT 0,
    billing_period      VARCHAR(32)   NOT NULL DEFAULT '',
    stripe_invoiced     TINYINT(1)    NOT NULL DEFAULT 0,
    stripe_invoice_id   VARCHAR(255)  NOT NULL DEFAULT '',
    created_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_rev_account (account_id),
    INDEX idx_rev_period  (account_id, billing_period)
);
