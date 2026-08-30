-- =============================================================
-- HappyFeet — Dev seed (vendor + products + SKUs + images)
-- Idempotent: safe to re-run; uses fixed UUIDs for products & vendor.
-- =============================================================

BEGIN;

-- ── Vendor owner (admin user) ────────────────────────────────────────────────
INSERT INTO users (id, email, password_hash, first_name, last_name, role, status, email_verified_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'vendor@happyfeet.dev',
    -- Argon2id hash of "Passw0rd!" (placeholder; vendor isn't expected to log in via API)
    '$argon2id$v=19$m=65536,t=3,p=4$YWFhYWFhYWFhYWFhYWFhYQ$+TfL5pYdvyN/8j9aE3X3lOQNZJZx2cQ7gT2bGv/2qVE',
    'Happy', 'Atelier', 'admin', 'active', NOW()
) ON CONFLICT (id) DO NOTHING;

-- ── Vendor ───────────────────────────────────────────────────────────────────
INSERT INTO vendors (id, owner_id, business_name, business_type, status, approved_at)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000001',
    'Happy Feet Atelier', 'individual', 'active', NOW()
) ON CONFLICT (id) DO NOTHING;

-- ── Helper: resolve category and brand IDs by slug/name ──────────────────────
WITH
  cat AS (
    SELECT slug, id FROM categories
  ),
  br AS (
    SELECT name, id FROM brands
  )

-- ── Products (fixed UUIDs so reruns are idempotent) ──────────────────────────
INSERT INTO products (
    id, vendor_id, category_id, brand_id,
    name, slug, description, base_price, currency,
    status, tags
)
SELECT v.id, '00000000-0000-0000-0000-000000000010', c.id, b.id,
       v.name, v.slug, v.description, v.base_price, 'XAF'::currency_code,
       'active'::product_status, v.tags
FROM (VALUES
    ('11111111-1111-1111-1111-000000000001'::uuid,
     'Air Max 270', 'air-max-270-onyx',
     'A cushioned everyday silhouette built on a visible Max Air heel. Mesh and synthetic upper, foam midsole, rubber outsole tuned for city wear.',
     45000.00,
     ARRAY['Lifestyle cushioned runner','Mesh upper','Foam midsole','Rubber outsole']::text[],
     'sneakers', 'Nike'),
    ('11111111-1111-1111-1111-000000000002'::uuid,
     'Penny Loafer', 'clarks-penny-tan',
     'Blake-stitched penny loafer in full-grain leather. Leather lining, leather sole with a thin rubber heel cap. Built to walk in for a season, then yours for a decade.',
     62000.00,
     ARRAY['Hand-stitched tan leather','Full-grain leather','Blake stitched','Leather lined']::text[],
     'loafers', 'Clarks'),
    ('11111111-1111-1111-1111-000000000003'::uuid,
     '1460 Smooth Boot', 'dm-1460-smooth-black',
     'The original eight-eye boot. Smooth leather, signature yellow welt stitch, AirWair sole. Built to break in, then never quit.',
     38250.00,
     ARRAY['Eight-eye, Goodyear-welted','Smooth leather','Goodyear welted','AirWair sole','sale']::text[],
     'boots', 'Dr. Martens'),
    ('11111111-1111-1111-1111-000000000004'::uuid,
     'Samba OG', 'adidas-samba-og',
     'Low-profile leather and gum-rubber sole. A pitch shoe that took a thirty-year detour through the city.',
     52000.00,
     ARRAY['Indoor terrace classic','Leather upper','Suede toe','Gum-rubber sole']::text[],
     'sneakers', 'Adidas'),
    ('11111111-1111-1111-1111-000000000005'::uuid,
     'Lisbon Derby', 'ecco-derby-cocoa',
     'A clean three-eyelet derby for the office or the wedding. Padded collar, leather-lined footbed, a sole that holds a polish.',
     78000.00,
     ARRAY['Cocoa calfskin','Calfskin','Leather lining','Stitched-down sole']::text[],
     'formal-dress', 'ECCO'),
    ('11111111-1111-1111-1111-000000000006'::uuid,
     'Arizona Sandal', 'birk-arizona-cream',
     'Adjustable two-strap profile on the legendary cork-and-latex footbed. Shapes to your foot within a fortnight.',
     36000.00,
     ARRAY['Two-strap cork footbed','Birko-Flor','Cork-latex footbed','EVA outsole']::text[],
     'sandals', 'Birkenstock'),
    ('11111111-1111-1111-1111-000000000007'::uuid,
     '993 Made in USA', 'nb-993-grey',
     'The dad-shoe that earned the title. Pigskin and mesh, ABZORB cushioning, hand-built in Skowhegan, Maine.',
     92000.00,
     ARRAY['ABZORB-cushioned heritage runner','Pigskin & mesh','ABZORB midsole']::text[],
     'sneakers', 'New Balance'),
    ('11111111-1111-1111-1111-000000000008'::uuid,
     'Old Skool Mono', 'vans-old-skool-mono',
     'The first Vans to wear the jazz stripe. Canvas-and-suede upper, padded collar, waffle outsole.',
     32000.00,
     ARRAY['Canvas-suede skate classic','Canvas & suede','Waffle rubber outsole']::text[],
     'sneakers', 'Vans')
) AS v(id, name, slug, description, base_price, tags, cat_slug, brand_name)
JOIN cat c ON c.slug = v.cat_slug
JOIN br b ON b.name = v.brand_name
ON CONFLICT (id) DO UPDATE
   SET name = EXCLUDED.name,
       description = EXCLUDED.description,
       base_price = EXCLUDED.base_price,
       status = EXCLUDED.status,
       tags = EXCLUDED.tags;

-- ── Clear any previous SKUs and images for these products (idempotent reseed) ─
DELETE FROM product_skus WHERE product_id IN (
    '11111111-1111-1111-1111-000000000001',
    '11111111-1111-1111-1111-000000000002',
    '11111111-1111-1111-1111-000000000003',
    '11111111-1111-1111-1111-000000000004',
    '11111111-1111-1111-1111-000000000005',
    '11111111-1111-1111-1111-000000000006',
    '11111111-1111-1111-1111-000000000007',
    '11111111-1111-1111-1111-000000000008'
);
DELETE FROM product_images WHERE product_id IN (
    '11111111-1111-1111-1111-000000000001',
    '11111111-1111-1111-1111-000000000002',
    '11111111-1111-1111-1111-000000000003',
    '11111111-1111-1111-1111-000000000004',
    '11111111-1111-1111-1111-000000000005',
    '11111111-1111-1111-1111-000000000006',
    '11111111-1111-1111-1111-000000000007',
    '11111111-1111-1111-1111-000000000008'
);

-- ── SKUs ─────────────────────────────────────────────────────────────────────
-- Air Max 270 — sizes 39..45, colours Onyx / Cream / Navy (44 OOS)
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000001', 'AM270-' || c.name || '-' || s, s, c.name, c.hex,
       CASE WHEN s = 44 THEN 0 ELSE 10 END
FROM unnest(ARRAY[39,40,41,42,43,44,45]) s
CROSS JOIN (VALUES ('Onyx','#15161B'),('Cream','#F5EFE0'),('Navy','#0E1B3A')) AS c(name, hex);

-- Penny Loafer — sizes 40..45, Tan / Oxblood / Black
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000002', 'PENNY-' || c.name || '-' || s, s, c.name, c.hex, 8
FROM unnest(ARRAY[40,41,42,43,44,45]) s
CROSS JOIN (VALUES ('Tan','#8B5A2B'),('Oxblood','#5A2A2A'),('Black','#15161B')) AS c(name, hex);

-- DM 1460 — sizes 39..44, Black / Cherry (40 OOS)
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000003', 'DM1460-' || c.name || '-' || s, s, c.name, c.hex,
       CASE WHEN s = 40 THEN 0 ELSE 12 END
FROM unnest(ARRAY[39,40,41,42,43,44]) s
CROSS JOIN (VALUES ('Black','#15161B'),('Cherry','#7A1F1F')) AS c(name, hex);

-- Samba OG — sizes 39..45, Cream / Black
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000004', 'SAMBA-' || c.name || '-' || s, s, c.name, c.hex, 9
FROM unnest(ARRAY[39,40,41,42,43,44,45]) s
CROSS JOIN (VALUES ('Cream','#F5EFE0'),('Black','#15161B')) AS c(name, hex);

-- Lisbon Derby — 40..45, Cocoa / Black (45 OOS)
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000005', 'LISBON-' || c.name || '-' || s, s, c.name, c.hex,
       CASE WHEN s = 45 THEN 0 ELSE 6 END
FROM unnest(ARRAY[40,41,42,43,44,45]) s
CROSS JOIN (VALUES ('Cocoa','#5C3A21'),('Black','#15161B')) AS c(name, hex);

-- Arizona — 38..43, Cream / Mocha / Black
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000006', 'BIRK-' || c.name || '-' || s, s, c.name, c.hex, 14
FROM unnest(ARRAY[38,39,40,41,42,43]) s
CROSS JOIN (VALUES ('Cream','#F5EFE0'),('Mocha','#7A5A3D'),('Black','#15161B')) AS c(name, hex);

-- NB 993 — 40..45, Grey / Navy (42 OOS)
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000007', 'NB993-' || c.name || '-' || s, s, c.name, c.hex,
       CASE WHEN s = 42 THEN 0 ELSE 5 END
FROM unnest(ARRAY[40,41,42,43,44,45]) s
CROSS JOIN (VALUES ('Grey','#8C8E96'),('Navy','#0E1B3A')) AS c(name, hex);

-- Old Skool Mono — 38..44, Black / Cream
INSERT INTO product_skus (product_id, sku_code, size_eu, color, color_hex, stock_qty)
SELECT '11111111-1111-1111-1111-000000000008', 'VANS-' || c.name || '-' || s, s, c.name, c.hex, 15
FROM unnest(ARRAY[38,39,40,41,42,43,44]) s
CROSS JOIN (VALUES ('Black','#15161B'),('Cream','#F5EFE0')) AS c(name, hex);

-- ── Images (primary + a couple of extras per product) ────────────────────────
INSERT INTO product_images (product_id, url_original, url_large, url_medium, url_small, url_thumbnail, is_primary, sort_order, processing_status)
VALUES
    -- Air Max 270
    ('11111111-1111-1111-1111-000000000001',
     'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=1600&q=80',
     'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=1200&q=80',
     'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=900&q=80',
     'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=400&q=80',
     'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=200&q=80',
     true, 0, 'complete'),
    ('11111111-1111-1111-1111-000000000001',
     'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=1600&q=80',
     'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=1200&q=80',
     'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=900&q=80',
     'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=400&q=80',
     'https://images.unsplash.com/photo-1556906781-9a412961c28c?w=200&q=80',
     false, 1, 'complete'),

    -- Penny Loafer
    ('11111111-1111-1111-1111-000000000002',
     'https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=1600&q=80',
     'https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=1200&q=80',
     'https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=900&q=80',
     'https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=400&q=80',
     'https://images.unsplash.com/photo-1543163521-1bf539c55dd2?w=200&q=80',
     true, 0, 'complete'),

    -- DM 1460
    ('11111111-1111-1111-1111-000000000003',
     'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=1600&q=80',
     'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=1200&q=80',
     'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=900&q=80',
     'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=400&q=80',
     'https://images.unsplash.com/photo-1549298916-b41d501d3772?w=200&q=80',
     true, 0, 'complete'),

    -- Samba OG
    ('11111111-1111-1111-1111-000000000004',
     'https://images.unsplash.com/photo-1606107557195-0e29a4b5b4aa?w=1600&q=80',
     'https://images.unsplash.com/photo-1606107557195-0e29a4b5b4aa?w=1200&q=80',
     'https://images.unsplash.com/photo-1606107557195-0e29a4b5b4aa?w=900&q=80',
     'https://images.unsplash.com/photo-1606107557195-0e29a4b5b4aa?w=400&q=80',
     'https://images.unsplash.com/photo-1606107557195-0e29a4b5b4aa?w=200&q=80',
     true, 0, 'complete'),

    -- Lisbon Derby
    ('11111111-1111-1111-1111-000000000005',
     'https://images.unsplash.com/photo-1614253429340-98120bd6d753?w=1600&q=80',
     'https://images.unsplash.com/photo-1614253429340-98120bd6d753?w=1200&q=80',
     'https://images.unsplash.com/photo-1614253429340-98120bd6d753?w=900&q=80',
     'https://images.unsplash.com/photo-1614253429340-98120bd6d753?w=400&q=80',
     'https://images.unsplash.com/photo-1614253429340-98120bd6d753?w=200&q=80',
     true, 0, 'complete'),

    -- Arizona Sandal
    ('11111111-1111-1111-1111-000000000006',
     'https://images.unsplash.com/photo-1603487742131-4160ec999306?w=1600&q=80',
     'https://images.unsplash.com/photo-1603487742131-4160ec999306?w=1200&q=80',
     'https://images.unsplash.com/photo-1603487742131-4160ec999306?w=900&q=80',
     'https://images.unsplash.com/photo-1603487742131-4160ec999306?w=400&q=80',
     'https://images.unsplash.com/photo-1603487742131-4160ec999306?w=200&q=80',
     true, 0, 'complete'),

    -- NB 993
    ('11111111-1111-1111-1111-000000000007',
     'https://images.unsplash.com/photo-1539185441755-769473a23570?w=1600&q=80',
     'https://images.unsplash.com/photo-1539185441755-769473a23570?w=1200&q=80',
     'https://images.unsplash.com/photo-1539185441755-769473a23570?w=900&q=80',
     'https://images.unsplash.com/photo-1539185441755-769473a23570?w=400&q=80',
     'https://images.unsplash.com/photo-1539185441755-769473a23570?w=200&q=80',
     true, 0, 'complete'),

    -- Old Skool Mono
    ('11111111-1111-1111-1111-000000000008',
     'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=1600&q=80',
     'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=1200&q=80',
     'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=900&q=80',
     'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=400&q=80',
     'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=200&q=80',
     true, 0, 'complete');

COMMIT;
