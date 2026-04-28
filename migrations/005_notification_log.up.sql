CREATE TABLE IF NOT EXISTS notification_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    device_token_prefix TEXT NOT NULL,
    push_type TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    error_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notification_log_user ON notification_log (user_id);
CREATE INDEX IF NOT EXISTS idx_notification_log_created ON notification_log (created_at);
