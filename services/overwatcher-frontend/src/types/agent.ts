export interface AgentStatus {
  name: string;
  stacks: string[];
  last_seen: string;
  remote_ip: string;
  connected: boolean;
}

export interface AgentListResponse {
  agents: AgentStatus[];
}
