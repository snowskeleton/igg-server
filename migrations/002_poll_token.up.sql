ALTER TABLE magic_tokens ADD COLUMN poll_token TEXT;
CREATE UNIQUE INDEX idx_magic_tokens_poll_token ON magic_tokens (poll_token) WHERE poll_token IS NOT NULL;
