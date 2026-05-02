import type {
  UserListResponse,
  UserResponse,
  CreateUserRequest,
  UpdateUserRequest,
} from "../types/user";
import { apiFetch, apiJSON } from "./client";

export async function fetchUsers(): Promise<UserListResponse> {
  return apiJSON<UserListResponse>("/api/v1/users");
}

export async function createUser(req: CreateUserRequest): Promise<UserResponse> {
  return apiJSON<UserResponse>("/api/v1/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export async function updateUser(
  id: string,
  req: UpdateUserRequest
): Promise<UserResponse> {
  return apiJSON<UserResponse>(`/api/v1/users/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export async function deleteUser(id: string): Promise<void> {
  const res = await apiFetch(`/api/v1/users/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
