package domain

import "time"

const (
	PreviewPending           = "pending"
	PreviewReady             = "ready"
	PreviewNeedsConfirmation = "needs_confirmation"
	PreviewFailed            = "failed"

	TrackerActive         = "active"
	TrackerPaused         = "paused"
	TrackerNeedsAttention = "needs_attention"

	AvailabilityInStock    = "in_stock"
	AvailabilityOutOfStock = "out_of_stock"
	AvailabilityPreorder   = "preorder"
	AvailabilityUnknown    = "unknown"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Country   string    `json:"country"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

type MagicLink struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ProductPreview struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	URL          string     `json:"url"`
	Status       string     `json:"status"`
	Name         string     `json:"name"`
	ImageURL     string     `json:"image_url"`
	CurrentPrice float64    `json:"current_price"`
	Currency     string     `json:"currency"`
	Availability string     `json:"availability"`
	Country      string     `json:"country"`
	Confidence   float64    `json:"confidence"`
	Error        string     `json:"error,omitempty"`
	ConfirmedAt  *time.Time `json:"confirmed_at,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type AlertRule struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	ThresholdPrice *float64 `json:"threshold_price,omitempty"`
	Enabled        bool     `json:"enabled"`
}

type Tracker struct {
	ID            string      `json:"id"`
	UserID        string      `json:"user_id"`
	PreviewID     string      `json:"preview_id"`
	ProductURL    string      `json:"product_url"`
	Name          string      `json:"name"`
	ImageURL      string      `json:"image_url"`
	Currency      string      `json:"currency"`
	Country       string      `json:"country"`
	IntervalHours int         `json:"interval_hours"`
	Status        string      `json:"status"`
	CurrentPrice  float64     `json:"current_price"`
	Availability  string      `json:"availability"`
	LastCheckedAt *time.Time  `json:"last_checked_at,omitempty"`
	NextCheckAt   time.Time   `json:"next_check_at"`
	Rules         []AlertRule `json:"rules"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type Observation struct {
	ID           string    `json:"id"`
	TrackerID    string    `json:"tracker_id"`
	Price        float64   `json:"price"`
	Currency     string    `json:"currency"`
	Availability string    `json:"availability"`
	Method       string    `json:"method"`
	Confidence   float64   `json:"confidence"`
	ObservedAt   time.Time `json:"observed_at"`
}

type ScrapeResult struct {
	Name          string
	CanonicalURL  string
	ImageURL      string
	CurrentPrice  float64
	OriginalPrice *float64
	Currency      string
	Availability  string
	Country       string
	Method        string
	Confidence    float64
	RawRef        string
	ObservedAt    time.Time
}
