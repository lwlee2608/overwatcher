// Package cloudprovider maps an agent's public IP to the cloud that hosts it
// (aws/gcp/alibaba/azure) using ipinfo.io's ASN data.
package cloudprovider

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Provider is the canonical id for a recognised hosting cloud. These values are
// passed through to the frontend, where they must match the CloudProvider TS
// union.
type Provider string

const (
	AWS     Provider = "aws"
	GCP     Provider = "gcp"
	Alibaba Provider = "alibaba"
	Azure   Provider = "azure"
	Unknown Provider = "" // resolved, but not a recognised cloud
)

// Resolver looks up the hosting cloud for an IP and caches the result in memory,
// keyed by IP — an IP is queried at most once per process. Lookups are
// non-blocking: Get returns whatever is cached now and schedules a one-off
// background query for an unseen IP, so a later call (the dashboard polls every
// 10s) picks up the answer without ever stalling a request.
type Resolver struct {
	client *http.Client
	token  string

	mu       sync.Mutex
	cache    map[string]Provider // ip -> provider (Unknown means resolved-but-not-a-known-cloud)
	inflight map[string]struct{}
}

func New(token string) *Resolver {
	return &Resolver{
		client:   &http.Client{Timeout: 5 * time.Second},
		token:    token,
		cache:    make(map[string]Provider),
		inflight: make(map[string]struct{}),
	}
}

// Get returns the cached provider for ip, or Unknown when it is non-public, not
// yet resolved, or not a recognised cloud. Non-public IPs (malformed, private,
// loopback, link-local) short-circuit without a lookup. The first call for a
// public ip schedules a background lookup; a failed lookup is left uncached so
// the next call retries, while a successful one is cached permanently.
func (r *Resolver) Get(ip string) Provider {
	if r == nil {
		return Unknown
	}
	if parsed := net.ParseIP(ip); parsed == nil || !parsed.IsGlobalUnicast() || parsed.IsPrivate() {
		return Unknown
	}
	r.mu.Lock()
	if p, ok := r.cache[ip]; ok {
		r.mu.Unlock()
		return p
	}
	if _, busy := r.inflight[ip]; busy {
		r.mu.Unlock()
		return Unknown
	}
	r.inflight[ip] = struct{}{}
	r.mu.Unlock()

	go r.resolve(ip)
	return Unknown
}

func (r *Resolver) resolve(ip string) {
	provider, ok := r.lookup(ip)
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inflight, ip)
	if ok {
		r.cache[ip] = provider
	}
}

func (r *Resolver) lookup(ip string) (Provider, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipinfo.io/"+ip+"/json", nil)
	if err != nil {
		return Unknown, false
	}
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return Unknown, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Unknown, false
	}
	var body struct {
		Org string `json:"org"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Unknown, false
	}
	return classify(body.Org), true
}

// classify maps an ipinfo "org" string (e.g. "AS45102 Alibaba (US) Technology
// Co., Ltd.") to a Provider, or Unknown when it is not a recognised cloud.
// ipinfo's org name carries the network owner across all of a provider's
// regional subsidiaries, so a name match is more robust than a fixed ASN list.
func classify(org string) Provider {
	s := strings.ToLower(org)
	switch {
	case strings.Contains(s, "alibaba"), strings.Contains(s, "aliyun"):
		return Alibaba
	case strings.Contains(s, "amazon"), strings.Contains(s, "aws"):
		return AWS
	case strings.Contains(s, "google"):
		return GCP
	case strings.Contains(s, "microsoft"), strings.Contains(s, "azure"):
		return Azure
	default:
		return Unknown
	}
}
