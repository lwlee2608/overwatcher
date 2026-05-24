export type AgentType = "systemd" | "docker";

export interface AgentStatus {
  id: string;
  name: string;
  last_seen: string;
  remote_ip: string;
  connected: boolean;
  project_id?: string;
  type?: AgentType;
}

export interface AgentListResponse {
  agents: AgentStatus[];
}

export interface BindAgentProjectRequest {
  project_id: string;
}
