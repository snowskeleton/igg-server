ALTER TABLE magic_tokens ADD COLUMN IF NOT EXISTS poll_token TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_magic_tokens_poll_token ON magic_tokens (poll_token) WHERE poll_token IS NOT NULL;
