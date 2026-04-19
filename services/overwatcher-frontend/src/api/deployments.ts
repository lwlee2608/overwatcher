import type { DeploymentListResponse } from "../types/deployment";

export async function fetchDeployments(
  limit: number = 50
): Promise<DeploymentListResponse> {
  const res = await fetch(`/api/v1/deployments?limit=${limit}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function redeployDeployment(id: string): Promise<void> {
  const res = await fetch(`/api/v1/deployments/${id}/redeploy`, {
    method: "POST",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
