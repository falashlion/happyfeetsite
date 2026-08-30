-- =============================================================
-- HappyFeet — Migration 000004
-- Cart, Orders, Payments, Reviews, Notifications
-- =============================================================

-- =============================================================
-- CART (persistent cart for authenticated users)
-- Guest carts stored in Redis with session token
-- =============================================================
CREATE TABLE carts (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    session_token    VARCHAR(255) UNIQUE,   -- Guest cart token
    coupon_id        UUID REFERENCES promotions(id) ON DELETE SET NULL,
    expires_at       TIMESTAMPTZ,           -- Guest cart TTL (7 days)
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_cart_owner CHECK (
        (user_id IS NOT NULL AND session_token IS NULL) OR
        (user_id IS NULL AND session_token IS NOT NULL)
    )
);

CREATE INDEX idx_carts_user ON carts(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_carts_session ON carts(session_token) WHERE session_token IS NOT NULL;

CREATE TRIGGER set_carts_updated_at
    BEFORE UPDATE ON carts
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

CREATE TABLE cart_items (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cart_id     UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    sku_id      UUID NOT NULL REFERENCES product_skus(id) ON DELETE CASCADE,
    quantity    INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0 AND quantity <= 10),
    unit_price  DECIMAL(12, 2) NOT NULL,   -- Price snapshot at add-to-cart time
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(cart_id, sku_id)
);

CREATE INDEX idx_cart_items_cart ON cart_items(cart_id);

-- =============================================================
-- ORDERS
-- =============================================================
CREATE TABLE orders (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_number     VARCHAR(30) NOT NULL UNIQUE DEFAULT generate_order_number(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status           order_status NOT NULL DEFAULT 'PLACED',

    -- Delivery
    delivery_address JSONB NOT NULL,         -- Snapshot of address at order time
    delivery_method  delivery_method NOT NULL DEFAULT 'STANDARD',
    delivery_fee     DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    estimated_delivery_date DATE,
    tracking_number  VARCHAR(100),
    tracking_url     VARCHAR(500),
    shipped_at       TIMESTAMPTZ,
    delivered_at     TIMESTAMPTZ,

    -- Pricing
    subtotal         DECIMAL(12, 2) NOT NULL,
    discount_amount  DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
    total_amount     DECIMAL(12, 2) NOT NULL,
    currency         currency_code NOT NULL DEFAULT 'XAF',
    coupon_id        UUID REFERENCES promotions(id) ON DELETE SET NULL,

    -- Payment
    payment_method   payment_provider NOT NULL,

    -- Vendor payout
    vendor_id        UUID REFERENCES vendors(id),
    commission_rate  DECIMAL(5, 2),          -- Snapshot at order time
    vendor_payout    DECIMAL(12, 2),
    payout_status    VARCHAR(30) DEFAULT 'pending',

    -- Notes
    customer_notes   TEXT,
    admin_notes      TEXT,

    placed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user ON orders(user_id, placed_at DESC);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_vendor ON orders(vendor_id, placed_at DESC);
CREATE INDEX idx_orders_number ON orders(order_number);
CREATE INDEX idx_orders_placed ON orders(placed_at DESC);

CREATE TRIGGER set_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- ORDER ITEMS
-- =============================================================
CREATE TABLE order_items (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    sku_id          UUID NOT NULL REFERENCES product_skus(id) ON DELETE RESTRICT,
    -- Snapshots at order time (product data may change)
    product_name    VARCHAR(255) NOT NULL,
    sku_code        VARCHAR(100) NOT NULL,
    size_eu         DECIMAL(4, 1) NOT NULL,
    color           VARCHAR(50) NOT NULL,
    primary_image   VARCHAR(1000),
    quantity        INTEGER NOT NULL CHECK (quantity > 0),
    unit_price      DECIMAL(12, 2) NOT NULL,
    subtotal        DECIMAL(12, 2) NOT NULL,
    -- Review tracking
    review_id       UUID,                    -- Populated after review submitted
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_order_items_product ON order_items(product_id);
CREATE INDEX idx_order_items_sku ON order_items(sku_id);

-- =============================================================
-- ORDER STATUS HISTORY
-- =============================================================
CREATE TABLE order_status_history (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status order_status,
    to_status   order_status NOT NULL,
    changed_by  UUID REFERENCES users(id),
    reason      TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_history ON order_status_history(order_id, created_at DESC);

-- =============================================================
-- RETURNS
-- =============================================================
CREATE TABLE returns (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reason        return_reason NOT NULL,
    description   TEXT,
    status        VARCHAR(30) NOT NULL DEFAULT 'PENDING_REVIEW',
    admin_notes   TEXT,
    reviewed_by   UUID REFERENCES users(id),
    reviewed_at   TIMESTAMPTZ,
    label_url     VARCHAR(500),    -- Return shipping label
    refund_id     UUID,            -- Reference to payment refund
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_returns_order ON returns(order_id);
CREATE INDEX idx_returns_user ON returns(user_id);

-- =============================================================
-- PAYMENTS
-- =============================================================
CREATE TABLE payments (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id          UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    provider          payment_provider NOT NULL,
    status            payment_status NOT NULL DEFAULT 'INITIATED',
    amount            DECIMAL(12, 2) NOT NULL,
    currency          currency_code NOT NULL DEFAULT 'XAF',

    -- Idempotency
    idempotency_key   UUID NOT NULL UNIQUE,

    -- Provider-specific
    provider_reference VARCHAR(255),         -- Transaction ID from provider
    provider_payload  JSONB,                 -- Full provider response stored for reconciliation
    phone_number      VARCHAR(25),           -- MoMo/Orange phone
    ussd_string       VARCHAR(100),          -- USSD fallback string

    -- Card-specific
    card_last_four    CHAR(4),
    card_brand        VARCHAR(20),           -- visa, mastercard, etc.

    -- Webhook handling
    webhook_received_at TIMESTAMPTZ,
    webhook_payload   JSONB,

    -- Timing
    initiated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ,           -- Payment session expiry (3 min for MoMo)
    confirmed_at      TIMESTAMPTZ,
    failed_at         TIMESTAMPTZ,
    failure_reason    TEXT,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_order ON payments(order_id);
CREATE INDEX idx_payments_provider_ref ON payments(provider, provider_reference);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_idempotency ON payments(idempotency_key);

CREATE TRIGGER set_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- PAYMENT REFUNDS
-- =============================================================
CREATE TABLE payment_refunds (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_id        UUID NOT NULL REFERENCES payments(id),
    initiated_by      UUID NOT NULL REFERENCES users(id),
    amount            DECIMAL(12, 2) NOT NULL,
    reason            VARCHAR(50) NOT NULL,
    status            VARCHAR(30) NOT NULL DEFAULT 'INITIATED',
    provider_reference VARCHAR(255),
    idempotency_key   UUID NOT NULL UNIQUE,
    initiated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX idx_refunds_payment ON payment_refunds(payment_id);

-- =============================================================
-- REVIEWS
-- =============================================================
CREATE TABLE reviews (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    order_item_id   UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    rating          SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title           VARCHAR(150),
    body            TEXT CHECK (char_length(body) <= 1000),
    status          VARCHAR(30) NOT NULL DEFAULT 'pending', -- pending, approved, rejected
    helpful_count   INTEGER NOT NULL DEFAULT 0,
    moderated_by    UUID REFERENCES users(id),
    moderated_at    TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(order_item_id)  -- One review per order item
);

CREATE INDEX idx_reviews_product ON reviews(product_id, created_at DESC) WHERE status = 'approved';
CREATE INDEX idx_reviews_user ON reviews(user_id);
CREATE INDEX idx_reviews_pending ON reviews(status, created_at) WHERE status = 'pending';

-- =============================================================
-- REVIEW IMAGES
-- =============================================================
CREATE TABLE review_images (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    review_id   UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    url         VARCHAR(1000) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_review_images ON review_images(review_id);

-- =============================================================
-- REVIEW HELPFUL VOTES
-- =============================================================
CREATE TABLE review_helpful_votes (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    review_id   UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, review_id)
);

-- =============================================================
-- NOTIFICATIONS
-- =============================================================
CREATE TABLE notifications (
    id          UUID NOT NULL DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(60) NOT NULL,   -- e.g. 'ORDER_SHIPPED', 'PRICE_DROP', 'RESTOCK'
    title       VARCHAR(200) NOT NULL,
    body        TEXT NOT NULL,
    action_url  VARCHAR(500),           -- Deep link for mobile
    is_read     BOOLEAN NOT NULL DEFAULT false,
    metadata    JSONB,                  -- Extra data (order_id, product_id, etc.)
    sent_via    notification_channel[],
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    read_at     TIMESTAMPTZ,
    -- Partition key must be part of the unique key on partitioned tables.
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE notifications_2026_q1 PARTITION OF notifications
    FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');
CREATE TABLE notifications_2026_q2 PARTITION OF notifications
    FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
CREATE TABLE notifications_2026_q3 PARTITION OF notifications
    FOR VALUES FROM ('2026-07-01') TO ('2026-10-01');
CREATE TABLE notifications_2026_q4 PARTITION OF notifications
    FOR VALUES FROM ('2026-10-01') TO ('2027-01-01');

CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id, is_read) WHERE is_read = false;

-- =============================================================
-- VIEWS FOR COMMON QUERIES
-- =============================================================

-- Active products with primary image and stock summary
CREATE VIEW v_product_catalog AS
SELECT
    p.id,
    p.name,
    p.slug,
    p.base_price,
    p.currency,
    p.rating_avg,
    p.rating_count,
    p.tags,
    p.status,
    c.name AS category_name,
    c.slug AS category_slug,
    b.name AS brand_name,
    b.slug AS brand_slug,
    v.business_name AS vendor_name,
    pi.url_thumbnail,
    pi.url_medium,
    pi.url_large,
    -- Stock summary
    COALESCE(SUM(s.stock_qty - s.reserved_qty), 0) AS total_available_stock,
    COUNT(DISTINCT CASE WHEN s.stock_qty > s.reserved_qty THEN s.size_eu END) AS available_sizes_count,
    ARRAY_AGG(DISTINCT s.size_eu ORDER BY s.size_eu) FILTER (WHERE s.stock_qty > s.reserved_qty) AS available_sizes
FROM products p
JOIN categories c ON p.category_id = c.id
JOIN brands b ON p.brand_id = b.id
JOIN vendors v ON p.vendor_id = v.id
LEFT JOIN product_images pi ON pi.product_id = p.id AND pi.is_primary = true
LEFT JOIN product_skus s ON s.product_id = p.id AND s.is_active = true
WHERE p.deleted_at IS NULL AND p.status = 'active'
GROUP BY p.id, c.name, c.slug, b.name, b.slug, v.business_name, pi.url_thumbnail, pi.url_medium, pi.url_large;

-- Function to update product rating after review approval
CREATE OR REPLACE FUNCTION update_product_rating()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'approved' AND (OLD.status IS NULL OR OLD.status != 'approved') THEN
        UPDATE products SET
            rating_avg = (
                SELECT ROUND(AVG(rating)::NUMERIC, 2)
                FROM reviews
                WHERE product_id = NEW.product_id AND status = 'approved'
            ),
            rating_count = (
                SELECT COUNT(*) FROM reviews
                WHERE product_id = NEW.product_id AND status = 'approved'
            )
        WHERE id = NEW.product_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_rating_on_review
    AFTER INSERT OR UPDATE OF status ON reviews
    FOR EACH ROW EXECUTE FUNCTION update_product_rating();
