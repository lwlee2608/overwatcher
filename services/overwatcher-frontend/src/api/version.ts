import { apiJSON } from "./client";

export interface VersionResponse {
  version: string;
  release_tag: string;
}

// fetchVersion reports the coordinator build version and the agent release tag
// derived from it — the release the install command installs.
export async function fetchVersion(): Promise<VersionResponse> {
  return apiJSON<VersionResponse>("/api/v1/version");
}
