-- =============================================================
-- HappyFeet — Migration 000002
-- Users, addresses, sessions, and auth tables
-- =============================================================

-- =============================================================
-- USERS
-- =============================================================
CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email               VARCHAR(255) UNIQUE,
    phone               VARCHAR(25) UNIQUE,  -- E.164 format
    password_hash       VARCHAR(255),        -- Argon2id hash (NULL for social-only)
    first_name          VARCHAR(100) NOT NULL,
    last_name           VARCHAR(100) NOT NULL,
    role                user_role NOT NULL DEFAULT 'customer',
    status              user_status NOT NULL DEFAULT 'pending_verification',
    avatar_url          VARCHAR(500),
    email_verified_at   TIMESTAMPTZ,
    phone_verified_at   TIMESTAMPTZ,
    last_login_at       TIMESTAMPTZ,
    login_attempt_count INTEGER NOT NULL DEFAULT 0,
    locked_until        TIMESTAMPTZ,         -- Brute-force lockout
    mfa_enabled         BOOLEAN NOT NULL DEFAULT false,
    mfa_secret          VARCHAR(64),         -- TOTP secret (encrypted)
    push_token          TEXT,                -- FCM/APNs token for push notifications
    push_platform       VARCHAR(10),         -- 'ios' | 'android'
    preferred_currency  currency_code NOT NULL DEFAULT 'XAF',
    preferred_language  VARCHAR(10) NOT NULL DEFAULT 'en',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,         -- Soft delete (GDPR right to erasure)

    -- Either email or phone must be provided
    CONSTRAINT chk_contact CHECK (email IS NOT NULL OR phone IS NOT NULL)
);

-- Partial index on non-deleted users
CREATE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_phone ON users(phone) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role ON users(role) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_status ON users(status) WHERE deleted_at IS NULL;

CREATE TRIGGER set_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- SOCIAL AUTH PROVIDERS
-- =============================================================
CREATE TABLE user_social_auth (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     VARCHAR(30) NOT NULL,  -- 'google' | 'apple'
    provider_uid VARCHAR(255) NOT NULL,
    access_token TEXT,
    id_token     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(provider, provider_uid)
);

CREATE INDEX idx_social_auth_user ON user_social_auth(user_id);

-- =============================================================
-- OTP (One-Time Passwords)
-- =============================================================
CREATE TABLE otps (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    identifier  VARCHAR(255) NOT NULL,  -- email or phone
    code        VARCHAR(10) NOT NULL,   -- Hashed 6-digit OTP
    purpose     VARCHAR(50) NOT NULL,   -- 'registration', 'login_mfa', 'password_reset', 'phone_verify'
    channel     notification_channel NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_otps_identifier ON otps(identifier, purpose) WHERE used_at IS NULL;
CREATE INDEX idx_otps_expires ON otps(expires_at) WHERE used_at IS NULL;

-- =============================================================
-- REFRESH TOKENS
-- =============================================================
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 of the token
    device_info JSONB,                       -- {userAgent, ip, deviceId}
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- =============================================================
-- USER ADDRESSES
-- =============================================================
CREATE TABLE user_addresses (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label            VARCHAR(50) NOT NULL DEFAULT 'Home',
    recipient_name   VARCHAR(200) NOT NULL,
    recipient_phone  VARCHAR(25),
    street           TEXT NOT NULL,
    city             VARCHAR(100) NOT NULL,
    state            VARCHAR(100),
    postal_code      VARCHAR(20),
    country          CHAR(2) NOT NULL DEFAULT 'CM',  -- ISO 3166-1 alpha-2
    latitude         DECIMAL(10, 7),
    longitude        DECIMAL(10, 7),
    is_default       BOOLEAN NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_addresses_user ON user_addresses(user_id);

-- Ensure only one default address per user
CREATE UNIQUE INDEX idx_addresses_default ON user_addresses(user_id)
    WHERE is_default = true;

CREATE TRIGGER set_addresses_updated_at
    BEFORE UPDATE ON user_addresses
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- AUDIT LOG
-- =============================================================
CREATE TABLE audit_logs (
    id          UUID NOT NULL DEFAULT uuid_generate_v4(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(100) NOT NULL,   -- e.g., 'user.login', 'order.cancel'
    resource    VARCHAR(100),            -- e.g., 'order'
    resource_id UUID,
    ip_address  INET,
    user_agent  TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Partitioned tables require the partition key in any unique constraint.
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Monthly partitions for audit_logs (example for 2026)
CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_logs_2026_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE audit_logs_2026_05 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE audit_logs_2026_06 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
CREATE TABLE audit_logs_2026_07 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
CREATE TABLE audit_logs_2026_08 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');
CREATE TABLE audit_logs_2026_09 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
CREATE TABLE audit_logs_2026_10 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-10-01') TO ('2026-11-01');
CREATE TABLE audit_logs_2026_11 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
CREATE TABLE audit_logs_2026_12 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-12-01') TO ('2027-01-01');

CREATE INDEX idx_audit_user ON audit_logs(user_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_logs(resource, resource_id, created_at DESC);
