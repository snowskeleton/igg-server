DROP INDEX IF EXISTS idx_magic_tokens_poll_token;
ALTER TABLE magic_tokens DROP COLUMN IF EXISTS poll_token;
