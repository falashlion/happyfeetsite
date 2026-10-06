-- Narrowing back truncates stored digests, which invalidates any outstanding
-- code. Expire them rather than leave rows that can never match.
UPDATE otps SET used_at = NOW() WHERE used_at IS NULL;
ALTER TABLE otps ALTER COLUMN code TYPE VARCHAR(10);
