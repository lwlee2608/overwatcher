export interface AgentStatus {
  id: string;
  name: string;
  compose_file: string;
  last_seen: string;
  remote_ip: string;
  connected: boolean;
}

export interface AgentListResponse {
  agents: AgentStatus[];
}
