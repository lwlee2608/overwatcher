import type {
  UserListResponse,
  UserResponse,
  CreateUserRequest,
  UpdateUserRequest,
} from "../types/user";

export async function fetchUsers(): Promise<UserListResponse> {
  const res = await fetch("/api/v1/users");
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function createUser(req: CreateUserRequest): Promise<UserResponse> {
  const res = await fetch("/api/v1/users", {
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

export async function updateUser(
  id: string,
  req: UpdateUserRequest
): Promise<UserResponse> {
  const res = await fetch(`/api/v1/users/${id}`, {
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

export async function deleteUser(id: string): Promise<void> {
  const res = await fetch(`/api/v1/users/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
