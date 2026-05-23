package agent

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
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

// defaultConfigYAML is the source of truth for agent defaults. It is shipped
// inside the binary so the systemd deployment (which has no on-disk YAML)
// still has a complete set of defaults that env vars then override.
//
//go:embed application-agent.yml
var defaultConfigYAML []byte

// InitConfig loads agent config in this order, each later layer overriding
// the previous:
//
//  1. Embedded application-agent.yml (always present).
//  2. ./application-agent.yml on disk, if found.
//  3. Env vars (handled by adder.AutomaticEnv).
//
// The embedded fallback means the binary works as a systemd unit with no
// on-disk YAML file. Convention from rule 3 of the adder skill stands:
// defaults belong in application-agent.yml, not in code.
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

// resolveConfigPath returns the YAML file path adder should read. If
// ./application-agent.yml is present it wins (lets local dev override
// defaults). Otherwise the embedded copy is materialised to a temp file
// and that path is returned, along with a cleanup func to remove it.
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

	// Sanity: make sure the path is absolute so adder doesn't try to walk
	// search paths.
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
	if cfg.Agent.SharedSecret == "" {
		return errors.New("AGENT_SHARED_SECRET must be set")
	}
	return nil
}
