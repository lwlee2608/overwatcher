export type AgentType = "systemd" | "docker";

export type ConnectionStatus = "connected" | "stale" | "disconnected" | "lost";

export interface AgentMetrics {
  cpu_percent: number;
  mem_used_bytes: number;
  mem_total_bytes: number;
  disk_used_bytes: number;
  disk_total_bytes: number;
}

export interface AgentStatus {
  id: string;
  name: string;
  last_seen: string;
  remote_ip: string;
  status: ConnectionStatus;
  project_id?: string;
  project_name?: string;
  type?: AgentType;
  version?: string;
  metrics?: AgentMetrics;
}

export interface AgentListResponse {
  agents: AgentStatus[];
}

export interface BindAgentProjectRequest {
  project_id: string;
}
