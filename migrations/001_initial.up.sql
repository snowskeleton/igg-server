CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- magic_tokens
CREATE TABLE magic_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- refresh_tokens
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    revoked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- cars
CREATE TABLE cars (
    id TEXT PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    make TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    plate TEXT NOT NULL DEFAULT '',
    vin TEXT NOT NULL DEFAULT '',
    year INT,
    starting_odometer INT NOT NULL DEFAULT 0,
    pinned BOOLEAN NOT NULL DEFAULT false,
    deleted BOOLEAN NOT NULL DEFAULT false,
    archived BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- services
CREATE TABLE services (
    id TEXT PRIMARY KEY,
    car_id TEXT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    cost DOUBLE PRECISION NOT NULL DEFAULT 0,
    date TIMESTAMPTZ NOT NULL DEFAULT now(),
    pending BOOLEAN NOT NULL DEFAULT false,
    name TEXT NOT NULL DEFAULT '',
    full_description TEXT NOT NULL DEFAULT '',
    odometer INT NOT NULL DEFAULT 0,
    is_fuel BOOLEAN NOT NULL DEFAULT false,
    is_full_tank BOOLEAN NOT NULL DEFAULT true,
    gallons DOUBLE PRECISION NOT NULL DEFAULT 0,
    vendor_name TEXT NOT NULL DEFAULT '',
    deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- scheduled_services
CREATE TABLE scheduled_services (
    id TEXT PRIMARY KEY,
    car_id TEXT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    full_description TEXT NOT NULL DEFAULT '',
    notification_uuid TEXT NOT NULL DEFAULT '',
    repeating BOOLEAN NOT NULL DEFAULT false,
    odometer_first_occurance INT NOT NULL DEFAULT 0,
    frequency_miles INT NOT NULL DEFAULT 0,
    frequency_time INT NOT NULL DEFAULT 0,
    frequency_time_interval TEXT NOT NULL DEFAULT 'month',
    frequency_time_start TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- car_settings (per user per car)
CREATE TABLE car_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    car_id TEXT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    selected_tab TEXT NOT NULL DEFAULT 'MPG',
    range_days INT NOT NULL DEFAULT 90,
    include_fuel BOOLEAN NOT NULL DEFAULT true,
    include_maintenance BOOLEAN NOT NULL DEFAULT true,
    include_completed BOOLEAN NOT NULL DEFAULT true,
    include_pending BOOLEAN NOT NULL DEFAULT false,
    custom BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(car_id, user_id)
);

-- car_shares
CREATE TABLE car_shares (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    car_id TEXT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    shared_by_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shared_with_id UUID REFERENCES users(id) ON DELETE SET NULL,
    invited_email TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'revoked')),
    token TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(car_id, invited_email)
);

-- user_car_prefs (per-user pinned/archived for shared cars)
CREATE TABLE user_car_prefs (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    car_id TEXT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
    pinned BOOLEAN NOT NULL DEFAULT false,
    archived BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, car_id)
);

-- sync_cursors
CREATE TABLE sync_cursors (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    cursor_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, device_id)
);

-- indexes
CREATE INDEX idx_cars_owner ON cars(owner_id);
CREATE INDEX idx_services_car ON services(car_id);
CREATE INDEX idx_services_updated ON services(updated_at);
CREATE INDEX idx_scheduled_services_car ON scheduled_services(car_id);
CREATE INDEX idx_scheduled_services_updated ON scheduled_services(updated_at);
CREATE INDEX idx_car_settings_car_user ON car_settings(car_id, user_id);
CREATE INDEX idx_car_shares_car ON car_shares(car_id);
CREATE INDEX idx_car_shares_invited_email ON car_shares(invited_email);
CREATE INDEX idx_car_shares_shared_with ON car_shares(shared_with_id);
CREATE INDEX idx_cars_updated ON cars(updated_at);
CREATE INDEX idx_magic_tokens_token ON magic_tokens(token);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
