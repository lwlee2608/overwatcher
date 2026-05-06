import type {
  ProjectListResponse,
  ProjectResponse,
  CreateProjectRequest,
  UpdateProjectRequest,
  ComposeServiceListResponse,
  ReplaceComposeServicesRequest,
  ProjectMemberListResponse,
  ProjectMemberResponse,
  AddProjectMemberRequest,
} from "../types/project";
import { apiFetch, apiJSON } from "./client";

export async function fetchProjects(): Promise<ProjectListResponse> {
  return apiJSON<ProjectListResponse>("/api/v1/projects");
}

export async function fetchProject(id: string): Promise<ProjectResponse> {
  return apiJSON<ProjectResponse>(`/api/v1/projects/${id}`);
}

export async function createProject(
  req: CreateProjectRequest
): Promise<ProjectResponse> {
  return apiJSON<ProjectResponse>("/api/v1/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export async function updateProject(
  id: string,
  req: UpdateProjectRequest
): Promise<ProjectResponse> {
  return apiJSON<ProjectResponse>(`/api/v1/projects/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export async function deleteProject(id: string): Promise<void> {
  const res = await apiFetch(`/api/v1/projects/${id}`, { method: "DELETE" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}

export async function fetchProjectMembers(
  id: string
): Promise<ProjectMemberListResponse> {
  return apiJSON<ProjectMemberListResponse>(`/api/v1/projects/${id}/members`);
}

export async function addProjectMember(
  id: string,
  req: AddProjectMemberRequest
): Promise<ProjectMemberResponse> {
  return apiJSON<ProjectMemberResponse>(`/api/v1/projects/${id}/members`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}

export async function removeProjectMember(
  id: string,
  userId: string
): Promise<void> {
  const res = await apiFetch(`/api/v1/projects/${id}/members/${userId}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
}

export async function replaceProjectServices(
  id: string,
  req: ReplaceComposeServicesRequest
): Promise<ComposeServiceListResponse> {
  return apiJSON<ComposeServiceListResponse>(`/api/v1/projects/${id}/services`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });
}
