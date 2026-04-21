export interface UserResponse {
  id: string;
  email: string;
  name: string;
  created_at: string;
  updated_at: string;
}

export interface UserListResponse {
  users: UserResponse[];
}

export interface CreateUserRequest {
  email: string;
  name: string;
}

export interface UpdateUserRequest {
  email: string;
  name: string;
}
