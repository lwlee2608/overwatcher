package dto

import "time"

type EventLogResponse struct {
	ID         string    `json:"id"`
	DeliveryID string    `json:"delivery_id"`
	EventType  string    `json:"event_type"`
	Repo       string    `json:"repo"`
	Sender     string    `json:"sender"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

type EventLogListResponse struct {
	Events []EventLogResponse `json:"events"`
}
