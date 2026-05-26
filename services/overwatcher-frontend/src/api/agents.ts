import type {
  AgentListResponse,
  AgentStatus,
  BindAgentProjectRequest,
} from "../types/agent";
import { apiFetch, apiJSON } from "./client";

export async function fetchAgents(): Promise<AgentListResponse> {
  return apiJSON<AgentListResponse>("/api/v1/agents");
}

export interface InstallConfig {
  shared_secret: string;
}

export async function fetchInstallConfig(): Promise<InstallConfig> {
  return apiJSON<InstallConfig>("/api/v1/install/config");
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
