package dto

import "time"

type ProjectResponse struct {
	ID                 string                   `json:"id"`
	UserID             string                   `json:"user_id"`
	UserEmail          string                   `json:"user_email,omitempty"`
	Name               string                   `json:"name"`
	Description        string                   `json:"description"`
	ComposeFile        string                   `json:"compose_file"`
	ComposeProjectName string                   `json:"compose_project_name"`
	Environment        string                   `json:"environment"`
	Enabled            bool                     `json:"enabled"`
	Role               string                   `json:"role,omitempty"`
	Services           []ComposeServiceResponse `json:"services,omitempty"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
}

type ProjectMemberResponse struct {
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	UserName  string    `json:"user_name"`
	Role      string    `json:"role"`
	AddedBy   string    `json:"added_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ProjectMemberListResponse struct {
	Members []ProjectMemberResponse `json:"members"`
}

type AddProjectMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
}

type CreateProjectRequest struct {
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
	Workflow      string    `json:"workflow"`
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
	Workflow      string `json:"workflow"`
	Position      int    `json:"position"`
}

type ReplaceComposeServicesRequest struct {
	Services []CreateComposeServiceRequest `json:"services" binding:"required,dive"`
}
