export interface ComposeServiceResponse {
  id: string;
  project_id: string;
  name: string;
  repo: string;
  root_directory: string;
  branch: string;
  image: string;
  tag: string;
  workflow: string;
  position: number;
  created_at: string;
  updated_at: string;
}

export interface ComposeServiceListResponse {
  services: ComposeServiceResponse[];
}

export interface ProjectResponse {
  id: string;
  user_id: string;
  user_email?: string;
  name: string;
  description: string;
  compose_file: string;
  compose_project_name: string;
  environment: string;
  enabled: boolean;
  role?: "owner" | "member";
  services?: ComposeServiceResponse[];
  created_at: string;
  updated_at: string;
}

export interface ProjectMemberResponse {
  user_id: string;
  user_email: string;
  user_name: string;
  role: string;
  added_by?: string;
  created_at: string;
}

export interface ProjectMemberListResponse {
  members: ProjectMemberResponse[];
}

export interface AddProjectMemberRequest {
  email: string;
}

export interface ProjectListResponse {
  projects: ProjectResponse[];
}

export interface CreateProjectRequest {
  name: string;
  description: string;
  compose_file: string;
  environment: string;
  enabled: boolean;
}

export interface UpdateProjectRequest {
  name: string;
  description: string;
  compose_file: string;
  environment: string;
  enabled: boolean;
}

export interface CreateComposeServiceRequest {
  name: string;
  repo: string;
  root_directory: string;
  branch: string;
  image: string;
  tag: string;
  workflow: string;
  position: number;
}

export interface ReplaceComposeServicesRequest {
  services: CreateComposeServiceRequest[];
}
