import type { AgentListResponse } from "../types/agent";

export async function fetchAgents(): Promise<AgentListResponse> {
  const res = await fetch("/api/v1/agents");
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
