package dto

import "time"

type AgentStatusResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	LastSeen    time.Time `json:"last_seen"`
	RemoteIP    string    `json:"remote_ip"`
	Status      string    `json:"status"`
	ProjectID   string    `json:"project_id,omitempty"`
	ProjectName string    `json:"project_name,omitempty"`
	Type        string    `json:"type,omitempty"`
	Version     string    `json:"version,omitempty"`
}

type AgentListResponse struct {
	Agents []AgentStatusResponse `json:"agents"`
}

// BindAgentProjectRequest sets the project binding on an agent.
// An empty project_id clears the binding.
type BindAgentProjectRequest struct {
	ProjectID string `json:"project_id"`
}

// CreateAgentRequest pre-provisions an agent. The name must be unique.
type CreateAgentRequest struct {
	Name string `json:"name" binding:"required"`
}

// AgentTokenResponse carries a freshly minted token. The raw token is returned
// exactly once (create or re-issue) and is never retrievable again.
type AgentTokenResponse struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
	Token   string `json:"agent_token"`
}
