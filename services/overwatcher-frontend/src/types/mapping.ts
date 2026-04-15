export interface DeployMappingResponse {
  id: string;
  repo: string;
  agent_id: string;
  agent_name: string;
  services: string[];
  environment: string;
  image: string;
  tag: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface DeployMappingListResponse {
  mappings: DeployMappingResponse[];
}

export interface CreateDeployMappingRequest {
  repo: string;
  agent_id: string;
  services: string[];
  environment: string;
  image: string;
  tag: string;
  enabled: boolean;
}

export interface UpdateDeployMappingRequest {
  repo: string;
  agent_id: string;
  services: string[];
  environment: string;
  image: string;
  tag: string;
  enabled: boolean;
}
