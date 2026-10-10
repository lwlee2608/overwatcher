package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

func resourcePath(resource, id string) (string, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\") {
		return "", fmt.Errorf("invalid %s ID", resource)
	}
	return "/" + resource + "/" + url.PathEscape(id), nil
}

func (c *Client) CreateProject(ctx context.Context, input dto.CreateProjectRequest) (Response[dto.ProjectResponse], error) {
	return request[dto.ProjectResponse](ctx, c, http.MethodPost, "/projects", input)
}
func (c *Client) DeleteProject(ctx context.Context, id string) error {
	path, err := resourcePath("projects", id)
	if err != nil {
		return err
	}
	_, err = request[struct{}](ctx, c, http.MethodDelete, path, nil)
	return err
}
func (c *Client) ReplaceServices(ctx context.Context, id string, input dto.ReplaceComposeServicesRequest) (Response[dto.ComposeServiceListResponse], error) {
	path, err := resourcePath("projects", id)
	if err != nil {
		return Response[dto.ComposeServiceListResponse]{}, err
	}
	return request[dto.ComposeServiceListResponse](ctx, c, http.MethodPut, path+"/services", input)
}
func (c *Client) ListAgents(ctx context.Context) (Response[dto.AgentListResponse], error) {
	return get[dto.AgentListResponse](ctx, c, "/agents")
}
func (c *Client) BindAgent(ctx context.Context, id string, input dto.BindAgentProjectRequest) (Response[dto.AgentStatusResponse], error) {
	path, err := resourcePath("agents", id)
	if err != nil {
		return Response[dto.AgentStatusResponse]{}, err
	}
	return request[dto.AgentStatusResponse](ctx, c, http.MethodPut, path+"/project", input)
}
