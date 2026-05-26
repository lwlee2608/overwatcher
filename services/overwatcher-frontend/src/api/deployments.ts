import type { DeploymentListResponse } from "../types/deployment";
import { apiFetch, apiJSON } from "./client";

export interface DeploymentFilter {
  status?: string;
  project_id?: string;
  repo?: string;
  environment?: string;
}

export async function fetchDeployments(
  page: number = 1,
  pageSize: number = 25,
  filter: DeploymentFilter = {},
): Promise<DeploymentListResponse> {
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  for (const [k, v] of Object.entries(filter)) {
    if (v) params.set(k, v);
  }
  return apiJSON<DeploymentListResponse>(`/api/v1/deployments?${params}`);
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
