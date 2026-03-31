package github

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gh "github.com/google/go-github/v84/github"
)

type Client struct {
	appID      int64
	privateKey []byte
}

func NewClient(appID int64, privateKey []byte) *Client {
	return &Client{
		appID:      appID,
		privateKey: privateKey,
	}
}

func (c *Client) GetInstallationClient(ctx context.Context, installationID int64) (*gh.Client, error) {
	transport, err := ghinstallation.New(http.DefaultTransport, c.appID, installationID, c.privateKey)
	if err != nil {
		slog.Error("Failed to create installation transport", "installation_id", installationID, "error", err)
		return nil, err
	}
	return gh.NewClient(&http.Client{Transport: transport}), nil
}
