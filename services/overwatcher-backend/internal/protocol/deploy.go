// Package protocol defines the wire contract between the coordinator and
// the agent. Anything an agent sees or sends lives here. Coordinator-only
// shapes (e.g. dashboard responses) stay in internal/api/http/dto.
//
//	  coordinator                                  agent
//	  ┌─────────────┐                          ┌─────────────┐
//	  │             │  GET /deploy/next        │             │
//	  │             │ ───────────────────────► │             │
//	  │             │  DeployIntentResponse    │             │
//	  │             │ ◄─────────────────────── │             │
//	  │             │                          │             │
//	  │             │  POST /deploy/{id}/result│             │
//	  │             │ ◄─────────────────────── │             │
//	  │             │  DeployResultRequest     │             │
//	  └─────────────┘                          └─────────────┘
//
// Coordinator and agent build from this package in the same Go module, so
// a field change breaks both compiles until they agree.
package protocol

import "time"

// ServiceSpecDTO is the per-service snapshot on an intent. Empty Name
// means "apply to every service in the compose file".
type ServiceSpecDTO struct {
	Name  string `json:"name"`
	Image string `json:"image" binding:"required"`
	Tag   string `json:"tag"`
}

// DeployIntentResponse is returned to agents on /deploy/next — a subset of
// intent.DeployIntent (no InstallationID, Status, etc.). ComposeFile is the
// absolute path on the agent VM, recorded on the project at enqueue time.
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
