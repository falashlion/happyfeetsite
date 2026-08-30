-- =============================================================
-- HappyFeet — Migration 000003
-- Vendors, categories, brands, products, SKUs, images
-- =============================================================

-- =============================================================
-- VENDORS
-- =============================================================
CREATE TABLE vendors (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id         UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    business_name    VARCHAR(255) NOT NULL,
    business_type    VARCHAR(30) NOT NULL DEFAULT 'individual',
    description      TEXT,
    website          VARCHAR(500),
    status           vendor_status NOT NULL DEFAULT 'pending',
    commission_rate  DECIMAL(5, 2) NOT NULL DEFAULT 8.50,  -- Platform commission %
    rating_avg       DECIMAL(3, 2) NOT NULL DEFAULT 0.00,
    rating_count     INTEGER NOT NULL DEFAULT 0,
    total_sales      DECIMAL(15, 2) NOT NULL DEFAULT 0.00,
    payout_phone     VARCHAR(25),               -- MoMo payout number
    payout_provider  payment_provider,
    rejection_reason TEXT,
    approved_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_vendors_owner ON vendors(owner_id);
CREATE INDEX idx_vendors_status ON vendors(status);

CREATE TRIGGER set_vendors_updated_at
    BEFORE UPDATE ON vendors
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- CATEGORIES (hierarchical, self-referencing)
-- =============================================================
CREATE TABLE categories (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_id    UUID REFERENCES categories(id) ON DELETE SET NULL,
    name         VARCHAR(100) NOT NULL,
    slug         VARCHAR(120) NOT NULL UNIQUE,
    description  TEXT,
    icon_url     VARCHAR(500),
    banner_url   VARCHAR(500),
    sort_order   INTEGER NOT NULL DEFAULT 0,
    is_active    BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed core categories
INSERT INTO categories (id, name, slug, sort_order) VALUES
    (uuid_generate_v4(), 'Men''s Shoes',     'mens-shoes',     1),
    (uuid_generate_v4(), 'Women''s Shoes',   'womens-shoes',   2),
    (uuid_generate_v4(), 'Kids'' Shoes',     'kids-shoes',     3),
    (uuid_generate_v4(), 'Sports & Athletic','sports-athletic', 4),
    (uuid_generate_v4(), 'Formal & Dress',   'formal-dress',   5),
    (uuid_generate_v4(), 'Casual',           'casual',         6),
    (uuid_generate_v4(), 'Sandals & Flip-Flops', 'sandals',   7),
    (uuid_generate_v4(), 'Boots',            'boots',          8),
    (uuid_generate_v4(), 'Sneakers',         'sneakers',       9),
    (uuid_generate_v4(), 'Loafers',          'loafers',        10);

-- =============================================================
-- BRANDS
-- =============================================================
CREATE TABLE brands (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    slug        VARCHAR(120) NOT NULL UNIQUE,
    logo_url    VARCHAR(500),
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO brands (name, slug) VALUES
    ('Nike', 'nike'), ('Adidas', 'adidas'), ('Puma', 'puma'),
    ('New Balance', 'new-balance'), ('Reebok', 'reebok'),
    ('Vans', 'vans'), ('Converse', 'converse'), ('Timberland', 'timberland'),
    ('Dr. Martens', 'dr-martens'), ('Clarks', 'clarks'),
    ('ECCO', 'ecco'), ('Birkenstock', 'birkenstock'),
    ('Skechers', 'skechers'), ('Asics', 'asics'), ('Under Armour', 'under-armour');

-- =============================================================
-- PRODUCTS
-- =============================================================
CREATE TABLE products (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    vendor_id      UUID NOT NULL REFERENCES vendors(id) ON DELETE RESTRICT,
    category_id    UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    brand_id       UUID NOT NULL REFERENCES brands(id) ON DELETE RESTRICT,
    name           VARCHAR(255) NOT NULL,
    slug           VARCHAR(300) NOT NULL UNIQUE,
    description    TEXT,
    base_price     DECIMAL(12, 2) NOT NULL CHECK (base_price >= 0),
    currency       currency_code NOT NULL DEFAULT 'XAF',
    status         product_status NOT NULL DEFAULT 'draft',
    tags           TEXT[] NOT NULL DEFAULT '{}',
    return_policy  TEXT DEFAULT '14-day returns accepted',
    rating_avg     DECIMAL(3, 2) NOT NULL DEFAULT 0.00,
    rating_count   INTEGER NOT NULL DEFAULT 0,

    -- Full-text search vector (auto-updated by trigger)
    search_vector  TSVECTOR,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ   -- Soft delete
);

-- Indexes
CREATE INDEX idx_products_vendor ON products(vendor_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_category ON products(category_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX idx_products_brand ON products(brand_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX idx_products_status ON products(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_search ON products USING GIN(search_vector);
CREATE INDEX idx_products_tags ON products USING GIN(tags);
CREATE INDEX idx_products_price ON products(base_price) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX idx_products_rating ON products(rating_avg DESC) WHERE deleted_at IS NULL AND status = 'active';

-- Auto-update search vector
CREATE OR REPLACE FUNCTION update_product_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', coalesce(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(NEW.description, '')), 'B') ||
        setweight(to_tsvector('english', coalesce(array_to_string(NEW.tags, ' '), '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_products_search_vector
    BEFORE INSERT OR UPDATE OF name, description, tags ON products
    FOR EACH ROW EXECUTE FUNCTION update_product_search_vector();

CREATE TRIGGER set_products_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- PRODUCT SKUs (Stock Keeping Units)
-- Each unique size + color combination = 1 SKU
-- =============================================================
CREATE TABLE product_skus (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id        UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku_code          VARCHAR(100) NOT NULL UNIQUE,
    size_eu           DECIMAL(4, 1) NOT NULL,   -- EU size (e.g. 42, 42.5)
    size_us           DECIMAL(4, 1),
    size_uk           DECIMAL(4, 1),
    color             VARCHAR(50) NOT NULL,
    color_hex         CHAR(7),                  -- e.g. #000000
    width             shoe_width NOT NULL DEFAULT 'standard',
    stock_qty         INTEGER NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
    reserved_qty      INTEGER NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),
    low_stock_alert   INTEGER NOT NULL DEFAULT 5, -- Alert when stock <= this
    additional_price  DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT available_stock CHECK (stock_qty >= reserved_qty)
);

CREATE INDEX idx_skus_product ON product_skus(product_id) WHERE is_active = true;
CREATE INDEX idx_skus_stock ON product_skus(product_id, stock_qty) WHERE is_active = true;

-- Computed view: available stock
CREATE VIEW sku_available_stock AS
    SELECT id, product_id, sku_code, stock_qty - reserved_qty AS available_qty
    FROM product_skus WHERE is_active = true;

CREATE TRIGGER set_skus_updated_at
    BEFORE UPDATE ON product_skus
    FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at();

-- =============================================================
-- PRODUCT IMAGES
-- =============================================================
CREATE TABLE product_images (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id      UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    upload_id       UUID,                  -- Reference to upload tracking
    url_original    VARCHAR(1000),
    url_large       VARCHAR(1000),         -- 1200x1200
    url_medium      VARCHAR(1000),         -- 800x800
    url_small       VARCHAR(1000),         -- 400x400
    url_thumbnail   VARCHAR(1000),         -- 150x150
    format          VARCHAR(10),           -- webp, jpg, png
    size_bytes      INTEGER,
    width_px        INTEGER,
    height_px       INTEGER,
    is_primary      BOOLEAN NOT NULL DEFAULT false,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    processing_status VARCHAR(20) DEFAULT 'pending', -- pending, processing, complete, failed
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_images_product ON product_images(product_id, sort_order);
CREATE UNIQUE INDEX idx_images_primary ON product_images(product_id) WHERE is_primary = true;

-- =============================================================
-- WISHLIST
-- =============================================================
CREATE TABLE wishlist_items (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, product_id)
);

CREATE INDEX idx_wishlist_user ON wishlist_items(user_id);

-- =============================================================
-- PROMOTIONS & COUPONS
-- =============================================================
CREATE TABLE promotions (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code               VARCHAR(50) UNIQUE,      -- NULL = automatic promotion
    name               VARCHAR(200) NOT NULL,
    description        TEXT,
    discount_type      discount_type NOT NULL,
    discount_value     DECIMAL(10, 2) NOT NULL,  -- % or fixed amount
    min_order_amount   DECIMAL(12, 2),
    max_uses           INTEGER,                   -- NULL = unlimited
    uses_count         INTEGER NOT NULL DEFAULT 0,
    max_uses_per_user  INTEGER NOT NULL DEFAULT 1,
    applicable_to      VARCHAR(50) DEFAULT 'all', -- 'all', 'category_id', 'product_id'
    applicable_id      UUID,                      -- category or product if applicable_to != 'all'
    starts_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at         TIMESTAMPTZ,
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_promotions_code ON promotions(code) WHERE code IS NOT NULL AND is_active = true;
CREATE INDEX idx_promotions_active ON promotions(is_active, starts_at, expires_at);
