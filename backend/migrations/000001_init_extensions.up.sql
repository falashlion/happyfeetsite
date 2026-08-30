-- =============================================================
-- HappyFeet — Migration 000001
-- Extensions, custom types, and utility functions
-- =============================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";        -- UUID generation
CREATE EXTENSION IF NOT EXISTS "pg_trgm";          -- Trigram search (fuzzy matching)
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements"; -- Query performance monitoring
CREATE EXTENSION IF NOT EXISTS "btree_gin";        -- GIN indexes for composite queries
CREATE EXTENSION IF NOT EXISTS "unaccent";         -- Remove accents for search

-- =============================================================
-- CUSTOM ENUM TYPES
-- =============================================================

CREATE TYPE user_role AS ENUM (
    'customer',
    'vendor',
    'admin',
    'super_admin'
);

CREATE TYPE user_status AS ENUM (
    'pending_verification',
    'active',
    'suspended',
    'deleted'
);

CREATE TYPE product_status AS ENUM (
    'draft',
    'active',
    'archived',
    'out_of_stock'
);

CREATE TYPE order_status AS ENUM (
    'PLACED',
    'PAID',
    'PROCESSING',
    'SHIPPED',
    'DELIVERED',
    'CANCELLED',
    'RETURN_REQUESTED',
    'RETURNED',
    'REFUNDED'
);

CREATE TYPE delivery_method AS ENUM (
    'STANDARD',
    'EXPRESS',
    'SAME_DAY'
);

CREATE TYPE payment_provider AS ENUM (
    'mtn_momo',
    'orange_money',
    'stripe',
    'cinetpay',
    'cash_on_delivery'
);

CREATE TYPE payment_status AS ENUM (
    'INITIATED',
    'PENDING',
    'COMPLETED',
    'FAILED',
    'CANCELLED',
    'REFUNDED',
    'PARTIALLY_REFUNDED'
);

CREATE TYPE vendor_status AS ENUM (
    'pending',
    'active',
    'suspended',
    'rejected'
);

CREATE TYPE notification_channel AS ENUM (
    'push',
    'sms',
    'email',
    'in_app'
);

CREATE TYPE shoe_width AS ENUM (
    'narrow',
    'standard',
    'wide',
    'extra_wide'
);

CREATE TYPE currency_code AS ENUM (
    'XAF',  -- Central African CFA franc
    'XOF',  -- West African CFA franc
    'USD',
    'EUR',
    'GHS',  -- Ghanaian Cedi
    'UGX',  -- Ugandan Shilling
    'NGN',  -- Nigerian Naira
    'KES'   -- Kenyan Shilling
);

CREATE TYPE return_reason AS ENUM (
    'wrong_size',
    'damaged',
    'not_as_described',
    'changed_mind',
    'wrong_item',
    'other'
);

CREATE TYPE discount_type AS ENUM (
    'percentage',
    'fixed_amount',
    'free_shipping',
    'buy_one_get_one'
);

-- =============================================================
-- UTILITY FUNCTIONS
-- =============================================================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Generate URL-friendly slug from text
CREATE OR REPLACE FUNCTION generate_slug(input_text TEXT)
RETURNS TEXT AS $$
BEGIN
    RETURN lower(
        regexp_replace(
            regexp_replace(
                unaccent(input_text),
                '[^a-zA-Z0-9\s-]', '', 'g'
            ),
            '[\s-]+', '-', 'g'
        )
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Generate unique order number
CREATE OR REPLACE FUNCTION generate_order_number()
RETURNS TEXT AS $$
DECLARE
    v_number TEXT;
    v_exists BOOLEAN;
BEGIN
    LOOP
        v_number := 'HF-' || TO_CHAR(NOW(), 'YYYY') || '-' || LPAD(FLOOR(RANDOM() * 100000000)::TEXT, 8, '0');
        SELECT EXISTS(SELECT 1 FROM orders WHERE order_number = v_number) INTO v_exists;
        EXIT WHEN NOT v_exists;
    END LOOP;
    RETURN v_number;
END;
$$ LANGUAGE plpgsql;
