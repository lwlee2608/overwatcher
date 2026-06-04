package dto

type VersionResponse struct {
	Version    string `json:"version"`
	ReleaseTag string `json:"release_tag"`
}
