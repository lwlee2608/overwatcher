import type {
  AgentListResponse,
  AgentStatus,
  BindAgentProjectRequest,
} from "../types/agent";

export async function fetchAgents(): Promise<AgentListResponse> {
  const res = await fetch("/api/v1/agents");
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function bindAgentProject(
  agentId: string,
  req: BindAgentProjectRequest,
): Promise<AgentStatus> {
  const res = await fetch(`/api/v1/agents/${agentId}/project`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(body || `HTTP ${res.status}`);
  }
  return res.json();
}
