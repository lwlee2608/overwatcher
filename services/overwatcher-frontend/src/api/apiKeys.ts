import type {
  APIKeyListResponse,
  CreateAPIKeyResponse,
} from "../types/apiKey";
import { apiFetch, apiJSON } from "./client";

export async function fetchAPIKeys(): Promise<APIKeyListResponse> {
  return apiJSON<APIKeyListResponse>("/api/v1/api-keys");
}

export async function createAPIKey(name: string): Promise<CreateAPIKeyResponse> {
  return apiJSON<CreateAPIKeyResponse>("/api/v1/api-keys", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
}

export async function deleteAPIKey(id: string): Promise<void> {
  const res = await apiFetch(`/api/v1/api-keys/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
