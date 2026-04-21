package dto

import "time"

type ProjectResponse struct {
	ID          string                   `json:"id"`
	UserID      string                   `json:"user_id"`
	UserEmail   string                   `json:"user_email,omitempty"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	ComposeFile string                   `json:"compose_file"`
	Environment string                   `json:"environment"`
	Enabled     bool                     `json:"enabled"`
	Services    []ComposeServiceResponse `json:"services,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
}

type CreateProjectRequest struct {
	UserID      string `json:"user_id" binding:"required,uuid"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ComposeFile string `json:"compose_file" binding:"required"`
	Environment string `json:"environment"`
	Enabled     *bool  `json:"enabled"`
}

type UpdateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ComposeFile string `json:"compose_file" binding:"required"`
	Environment string `json:"environment"`
	Enabled     *bool  `json:"enabled"`
}

type ComposeServiceResponse struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Name          string    `json:"name"`
	Repo          string    `json:"repo"`
	RootDirectory string    `json:"root_directory"`
	Branch        string    `json:"branch"`
	Image         string    `json:"image"`
	Tag           string    `json:"tag"`
	Position      int       `json:"position"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ComposeServiceListResponse struct {
	Services []ComposeServiceResponse `json:"services"`
}

type CreateComposeServiceRequest struct {
	Name          string `json:"name" binding:"required"`
	Repo          string `json:"repo" binding:"required"`
	RootDirectory string `json:"root_directory"`
	Branch        string `json:"branch"`
	Image         string `json:"image" binding:"required"`
	Tag           string `json:"tag"`
	Position      int    `json:"position"`
}

type ReplaceComposeServicesRequest struct {
	Services []CreateComposeServiceRequest `json:"services" binding:"required,dive"`
}
