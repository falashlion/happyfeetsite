-- otps.code stores a SHA-256 hex digest (64 characters), never the 6-digit code
-- itself. The column was declared VARCHAR(10) — the width of the plaintext code
-- — so every insert failed with "value too long for type character varying(10)".
--
-- That made the whole OTP machinery dead on arrival: password reset and email
-- verification both generate a code and then 500 trying to store it. It went
-- unnoticed because nothing delivered the codes, so nobody had reason to call
-- these endpoints until now.
ALTER TABLE otps ALTER COLUMN code TYPE VARCHAR(64);
