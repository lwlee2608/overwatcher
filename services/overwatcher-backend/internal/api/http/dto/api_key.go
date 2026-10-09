package dto

import "time"

type APIKeyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type APIKeyListResponse struct {
	Keys []APIKeyResponse `json:"keys"`
}

type CreateAPIKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateAPIKeyResponse carries the raw key. It is returned only here, once.
type CreateAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}
