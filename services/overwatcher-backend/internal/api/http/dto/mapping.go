package dto

import "time"

type ServiceSpecDTO struct {
	Name  string `json:"name"`
	Image string `json:"image" binding:"required"`
	Tag   string `json:"tag"`
}

type DeployMappingResponse struct {
	ID          string           `json:"id"`
	Repo        string           `json:"repo"`
	AgentID     string           `json:"agent_id"`
	AgentName   string           `json:"agent_name"`
	Services    []ServiceSpecDTO `json:"services"`
	Environment string           `json:"environment"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type DeployMappingListResponse struct {
	Mappings []DeployMappingResponse `json:"mappings"`
}

type CreateDeployMappingRequest struct {
	Repo        string           `json:"repo" binding:"required"`
	AgentID     string           `json:"agent_id" binding:"required"`
	Services    []ServiceSpecDTO `json:"services" binding:"required,min=1,dive"`
	Environment string           `json:"environment"`
	Enabled     *bool            `json:"enabled"`
}

type UpdateDeployMappingRequest struct {
	Repo        string           `json:"repo" binding:"required"`
	AgentID     string           `json:"agent_id" binding:"required"`
	Services    []ServiceSpecDTO `json:"services" binding:"required,min=1,dive"`
	Environment string           `json:"environment"`
	Enabled     *bool            `json:"enabled"`
}
