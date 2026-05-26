export interface ServiceSpec {
  name: string;
  image: string;
  tag: string;
}

export interface Deployment {
  id: string;
  created_at: string;
  delivery_id: string;
  project_id?: string;
  repo: string;
  ref: string;
  sha: string;
  stack: string;
  services: ServiceSpec[];
  environment: string;
  status: string;
  attempts: number;
}

export interface DeploymentListResponse {
  deployments: Deployment[];
  total: number;
  page: number;
  page_size: number;
}
