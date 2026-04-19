export interface ServiceSpec {
  name: string;
  image: string;
  tag: string;
}

export interface DeployMappingResponse {
  id: string;
  repo: string;
  agent_id: string;
  agent_name: string;
  services: ServiceSpec[];
  environment: string;
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
  services: ServiceSpec[];
  environment: string;
  enabled: boolean;
}

export interface UpdateDeployMappingRequest {
  repo: string;
  agent_id: string;
  services: ServiceSpec[];
  environment: string;
  enabled: boolean;
}
