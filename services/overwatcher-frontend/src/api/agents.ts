import type {
  AgentListResponse,
  AgentStatus,
  BindAgentProjectRequest,
} from "../types/agent";
import { apiFetch, apiJSON } from "./client";

export async function fetchAgents(): Promise<AgentListResponse> {
  return apiJSON<AgentListResponse>("/api/v1/agents");
}

export interface AgentTokenResponse {
  agent_id: string;
  name: string;
  agent_token: string;
}

// createAgent provisions a new agent and returns its token. The raw token is
// returned exactly once — there's no way to fetch it again later.
export async function createAgent(name: string): Promise<AgentTokenResponse> {
  const res = await apiFetch("/api/v1/agents", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    const body = await res.text();
    let msg = body || `HTTP ${res.status}`;
    try {
      const parsed = JSON.parse(body) as { error?: string };
      if (parsed.error) msg = parsed.error;
    } catch {
      // body is not JSON; use raw text
    }
    throw new Error(msg);
  }
  return res.json();
}

export async function bindAgentProject(
  agentId: string,
  req: BindAgentProjectRequest,
): Promise<AgentStatus> {
  const res = await apiFetch(`/api/v1/agents/${agentId}/project`, {
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

export async function deleteAgent(agentId: string): Promise<void> {
  const res = await apiFetch(`/api/v1/agents/${agentId}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.text();
    let msg = body || `HTTP ${res.status}`;
    try {
      const parsed = JSON.parse(body) as { error?: string };
      if (parsed.error) msg = parsed.error;
    } catch {
      // body is not JSON; use raw text
    }
    throw new Error(msg);
  }
}
