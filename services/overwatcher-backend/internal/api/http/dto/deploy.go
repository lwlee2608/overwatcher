package dto

import (
	"time"

	"github.com/lwlee2608/overwatcher/internal/protocol"
)

// DeploymentResponse is the wire shape for the deployments dashboard.
type DeploymentResponse struct {
	ID          string                    `json:"id"`
	CreatedAt   time.Time                 `json:"created_at"`
	DeliveryID  string                    `json:"delivery_id"`
	ProjectID   string                    `json:"project_id,omitempty"`
	Repo        string                    `json:"repo"`
	Ref         string                    `json:"ref"`
	SHA         string                    `json:"sha"`
	Stack       string                    `json:"stack"`
	Services    []protocol.ServiceSpecDTO `json:"services"`
	Environment string                    `json:"environment"`
	Status      string                    `json:"status"`
	Attempts    int                       `json:"attempts"`
}

type DeploymentListResponse struct {
	Deployments []DeploymentResponse `json:"deployments"`
	Total       int64                `json:"total"`
	Page        int                  `json:"page"`
	PageSize    int                  `json:"page_size"`
}
