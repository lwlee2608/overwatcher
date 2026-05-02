import type {
  ChangePasswordRequest,
  LoginRequest,
  MeResponse,
} from "../types/auth";

// /auth/login is unauthenticated, so it bypasses apiFetch's 401 redirect on
// purpose: a bad password should surface as an error in the form, not as a
// page reload.
export async function login(req: LoginRequest): Promise<MeResponse> {
  const res = await fetch("/api/v1/auth/login", {
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

export async function logout(): Promise<void> {
  await fetch("/api/v1/auth/logout", { method: "POST" });
}

// Returns null when the caller is not logged in. Any other failure throws.
export async function fetchMe(): Promise<MeResponse | null> {
  const res = await fetch("/api/v1/auth/me");
  if (res.status === 401) return null;
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function changePassword(req: ChangePasswordRequest): Promise<void> {
  const res = await fetch("/api/v1/auth/password", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}
