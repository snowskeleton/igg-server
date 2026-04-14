package model

import "time"

// ── API request/response types ──

// Auth
type AuthRequestBody struct {
	Email string `json:"email"`
}

type AuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type RefreshRequestBody struct {
	RefreshToken string `json:"refresh_token"`
}

// Sync
type SyncRequest struct {
	DeviceID     string      `json:"device_id"`
	LastSyncedAt *time.Time  `json:"last_synced_at"`
	Changes      SyncChanges `json:"changes"`
}

type SyncChanges struct {
	Cars              []Car              `json:"cars"`
	Services          []Service          `json:"services"`
	ScheduledServices []ScheduledService `json:"scheduled_services"`
	CarSettings       []CarSettings      `json:"car_settings"`
}

type SyncResponse struct {
	SyncedAt time.Time   `json:"synced_at"`
	Changes  SyncChanges `json:"changes"`
	Shares   SyncShares  `json:"shares"`
}

type SyncShares struct {
	Owned    []OwnedShare    `json:"owned"`
	Received []ReceivedShare `json:"received"`
}

type OwnedShare struct {
	CarID      string        `json:"car_id"`
	SharedWith []SharePerson `json:"shared_with"`
}

type ReceivedShare struct {
	CarID      string `json:"car_id"`
	OwnerEmail string `json:"owner_email"`
	Status     string `json:"status"`
}

type SharePerson struct {
	Email  string `json:"email"`
	Status string `json:"status"`
}

// Sharing
type CreateShareRequest struct {
	Email string `json:"email"`
}

type ShareResponse struct {
	ID           string    `json:"id"`
	CarID        string    `json:"car_id"`
	InvitedEmail string    `json:"invited_email"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// User
type MeResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// Health
type HealthResponse struct {
	Status string `json:"status"`
}

// Error
type ErrorResponse struct {
	Error string `json:"error"`
}
