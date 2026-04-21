import type {
  ProjectListResponse,
  ProjectResponse,
  CreateProjectRequest,
  UpdateProjectRequest,
  ComposeServiceListResponse,
  ReplaceComposeServicesRequest,
} from "../types/project";

export async function fetchProjects(): Promise<ProjectListResponse> {
  const res = await fetch("/api/v1/projects");
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function fetchProject(id: string): Promise<ProjectResponse> {
  const res = await fetch(`/api/v1/projects/${id}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function createProject(
  req: CreateProjectRequest
): Promise<ProjectResponse> {
  const res = await fetch("/api/v1/projects", {
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

export async function updateProject(
  id: string,
  req: UpdateProjectRequest
): Promise<ProjectResponse> {
  const res = await fetch(`/api/v1/projects/${id}`, {
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

export async function deleteProject(id: string): Promise<void> {
  const res = await fetch(`/api/v1/projects/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}

export async function replaceProjectServices(
  id: string,
  req: ReplaceComposeServicesRequest
): Promise<ComposeServiceListResponse> {
  const res = await fetch(`/api/v1/projects/${id}/services`, {
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
