package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// Response retains the original JSON, including fields unknown to this client.
type Response[T any] struct {
	Data T
	Raw  json.RawMessage
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string { return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message) }

func New(baseURL, apiKey string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("URL must be an http or https server URL without credentials, query, or fragment")
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) ListProjects(ctx context.Context) (Response[dto.ProjectListResponse], error) {
	return get[dto.ProjectListResponse](ctx, c, "/projects")
}
func (c *Client) GetProject(ctx context.Context, id string) (Response[dto.ProjectResponse], error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\") {
		return Response[dto.ProjectResponse]{}, fmt.Errorf("invalid project ID")
	}
	return get[dto.ProjectResponse](ctx, c, "/projects/"+url.PathEscape(id))
}
func (c *Client) Version(ctx context.Context) (Response[dto.VersionResponse], error) {
	return get[dto.VersionResponse](ctx, c, "/version")
}

func get[T any](ctx context.Context, c *Client, path string) (Response[T], error) {
	return request[T](ctx, c, http.MethodGet, path, nil)
}

func request[T any](ctx context.Context, c *Client, method, path string, payload any) (Response[T], error) {
	var result Response[T]
	var input io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return result, fmt.Errorf("encode API request: %w", err)
		}
		input = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, input)
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return result, fmt.Errorf("request API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		return result, fmt.Errorf("read API response: %w", err)
	}
	if len(body) > 16*1024*1024 {
		return result, fmt.Errorf("API response exceeds 16 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &payload)
		if payload.Error == "" {
			payload.Error = http.StatusText(resp.StatusCode)
		}
		return result, &APIError{StatusCode: resp.StatusCode, Message: payload.Error}
	}
	if resp.StatusCode == http.StatusNoContent {
		return result, nil
	}
	if err := json.Unmarshal(body, &result.Data); err != nil {
		return result, fmt.Errorf("decode API response: %w", err)
	}
	result.Raw = body
	return result, nil
}
