package intent

import (
	"errors"
	"time"
)

// ErrNotFound is returned by DBStore.GetByID when no intent exists with the
// given id.
var ErrNotFound = errors.New("intent not found")

// Status tracks a DeployIntent through its lifecycle.
type Status string

const (
	StatusCreated           Status = "created"
	StatusDispatched        Status = "dispatched"
	StatusSucceeded         Status = "succeeded"
	StatusFailed            Status = "failed"
	StatusPermanentlyFailed Status = "permanently_failed"
)

// ServiceSpec is the snapshot of a compose service embedded in an intent.
// An empty Name means "apply to every service in the compose file".
type ServiceSpec struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

// DeployIntent is everything an agent needs to perform a deploy. It is produced
// on a push webhook when the repo matches services within an enabled project.
type DeployIntent struct {
	ID                 string
	CreatedAt          time.Time
	DeliveryID         string
	ProjectID          string
	ProjectName        string
	ComposeFile        string
	ComposeProjectName string
	Repo               string
	Ref                string
	SHA                string
	Stack              string
	Services           []ServiceSpec
	Environment        string
	DeploymentID       int64
	InstallationID     int64
	Status             Status
	Attempts           int
	DispatchedAt       time.Time
}
