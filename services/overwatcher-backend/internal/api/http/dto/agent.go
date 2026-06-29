package dto

import "time"

type AgentStatusResponse struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	LastSeen      *time.Time    `json:"last_seen,omitempty"` // LastSeen is omitted until the agent's first heartbeat (never connected).
	RemoteIP      string        `json:"remote_ip"`
	CloudProvider string        `json:"cloud_provider,omitempty"`
	Status        string        `json:"status"`
	ProjectID     string        `json:"project_id,omitempty"`
	ProjectName   string        `json:"project_name,omitempty"`
	Type          string        `json:"type,omitempty"`
	Version       string        `json:"version,omitempty"`
	Metrics       *AgentMetrics `json:"metrics,omitempty"`
}

// AgentMetrics is the agent's last reported resource snapshot, no fresher
// than last_seen. Omitted until the agent first reports. swap_total_bytes is
// 0 when the host has no swap configured.
type AgentMetrics struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	DiskUsedBytes  uint64  `json:"disk_used_bytes"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
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
