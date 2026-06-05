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

// BindAgentProjectRequest binds an agent; an empty project_id clears the binding.
type BindAgentProjectRequest struct {
	ProjectID string `json:"project_id"`
}

type CreateAgentRequest struct {
	Name string `json:"name" binding:"required"`
	// Token, when set, is a token minted earlier via MintToken. Empty means
	// generate a fresh one on create.
	Token string `json:"agent_token"`
}

// AgentTokenResponse carries a freshly minted token. The raw token is returned
// exactly once (create or re-issue) and is never retrievable again.
type AgentTokenResponse struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
	Token   string `json:"agent_token"`
}
