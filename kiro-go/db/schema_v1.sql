-- PivotStack PostgreSQL v1 schema.
-- Canonical migration copy is db/migrations/0001_init.sql; keep this file identical.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gin;

CREATE TABLE schema_migrations (
    version         integer PRIMARY KEY,
    name            text NOT NULL,
    checksum_sha256 text NOT NULL,
    applied_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE migration_imports (
    source_name     text NOT NULL,
    legacy_id       text NOT NULL,
    payload_sha256  text NOT NULL,
    imported_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (source_name, legacy_id)
);

CREATE TABLE settings_kv (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by text
);

CREATE TABLE pricing_config (
    singleton_id boolean PRIMARY KEY DEFAULT true CHECK (singleton_id),
    payload      jsonb NOT NULL,
    version      integer NOT NULL DEFAULT 2,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    updated_by   text
);

CREATE TABLE stealth_config (
    singleton_id boolean PRIMARY KEY DEFAULT true CHECK (singleton_id),
    payload      jsonb NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    updated_by   text
);

CREATE TABLE promotion_config (
    singleton_id boolean PRIMARY KEY DEFAULT true CHECK (singleton_id),
    payload      jsonb NOT NULL,
    enabled      boolean NOT NULL DEFAULT false,
    start_at     timestamptz,
    end_at       timestamptz,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    updated_by   text
);

CREATE TABLE users (
    id              text PRIMARY KEY,
    email           text NOT NULL,
    email_norm      text NOT NULL UNIQUE,
    username        text NOT NULL UNIQUE,
    password_hash   text NOT NULL,
    default_key_id  text,
    invited_by      text,
    inviter_user_id text,
    created_at      timestamptz NOT NULL,
    last_login_at   timestamptz,
    disabled        boolean NOT NULL DEFAULT false,
    schema_version  integer NOT NULL DEFAULT 3,
    metadata        jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE user_wallets (
    user_id         text PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance         numeric(20,8) NOT NULL DEFAULT 0,
    gift_balance    numeric(20,8) NOT NULL DEFAULT 0,
    total_recharged numeric(20,8) NOT NULL DEFAULT 0,
    total_gifted    numeric(20,8) NOT NULL DEFAULT 0,
    version         bigint NOT NULL DEFAULT 0,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (balance >= 0),
    CHECK (gift_balance >= 0),
    CHECK (total_recharged >= 0),
    CHECK (total_gifted >= 0)
);

CREATE TABLE api_keys (
    id                   text PRIMARY KEY,
    key_hash             bytea NOT NULL UNIQUE,
    key_ciphertext       text NOT NULL,
    tier                 text,
    plan                 text NOT NULL DEFAULT 'credit',
    expires_at           timestamptz,
    enabled              boolean NOT NULL DEFAULT true,

    -- Legacy wallet: only orphan keys and reseller child keys should use these.
    balance              numeric(20,8) NOT NULL DEFAULT 0,
    gift_balance         numeric(20,8) NOT NULL DEFAULT 0,
    total_recharged      numeric(20,8) NOT NULL DEFAULT 0,
    total_gifted         numeric(20,8) NOT NULL DEFAULT 0,

    note                 text,
    created_at           timestamptz NOT NULL,
    last_used            timestamptz,
    requests             bigint NOT NULL DEFAULT 0,
    errors               bigint NOT NULL DEFAULT 0,
    tokens               bigint NOT NULL DEFAULT 0,
    credits              numeric(20,8) NOT NULL DEFAULT 0,
    models               jsonb NOT NULL DEFAULT '{}'::jsonb,

    parent_key_id        text REFERENCES api_keys(id) ON DELETE RESTRICT,
    is_reseller          boolean NOT NULL DEFAULT false,
    max_child_keys       integer NOT NULL DEFAULT 0,
    reseller_discount    numeric(12,6) NOT NULL DEFAULT 0,
    sold_to_children     numeric(20,8) NOT NULL DEFAULT 0,
    rate_limit_per_min   integer NOT NULL DEFAULT 0,
    series_preferences   jsonb NOT NULL DEFAULT '{}'::jsonb,
    channel_preferences  jsonb NOT NULL DEFAULT '{}'::jsonb,
    debt_usd             numeric(20,8) NOT NULL DEFAULT 0,

    deleted_at           timestamptz,
    metadata             jsonb NOT NULL DEFAULT '{}'::jsonb,

    CHECK (balance >= 0),
    CHECK (gift_balance >= 0),
    CHECK (max_child_keys >= 0),
    CHECK (rate_limit_per_min >= 0)
);

ALTER TABLE users
    ADD CONSTRAINT users_default_key_fk
    FOREIGN KEY (default_key_id) REFERENCES api_keys(id) ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE user_api_keys (
    user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id text NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    bound_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, api_key_id),
    UNIQUE (api_key_id)
);

CREATE TABLE accounts (
    id                    text PRIMARY KEY,
    email                 text,
    email_norm            text,
    user_id               text,
    nickname              text,

    access_token_enc      text,
    refresh_token_enc     text,
    client_id             text,
    client_secret_enc     text,
    auth_method           text NOT NULL DEFAULT 'idc',
    provider              text,
    region                text,
    start_url             text,
    expires_at            timestamptz,
    machine_id            text,

    weight                integer NOT NULL DEFAULT 0,
    enabled               boolean NOT NULL DEFAULT true,
    allow_over_quota      boolean NOT NULL DEFAULT false,
    ban_status            text,
    ban_reason            text,
    ban_time              timestamptz,

    subscription_type     text,
    subscription_title    text,
    days_remaining        integer NOT NULL DEFAULT 0,
    usage_current         numeric(20,8) NOT NULL DEFAULT 0,
    usage_limit           numeric(20,8) NOT NULL DEFAULT 0,
    usage_percent         numeric(12,8) NOT NULL DEFAULT 0,
    next_reset_date       text,
    last_refresh          timestamptz,

    trial_usage_current   numeric(20,8) NOT NULL DEFAULT 0,
    trial_usage_limit     numeric(20,8) NOT NULL DEFAULT 0,
    trial_usage_percent   numeric(12,8) NOT NULL DEFAULT 0,
    trial_status          text,
    trial_expires_at      timestamptz,

    request_count         bigint NOT NULL DEFAULT 0,
    error_count           bigint NOT NULL DEFAULT 0,
    last_used             timestamptz,
    total_tokens          bigint NOT NULL DEFAULT 0,
    total_credits         numeric(20,8) NOT NULL DEFAULT 0,
    deleted_at            timestamptz,
    metadata              jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX accounts_identity_uq
    ON accounts (coalesce(auth_method, ''), coalesce(email_norm, ''), coalesce(provider, ''))
    WHERE deleted_at IS NULL AND email_norm IS NOT NULL AND email_norm <> '';

CREATE TABLE series (
    id                 text PRIMARY KEY,
    name               text NOT NULL,
    default_channel_id text,
    model_patterns     jsonb NOT NULL DEFAULT '[]'::jsonb,
    sort_order         integer NOT NULL DEFAULT 0
);

CREATE TABLE legacy_channels (
    id             text PRIMARY KEY,
    type           text NOT NULL,
    series_id      text REFERENCES series(id) ON DELETE SET NULL,
    base_url       text,
    api_key_enc    text,
    models         jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_prices   jsonb NOT NULL DEFAULT '{}'::jsonb,
    model_aliases  jsonb NOT NULL DEFAULT '{}'::jsonb,
    extra_headers  jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled        boolean NOT NULL DEFAULT true
);

CREATE TABLE direct_channels (
    id              text PRIMARY KEY,
    type            text NOT NULL CHECK (type IN ('openai', 'kiro')),
    alias           text NOT NULL,
    alias_norm      text NOT NULL,
    base_url        text,
    api_key_enc     text,
    models          jsonb NOT NULL DEFAULT '[]'::jsonb,
    sell_price      jsonb NOT NULL DEFAULT '{}'::jsonb,
    model_mapping   jsonb NOT NULL DEFAULT '{}'::jsonb,
    extra_headers   jsonb NOT NULL DEFAULT '{}'::jsonb,
    enabled         boolean NOT NULL DEFAULT true,
    status          text,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    deleted_at      timestamptz,
    UNIQUE (alias_norm)
);

-- TODO: The plan preserves cross-table alias uniqueness between direct and newapi channels in application validation.
CREATE TABLE newapi_providers (
    id                         text PRIMARY KEY,
    name                       text NOT NULL,
    base_url                   text NOT NULL,
    username                   text NOT NULL,
    password_enc               text,
    access_token_enc           text,
    access_token_expires_at    timestamptz,
    upstream_user_id           integer,
    quota_per_unit_dollar      numeric(20,8) NOT NULL,
    yuan_per_upstream_dollar   numeric(20,8) NOT NULL,
    last_sync_at               timestamptz,
    last_sync_error            text,
    sync_interval_sec          integer NOT NULL DEFAULT 0,
    enabled                    boolean NOT NULL DEFAULT true,
    metadata                   jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE newapi_channels (
    id                   text PRIMARY KEY,
    provider_id          text NOT NULL REFERENCES newapi_providers(id) ON DELETE RESTRICT,
    alias                text NOT NULL,
    alias_norm           text NOT NULL,
    upstream_token_id    integer NOT NULL,
    upstream_key_enc     text,
    upstream_token_name  text,
    group_name           text NOT NULL,
    models               jsonb NOT NULL DEFAULT '[]'::jsonb,
    markup               numeric(20,8) NOT NULL,
    series_id            text REFERENCES series(id) ON DELETE SET NULL,
    create_mode          text,
    enabled              boolean NOT NULL DEFAULT true,
    remain_quota         bigint NOT NULL DEFAULT 0,
    unlimited_quota      boolean NOT NULL DEFAULT false,
    status               integer NOT NULL DEFAULT 0,
    created_at           timestamptz,
    updated_at           timestamptz,
    last_seen_at         timestamptz,
    deleted_at           timestamptz,
    UNIQUE (alias_norm),
    CHECK (markup > 0)
);

CREATE TABLE channel_groups (
    id                         text PRIMARY KEY,
    name                       text NOT NULL,
    description                text,
    enabled                    boolean NOT NULL DEFAULT true,
    model_patterns             jsonb NOT NULL DEFAULT '[]'::jsonb,
    default_runtime_channel_id text,
    sort_order                 integer NOT NULL DEFAULT 0,
    created_at                 timestamptz NOT NULL,
    updated_at                 timestamptz NOT NULL,
    deleted_at                 timestamptz
);

CREATE TABLE channel_group_members (
    group_id    text NOT NULL REFERENCES channel_groups(id) ON DELETE CASCADE,
    source_type text NOT NULL CHECK (source_type IN ('newapi', 'direct')),
    channel_id  text NOT NULL,
    sort_order  integer NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, source_type, channel_id)
);

CREATE TABLE activation_codes (
    code               text PRIMARY KEY,
    type               text NOT NULL CHECK (type IN ('balance', 'days', 'time')),
    amount             numeric(20,8) NOT NULL,
    tier               text,
    code_expires_at    timestamptz,
    used               boolean NOT NULL DEFAULT false,
    used_by_key_id     text REFERENCES api_keys(id) ON DELETE SET NULL,
    used_at            timestamptz,
    created_at         timestamptz NOT NULL,
    note               text,
    rate_limit_per_min integer NOT NULL DEFAULT 0,
    sale_price_cny     numeric(20,8) NOT NULL DEFAULT 0
);

CREATE TABLE wallet_ledger (
    id              text PRIMARY KEY,
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    api_key_id      text NOT NULL,
    owner_type      text NOT NULL CHECK (owner_type IN ('user', 'api_key')),
    owner_id        text NOT NULL,
    operation       text NOT NULL,
    reservation_id  text,
    request_id      text,
    paid_delta      numeric(20,8) NOT NULL DEFAULT 0,
    gift_delta      numeric(20,8) NOT NULL DEFAULT 0,
    paid_after      numeric(20,8) NOT NULL DEFAULT 0,
    gift_after      numeric(20,8) NOT NULL DEFAULT 0,
    metadata        jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE billing_reservations (
    id              text PRIMARY KEY,
    request_id      text,
    api_key_id      text NOT NULL,
    owner_type      text NOT NULL CHECK (owner_type IN ('user', 'api_key')),
    owner_id        text NOT NULL,
    channel_id      text,
    model           text,
    status          text NOT NULL CHECK (status IN ('pending', 'reconciled', 'refunded', 'expired')),
    action          text NOT NULL,
    est_cost_usd    numeric(20,8) NOT NULL DEFAULT 0,
    pre_paid_usd    numeric(20,8) NOT NULL DEFAULT 0,
    pre_gift_usd    numeric(20,8) NOT NULL DEFAULT 0,
    actual_cost_usd numeric(20,8),
    price_snapshot  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    settled_at      timestamptz
);

CREATE UNIQUE INDEX billing_reservations_request_id_uq
    ON billing_reservations (request_id)
    WHERE request_id IS NOT NULL;

CREATE TABLE recharge_records (
    id             text PRIMARY KEY,
    time_label     text NOT NULL,
    timestamp_unix bigint NOT NULL,
    occurred_at    timestamptz NOT NULL,
    day_cst        date NOT NULL,

    api_key_id     text NOT NULL,
    user_id        text REFERENCES users(id) ON DELETE SET NULL,
    key_note       text,
    type           text NOT NULL,
    code           text,

    amount_usd     numeric(20,8) NOT NULL DEFAULT 0,
    amount_cny     numeric(20,8) NOT NULL DEFAULT 0,
    balance_before numeric(20,8) NOT NULL DEFAULT 0,
    balance_after  numeric(20,8) NOT NULL DEFAULT 0,
    gift_before    numeric(20,8) NOT NULL DEFAULT 0,
    gift_after     numeric(20,8) NOT NULL DEFAULT 0,

    operator       text NOT NULL,
    note           text,
    ip             text,
    raw_payload    jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE call_logs (
    id               text NOT NULL,
    occurred_at      timestamptz NOT NULL,
    timestamp_unix   bigint NOT NULL,
    day_cst          date NOT NULL,
    time_label       text NOT NULL,

    request_id       text,
    api_type         text NOT NULL,
    original_model   text,
    actual_model     text,
    account          text,
    api_key_id       text,

    input_tokens     integer NOT NULL DEFAULT 0,
    output_tokens    integer NOT NULL DEFAULT 0,
    total_tokens     integer NOT NULL DEFAULT 0,
    credits          numeric(20,8) NOT NULL DEFAULT 0,
    upstream_credits numeric(20,8) NOT NULL DEFAULT 0,
    paid_credits     numeric(20,8) NOT NULL DEFAULT 0,
    gifted_credits   numeric(20,8) NOT NULL DEFAULT 0,
    cost_usd         numeric(20,8) NOT NULL DEFAULT 0,
    charged_usd      numeric(20,8) NOT NULL DEFAULT 0,
    cost_usd_legacy  numeric(20,8) NOT NULL DEFAULT 0,

    price_model      text,
    stream           boolean NOT NULL DEFAULT false,
    error            text,
    payload_kb       integer NOT NULL DEFAULT 0,
    status           text NOT NULL,
    stop_reason      text,
    duration_ms      bigint NOT NULL DEFAULT 0,
    attempt          integer NOT NULL DEFAULT 0,
    subscription     text,
    request_summary  text,
    response_summary text,

    channel_id       text,
    channel_type     text,
    billing_mode     text,
    billing_status   text,
    usage_estimated  boolean NOT NULL DEFAULT false,

    raw_payload      jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

CREATE TABLE call_logs_2026_05 PARTITION OF call_logs
    FOR VALUES FROM ('2026-05-01 00:00:00+00') TO ('2026-06-01 00:00:00+00');
CREATE TABLE call_logs_2026_06 PARTITION OF call_logs
    FOR VALUES FROM ('2026-06-01 00:00:00+00') TO ('2026-07-01 00:00:00+00');
CREATE TABLE call_logs_2026_07 PARTITION OF call_logs
    FOR VALUES FROM ('2026-07-01 00:00:00+00') TO ('2026-08-01 00:00:00+00');

CREATE TABLE call_log_reconcile_events (
    id              text PRIMARY KEY,
    request_id      text NOT NULL,
    billing_status  text NOT NULL,
    upstream_quota  bigint NOT NULL DEFAULT 0,
    paid_delta      numeric(20,8) NOT NULL DEFAULT 0,
    gift_delta      numeric(20,8) NOT NULL DEFAULT 0,
    debt_delta      numeric(20,8) NOT NULL DEFAULT 0,
    occurred_at     timestamptz NOT NULL,
    raw_payload     jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE audit_logs (
    id           text PRIMARY KEY,
    occurred_at  timestamptz NOT NULL,
    action       text,
    operator     text,
    detail       text,
    raw_line     text,
    fields       jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_api_keys_parent ON api_keys(parent_key_id);
CREATE INDEX idx_api_keys_enabled ON api_keys(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_keys_last_used ON api_keys(last_used DESC);
CREATE INDEX idx_api_keys_channel_preferences_gin ON api_keys USING gin(channel_preferences);

CREATE INDEX idx_user_api_keys_key ON user_api_keys(api_key_id);
CREATE INDEX idx_user_wallets_updated ON user_wallets(updated_at DESC);

CREATE INDEX idx_recharge_records_timestamp ON recharge_records(occurred_at DESC);
CREATE INDEX idx_recharge_records_user_id ON recharge_records(user_id, occurred_at DESC);
CREATE INDEX idx_recharge_records_key_id ON recharge_records(api_key_id, occurred_at DESC);
CREATE INDEX idx_recharge_records_type ON recharge_records(type, occurred_at DESC);
CREATE INDEX idx_recharge_records_day_cst ON recharge_records(day_cst);

CREATE INDEX idx_wallet_ledger_owner ON wallet_ledger(owner_type, owner_id, occurred_at DESC);
CREATE INDEX idx_wallet_ledger_key ON wallet_ledger(api_key_id, occurred_at DESC);
CREATE INDEX idx_billing_reservations_key ON billing_reservations(api_key_id, created_at DESC);
CREATE INDEX idx_billing_reservations_status ON billing_reservations(status, created_at DESC);

CREATE INDEX idx_call_logs_ts_channel ON call_logs(occurred_at DESC, channel_id);
CREATE INDEX idx_call_logs_api_key ON call_logs(api_key_id, occurred_at DESC);
CREATE INDEX idx_call_logs_status ON call_logs(status, occurred_at DESC);
CREATE INDEX idx_call_logs_request_id ON call_logs(request_id);
CREATE INDEX idx_call_logs_day_cst ON call_logs(day_cst);
CREATE INDEX idx_call_logs_channel_model_day ON call_logs(channel_id, price_model, day_cst);

CREATE INDEX idx_reconcile_request_id ON call_log_reconcile_events(request_id);
CREATE INDEX idx_audit_logs_action_time ON audit_logs(action, occurred_at DESC);
CREATE INDEX idx_activation_codes_used ON activation_codes(used, code_expires_at);
CREATE INDEX idx_newapi_channels_provider ON newapi_channels(provider_id);
CREATE INDEX idx_channel_group_members_channel ON channel_group_members(source_type, channel_id);

CREATE INDEX idx_pricing_payload_gin ON pricing_config USING gin(payload);
CREATE INDEX idx_stealth_payload_gin ON stealth_config USING gin(payload);
CREATE INDEX idx_promotion_payload_gin ON promotion_config USING gin(payload);
