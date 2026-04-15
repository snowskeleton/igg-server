package model

import "time"

// ── Database models ──

type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type MagicToken struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	PollToken *string   `json:"poll_token" db:"poll_token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	Used      bool      `json:"used" db:"used"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type RefreshToken struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	TokenHash string    `json:"token_hash" db:"token_hash"`
	Revoked   bool      `json:"revoked" db:"revoked"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Car struct {
	ID               string    `json:"id" db:"id"`
	OwnerID          string    `json:"owner_id" db:"owner_id"`
	Make             string    `json:"make" db:"make"`
	Model            string    `json:"model" db:"model"`
	Name             string    `json:"name" db:"name"`
	Plate            string    `json:"plate" db:"plate"`
	VIN              string    `json:"vin" db:"vin"`
	Year             *int      `json:"year" db:"year"`
	StartingOdometer int       `json:"starting_odometer" db:"starting_odometer"`
	Pinned           bool      `json:"pinned" db:"pinned"`
	Deleted          bool      `json:"deleted" db:"deleted"`
	Archived         bool      `json:"archived" db:"archived"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

type Service struct {
	ID              string    `json:"id" db:"id"`
	CarID           string    `json:"car_id" db:"car_id"`
	Cost            float64   `json:"cost" db:"cost"`
	Date            time.Time `json:"date" db:"date"`
	Pending         bool      `json:"pending" db:"pending"`
	Name            string    `json:"name" db:"name"`
	FullDescription string    `json:"full_description" db:"full_description"`
	Odometer        int       `json:"odometer" db:"odometer"`
	IsFuel          bool      `json:"is_fuel" db:"is_fuel"`
	IsFullTank      bool      `json:"is_full_tank" db:"is_full_tank"`
	Gallons         float64   `json:"gallons" db:"gallons"`
	VendorName       string    `json:"vendor_name" db:"vendor_name"`
	AnomalyDismissed bool      `json:"anomaly_dismissed" db:"anomaly_dismissed"`
	Deleted          bool      `json:"deleted" db:"deleted"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type ScheduledService struct {
	ID                    string    `json:"id" db:"id"`
	CarID                 string    `json:"car_id" db:"car_id"`
	Name                  string    `json:"name" db:"name"`
	FullDescription       string    `json:"full_description" db:"full_description"`
	NotificationUUID      string    `json:"notification_uuid" db:"notification_uuid"`
	Repeating             bool      `json:"repeating" db:"repeating"`
	OdometerFirstOccurance int      `json:"odometer_first_occurance" db:"odometer_first_occurance"`
	FrequencyMiles        int       `json:"frequency_miles" db:"frequency_miles"`
	FrequencyTime         int       `json:"frequency_time" db:"frequency_time"`
	FrequencyTimeInterval string    `json:"frequency_time_interval" db:"frequency_time_interval"`
	FrequencyTimeStart    time.Time `json:"frequency_time_start" db:"frequency_time_start"`
	Deleted               bool      `json:"deleted" db:"deleted"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

type CarSettings struct {
	ID                 string    `json:"id" db:"id"`
	CarID              string    `json:"car_id" db:"car_id"`
	UserID             string    `json:"user_id" db:"user_id"`
	SelectedTab        string    `json:"selected_tab" db:"selected_tab"`
	RangeDays          int       `json:"range_days" db:"range_days"`
	IncludeFuel        bool      `json:"include_fuel" db:"include_fuel"`
	IncludeMaintenance bool      `json:"include_maintenance" db:"include_maintenance"`
	IncludeCompleted   bool      `json:"include_completed" db:"include_completed"`
	IncludePending     bool      `json:"include_pending" db:"include_pending"`
	Custom             bool      `json:"custom" db:"custom"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

type CarShare struct {
	ID           string    `json:"id" db:"id"`
	CarID        string    `json:"car_id" db:"car_id"`
	SharedByID   string    `json:"shared_by_id" db:"shared_by_id"`
	SharedWithID *string   `json:"shared_with_id" db:"shared_with_id"`
	InvitedEmail string    `json:"invited_email" db:"invited_email"`
	Status       string    `json:"status" db:"status"`
	Token        string    `json:"token" db:"token"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type UserCarPrefs struct {
	UserID   string `json:"user_id" db:"user_id"`
	CarID    string `json:"car_id" db:"car_id"`
	Pinned   bool   `json:"pinned" db:"pinned"`
	Archived bool   `json:"archived" db:"archived"`
}

type SyncCursor struct {
	UserID   string    `json:"user_id" db:"user_id"`
	DeviceID string    `json:"device_id" db:"device_id"`
	CursorAt time.Time `json:"cursor_at" db:"cursor_at"`
}
