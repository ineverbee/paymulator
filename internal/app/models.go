package app

import "time"

//Transaction defines a structure for an item in transaction list
type Transaction struct {
	ID         int       `json:"id,omitempty"`
	UserID     int       `json:"user_id,omitempty"`
	Email      string    `json:"email,omitempty"`
	Amount     float64   `json:"amount,omitempty"`
	Currency   string    `json:"currency,omitempty"`
	Created_at time.Time `json:"created_at,omitempty"`
	Changed_at time.Time `json:"changed_at,omitempty"`
	Status     string    `json:"transaction_status,omitempty"`
}
