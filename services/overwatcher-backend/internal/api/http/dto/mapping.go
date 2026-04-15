package dto

import "time"

type DeployMappingResponse struct {
	ID          string    `json:"id"`
	Repo        string    `json:"repo"`
	AgentID     string    `json:"agent_id"`
	AgentName   string    `json:"agent_name"`
	Services    []string  `json:"services"`
	Environment string    `json:"environment"`
	Image       string    `json:"image"`
	Tag         string    `json:"tag"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type DeployMappingListResponse struct {
	Mappings []DeployMappingResponse `json:"mappings"`
}

type CreateDeployMappingRequest struct {
	Repo        string   `json:"repo" binding:"required"`
	AgentID     string   `json:"agent_id" binding:"required"`
	Services    []string `json:"services"`
	Environment string   `json:"environment"`
	Image       string   `json:"image" binding:"required"`
	Tag         string   `json:"tag"`
	Enabled     *bool    `json:"enabled"`
}

type UpdateDeployMappingRequest struct {
	Repo        string   `json:"repo" binding:"required"`
	AgentID     string   `json:"agent_id" binding:"required"`
	Services    []string `json:"services"`
	Environment string   `json:"environment"`
	Image       string   `json:"image" binding:"required"`
	Tag         string   `json:"tag"`
	Enabled     *bool    `json:"enabled"`
}
