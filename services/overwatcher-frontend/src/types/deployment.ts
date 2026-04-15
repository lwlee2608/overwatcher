export interface Deployment {
  id: string;
  created_at: string;
  delivery_id: string;
  repo: string;
  ref: string;
  sha: string;
  image: string;
  tag: string;
  stack: string;
  services: string[];
  environment: string;
  status: string;
  attempts: number;
}

export interface DeploymentListResponse {
  deployments: Deployment[];
}
