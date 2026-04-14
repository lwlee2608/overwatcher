package dto

import "time"

type AgentStatusResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ComposeFile string    `json:"compose_file"`
	LastSeen    time.Time `json:"last_seen"`
	RemoteIP    string    `json:"remote_ip"`
	Connected   bool      `json:"connected"`
}

type AgentListResponse struct {
	Agents []AgentStatusResponse `json:"agents"`
}
