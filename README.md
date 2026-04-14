# igg-server

Sync server for the I Got Gas iOS app. Handles authentication, data sync, and vehicle sharing.

## Quick Start

```bash
# 1. Copy the example env and edit it
cp .env.example .env
```

Edit `.env` with your values:

```
DATABASE_URL=postgres://igg:igg@db:5432/igg?sslmode=disable
JWT_SECRET=<generate-a-random-64-char-string>
BASE_URL=https://api.igg.snowskeleton.net

SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=noreply@example.com
SMTP_PASS=your-smtp-password
SMTP_FROM=noreply@example.com
SMTP_MOCK=true
```

Set `SMTP_MOCK=true` to log emails to stdout instead of sending them (useful for development).

```bash
# 2. Start everything
docker compose up -d

# 3. Check it's running
curl http://localhost:8080/v1/health
# {"status":"ok"}
```

That's it. The server runs migrations automatically on startup.

## Generate a JWT Secret

```bash
openssl rand -base64 48
```

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | Secret key for signing JWTs (min 32 chars) |
| `BASE_URL` | Yes | Public URL of the server (used in email links) |
| `PORT` | No | Server port (default: `8080`) |
| `SMTP_HOST` | No | SMTP server hostname |
| `SMTP_PORT` | No | SMTP server port (default: `587`) |
| `SMTP_USER` | No | SMTP username |
| `SMTP_PASS` | No | SMTP password |
| `SMTP_FROM` | No | From address for emails |
| `SMTP_MOCK` | No | Set `true` to log emails instead of sending (default: `true`) |
| `MIGRATIONS_DIR` | No | Path to migrations folder (default: `migrations`) |

## API Endpoints

### Auth (no auth required)

- `POST /v1/auth/request` — send magic login link
- `GET /v1/auth/verify?token=...` — verify magic link, returns JWT
- `POST /v1/auth/refresh` — rotate access + refresh tokens
- `POST /v1/auth/logout` — revoke refresh token

### Sync (auth required)

- `POST /v1/sync` — bidirectional push+pull sync

### Sharing (auth required)

- `POST /v1/cars/{carId}/shares` — invite someone to a car
- `GET /v1/cars/{carId}/shares` — list shares for a car
- `DELETE /v1/cars/{carId}/shares/{shareId}` — revoke a share
- `GET /v1/shares/pending` — list pending invitations for current user
- `POST /v1/shares/{shareId}/accept` — accept invitation
- `POST /v1/shares/{shareId}/decline` — decline invitation

### User (auth required)

- `GET /v1/me` — current user info
- `DELETE /v1/me` — delete account and all data

### Health (no auth)

- `GET /v1/health` — returns `{"status":"ok"}`

## Reverse Proxy

The server doesn't handle TLS. Put Caddy, nginx, or Traefik in front of it. Example Caddyfile:

```
api.igg.snowskeleton.net {
    reverse_proxy localhost:8080
}
```

## Development

```bash
# Run locally (requires Go 1.23+ and a running PostgreSQL)
export DATABASE_URL=postgres://igg:igg@localhost:5432/igg?sslmode=disable
export JWT_SECRET=dev-secret-change-me-in-production
export SMTP_MOCK=true
go run ./cmd/server

# Build
go build -o igg-server ./cmd/server
```
