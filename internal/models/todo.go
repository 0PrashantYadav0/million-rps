package models

import "time"

// Todo represents a todo item.
type Todo struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TodoCommand is the message payload for Kafka (create/update/delete).
type TodoCommand struct {
	Action      string    `json:"action"` // create, update, delete
	ID          string    `json:"id"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Completed   *bool     `json:"completed,omitempty"`
	UserID      string    `json:"user_id"`
	RequestedAt time.Time `json:"requested_at"`
}
