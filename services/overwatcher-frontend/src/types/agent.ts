export type AgentType = "systemd" | "docker";

export type ConnectionStatus = "connected" | "stale" | "disconnected" | "lost";

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
}

export interface AgentListResponse {
  agents: AgentStatus[];
}

export interface BindAgentProjectRequest {
  project_id: string;
}
