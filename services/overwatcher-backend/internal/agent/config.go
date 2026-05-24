package agent

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
)

type AgentConfig struct {
	Name           string        `mapstructure:"name"`
	CoordinatorURL string        `mapstructure:"coordinator_url"`
	SharedSecret   string        `mapstructure:"shared_secret" mask:"true"`
	PollTimeout    time.Duration `mapstructure:"poll_timeout"`
}

type Config struct {
	Log   LogConfig
	Agent AgentConfig `mapstructure:"agent"`
}

// Embedded so the systemd binary runs without on-disk YAML; env vars override.
//
//go:embed application-agent.yml
var defaultConfigYAML []byte

func InitConfig() (Config, error) {
	var cfg Config

	_ = godotenv.Load()

	configPath, cleanup, err := resolveConfigPath()
	if err != nil {
		return cfg, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	adder.SetConfigFile(configPath)
	adder.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	adder.AutomaticEnv()

	if err := adder.ReadInConfig(); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := adder.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return cfg, err
	}

	initLogger(cfg.Log.Level)

	if strings.ToUpper(cfg.Log.Level) == LOG_LEVEL_DEBUG {
		configJSON, err := adder.PrettyJSON(cfg)
		if err == nil {
			slog.Debug("Config loaded:")
			slog.Debug(configJSON)
		}
	}

	return cfg, nil
}

// On-disk file wins so local dev can override; otherwise materialise the
// embedded copy because adder.ReadInConfig needs a path.
func resolveConfigPath() (string, func(), error) {
	const onDisk = "application-agent.yml"
	if _, err := os.Stat(onDisk); err == nil {
		return onDisk, nil, nil
	}

	tmp, err := os.CreateTemp("", "overwatcher-agent-*.yml")
	if err != nil {
		return "", nil, fmt.Errorf("write embedded config: %w", err)
	}
	if _, err := tmp.Write(defaultConfigYAML); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("write embedded config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("write embedded config: %w", err)
	}
	path := tmp.Name()
	cleanup := func() { _ = os.Remove(path) }

	// Absolute path so adder doesn't walk its search paths.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path, cleanup, nil
}

func validate(cfg *Config) error {
	if cfg.Agent.Name == "" {
		return errors.New("agent.name must be set")
	}
	if cfg.Agent.CoordinatorURL == "" {
		return errors.New("agent.coordinator_url must be set")
	}
	if err := validateCoordinatorURL(cfg.Agent.CoordinatorURL); err != nil {
		return err
	}
	if cfg.Agent.SharedSecret == "" {
		return errors.New("AGENT_SHARED_SECRET must be set")
	}
	return nil
}

// validateCoordinatorURL rejects plain http:// for non-loopback hosts. Reason:
// edge proxies (Railway, Cloudflare) 301 http→https, and Go's http client
// silently downgrades POST→GET on 301 — so /deploy/:id/result becomes a GET,
// hits no route, and the agent never reports deploy outcomes. Loopback stays
// allowed for local dev where no such proxy exists.
func validateCoordinatorURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("agent.coordinator_url: %w", err)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("agent.coordinator_url must use https:// (got %q); plain http to non-loopback hosts breaks POST through 301 redirects", raw)
	default:
		return fmt.Errorf("agent.coordinator_url: unsupported scheme %q", u.Scheme)
	}
}
