-- A product can sit in more than one category (a loafer that is also in
-- "Women's Shoes"). products.category_id stays as the primary category — it
-- names the breadcrumb and is NOT NULL, so nothing that reads it breaks — and
-- this table holds every category the product appears under, primary included.
CREATE TABLE IF NOT EXISTS product_categories (
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_product_categories_category ON product_categories(category_id);

INSERT INTO product_categories (product_id, category_id)
SELECT id, category_id FROM products
ON CONFLICT DO NOTHING;
