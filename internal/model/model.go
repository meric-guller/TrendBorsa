package model

import "time"

type Drop struct {
	ID             string    `json:"id"`
	ProductName    string    `json:"product_name"`
	ProductImage   string    `json:"product_image"`
	Description    string    `json:"description"`
	StartPrice     float64   `json:"start_price"`
	CurrentPrice   float64   `json:"current_price"`
	FloorPrice     float64   `json:"floor_price"`
	CeilingPrice   float64   `json:"ceiling_price"`
	TotalStock     int       `json:"total_stock"`
	RemainingStock int       `json:"remaining_stock"`
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	Status         string    `json:"status"`
	Volatility     float64   `json:"volatility"`
	CreatedAt      time.Time `json:"created_at"`
}

type Purchase struct {
	ID        string    `json:"id"`
	DropID    string    `json:"drop_id"`
	UserID    string    `json:"user_id"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
}

type PricePoint struct {
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

type WSEvent struct {
	Type string `json:"type"`
}

type PriceUpdateEvent struct {
	Type      string  `json:"type"`
	DropID    string  `json:"drop_id"`
	Price     float64 `json:"price"`
	PrevPrice float64 `json:"prev_price"`
	Direction string  `json:"direction"`
}

type StockUpdateEvent struct {
	Type         string  `json:"type"`
	DropID       string  `json:"drop_id"`
	RemainingPct float64 `json:"remaining_pct"`
}

type PurchaseFeedEvent struct {
	Type    string  `json:"type"`
	DropID  string  `json:"drop_id"`
	Price   float64 `json:"price"`
	Message string  `json:"message"`
}

type ViewerCountEvent struct {
	Type   string `json:"type"`
	DropID string `json:"drop_id"`
	Count  int    `json:"count"`
}

type DropStatusEvent struct {
	Type   string `json:"type"`
	DropID string `json:"drop_id"`
	Status string `json:"status"`
}

type DropEndedEvent struct {
	Type       string  `json:"type"`
	DropID     string  `json:"drop_id"`
	FinalPrice float64 `json:"final_price"`
	TotalSold  int     `json:"total_sold"`
}

type PriceLock struct {
	ID          string    `json:"id"`
	DropID      string    `json:"drop_id"`
	UserID      string    `json:"user_id"`
	LockedPrice float64   `json:"locked_price"`
	Quantity    int       `json:"quantity"`
	Status      string    `json:"status"` // pending, confirmed, expired, cancelled
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type PriceLockEvent struct {
	Type     string `json:"type"`
	DropID   string `json:"drop_id"`
	Quantity int    `json:"quantity"`
}

type LockExpiredEvent struct {
	Type     string `json:"type"`
	DropID   string `json:"drop_id"`
	Quantity int    `json:"quantity"`
}
