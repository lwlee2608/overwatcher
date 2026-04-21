package dto

import "time"

// ServiceSpecDTO is the per-service snapshot carried on an intent.
// An empty Name means "apply to every service in the compose file".
type ServiceSpecDTO struct {
	Name  string `json:"name"`
	Image string `json:"image" binding:"required"`
	Tag   string `json:"tag"`
}

// DeployIntentResponse is the wire shape returned to agents on /deploy/next.
// It is a subset of intent.DeployIntent — internal fields like InstallationID
// and Status are deliberately omitted. ComposeFile is the absolute path on
// the agent VM that was recorded on the project at enqueue time.
type DeployIntentResponse struct {
	ID           string           `json:"id"`
	CreatedAt    time.Time        `json:"created_at"`
	DeliveryID   string           `json:"delivery_id"`
	ProjectID    string           `json:"project_id"`
	ComposeFile  string           `json:"compose_file"`
	Repo         string           `json:"repo"`
	Ref          string           `json:"ref"`
	SHA          string           `json:"sha"`
	Stack        string           `json:"stack"`
	Services     []ServiceSpecDTO `json:"services"`
	Environment  string           `json:"environment"`
	DeploymentID int64            `json:"deployment_id"`
}

// DeployResultRequest is what an agent POSTs to /deploy/{id}/result.
type DeployResultRequest struct {
	State string `json:"state"` // "success" or "failure"
	Error string `json:"error,omitempty"`
}

// DeploymentResponse is the wire shape for the deployments dashboard.
type DeploymentResponse struct {
	ID          string           `json:"id"`
	CreatedAt   time.Time        `json:"created_at"`
	DeliveryID  string           `json:"delivery_id"`
	ProjectID   string           `json:"project_id,omitempty"`
	Repo        string           `json:"repo"`
	Ref         string           `json:"ref"`
	SHA         string           `json:"sha"`
	Stack       string           `json:"stack"`
	Services    []ServiceSpecDTO `json:"services"`
	Environment string           `json:"environment"`
	Status      string           `json:"status"`
	Attempts    int              `json:"attempts"`
}

type DeploymentListResponse struct {
	Deployments []DeploymentResponse `json:"deployments"`
}
