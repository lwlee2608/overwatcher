import type { DeploymentListResponse } from "../types/deployment";

export async function fetchDeployments(
  limit: number = 50
): Promise<DeploymentListResponse> {
  const res = await fetch(`/api/v1/deployments?limit=${limit}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
