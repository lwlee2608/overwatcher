import type {
  DeployMappingListResponse,
  DeployMappingResponse,
  CreateDeployMappingRequest,
  UpdateDeployMappingRequest,
} from "../types/mapping";

export async function fetchMappings(): Promise<DeployMappingListResponse> {
  const res = await fetch("/api/v1/mappings");
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function createMapping(
  req: CreateDeployMappingRequest
): Promise<DeployMappingResponse> {
  const res = await fetch("/api/v1/mappings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function updateMapping(
  id: string,
  req: UpdateDeployMappingRequest
): Promise<DeployMappingResponse> {
  const res = await fetch(`/api/v1/mappings/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function deleteMapping(id: string): Promise<void> {
  const res = await fetch(`/api/v1/mappings/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
