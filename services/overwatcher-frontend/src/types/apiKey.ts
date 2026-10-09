export interface APIKeyResponse {
  id: string;
  name: string;
  last_used_at?: string;
  created_at: string;
}

export interface APIKeyListResponse {
  keys: APIKeyResponse[];
}

export interface CreateAPIKeyResponse extends APIKeyResponse {
  key: string;
}
