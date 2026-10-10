package client

import (
	"context"
	"net/http"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

func (c *Client) ServerURL() string { return c.baseURL }

func (c *Client) CreateAgent(ctx context.Context, input dto.CreateAgentRequest) (Response[dto.AgentTokenResponse], error) {
	return request[dto.AgentTokenResponse](ctx, c, http.MethodPost, "/agents", input)
}

func (c *Client) GetAgent(ctx context.Context, id string) (Response[dto.AgentStatusResponse], error) {
	path, err := resourcePath("agents", id)
	if err != nil {
		return Response[dto.AgentStatusResponse]{}, err
	}
	return get[dto.AgentStatusResponse](ctx, c, path)
}

func (c *Client) DeleteAgent(ctx context.Context, id string) error {
	path, err := resourcePath("agents", id)
	if err != nil {
		return err
	}
	_, err = request[struct{}](ctx, c, http.MethodDelete, path, nil)
	return err
}
