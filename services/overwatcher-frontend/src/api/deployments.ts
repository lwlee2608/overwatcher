import type { DeploymentListResponse } from "../types/deployment";
import { apiFetch, apiJSON } from "./client";

export async function fetchDeployments(
  limit: number = 50
): Promise<DeploymentListResponse> {
  return apiJSON<DeploymentListResponse>(`/api/v1/deployments?limit=${limit}`);
}

export async function redeployDeployment(id: string): Promise<void> {
  const res = await apiFetch(`/api/v1/deployments/${id}/redeploy`, {
    method: "POST",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
