-- The demo catalogue from seed_products.sql (Air Max 270, Penny Loafer, …) was
-- loaded into production to fill an empty shop. Real products exist now, and
-- the demo ones sit beside them as if they were stock.
--
-- Soft-deleted, not removed: an order or basket may reference these rows, and
-- an old order must still be able to say what was bought. deleted_at hides
-- them from every storefront and console query. The fixed UUIDs match only the
-- seed rows, so nothing created in the console is touched.
UPDATE products
SET status = 'archived', deleted_at = NOW(), updated_at = NOW()
WHERE id IN (
    '11111111-1111-1111-1111-000000000001',
    '11111111-1111-1111-1111-000000000002',
    '11111111-1111-1111-1111-000000000003',
    '11111111-1111-1111-1111-000000000004',
    '11111111-1111-1111-1111-000000000005',
    '11111111-1111-1111-1111-000000000006',
    '11111111-1111-1111-1111-000000000007',
    '11111111-1111-1111-1111-000000000008'
)
AND deleted_at IS NULL;

UPDATE product_skus SET is_active = false
WHERE product_id IN (
    '11111111-1111-1111-1111-000000000001',
    '11111111-1111-1111-1111-000000000002',
    '11111111-1111-1111-1111-000000000003',
    '11111111-1111-1111-1111-000000000004',
    '11111111-1111-1111-1111-000000000005',
    '11111111-1111-1111-1111-000000000006',
    '11111111-1111-1111-1111-000000000007',
    '11111111-1111-1111-1111-000000000008'
);
