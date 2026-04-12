package dto

import "time"

type AgentStatusResponse struct {
	Name      string    `json:"name"`
	Stacks    []string  `json:"stacks"`
	LastSeen  time.Time `json:"last_seen"`
	RemoteIP  string    `json:"remote_ip"`
	Connected bool      `json:"connected"`
}

type AgentListResponse struct {
	Agents []AgentStatusResponse `json:"agents"`
}
