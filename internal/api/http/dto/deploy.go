package dto

import "time"

// DeployIntentResponse is the wire shape returned to agents on /deploy/next.
// It is a subset of service.DeployIntent — internal fields like InstallationID
// and Status are deliberately omitted.
type DeployIntentResponse struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	DeliveryID   string    `json:"delivery_id"`
	Repo         string    `json:"repo"`
	Ref          string    `json:"ref"`
	SHA          string    `json:"sha"`
	Image        string    `json:"image"`
	Tag          string    `json:"tag"`
	Stack        string    `json:"stack"`
	Services     []string  `json:"services,omitempty"`
	Environment  string    `json:"environment"`
	DeploymentID int64     `json:"deployment_id"`
}

// DeployResultRequest is what an agent POSTs to /deploy/{id}/result.
type DeployResultRequest struct {
	State string `json:"state"` // "success" or "failure"
	Error string `json:"error,omitempty"`
}
