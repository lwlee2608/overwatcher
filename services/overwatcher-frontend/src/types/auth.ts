export interface MeResponse {
  id: string;
  email: string;
  name: string;
  must_change_password: boolean;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}
