package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
)

type LogConfig struct {
	Level string
}

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

const (
	LOG_LEVEL_ERROR   = "ERROR"
	LOG_LEVEL_WARNING = "WARNING"
	LOG_LEVEL_INFO    = "INFO"
	LOG_LEVEL_DEBUG   = "DEBUG"
)

var config Config

// InitConfig loads application-agent.yml and overlays env vars.
//
// Convention: application-agent.yml is the source of truth for defaults.
// Do NOT add `if config.X == 0 { config.X = ... }` fallbacks here — that
// duplicates the value across two places and silently drifts. If a field
// needs a default, set it in application-agent.yml.
func InitConfig() error {
	_ = godotenv.Load()

	adder.SetConfigName("application-agent")
	adder.AddConfigPath(".")
	adder.SetConfigType("yaml")
	adder.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	adder.AutomaticEnv()

	if err := adder.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	if err := adder.Unmarshal(&config); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(); err != nil {
		return err
	}

	initLogger(config.Log.Level)

	if strings.ToUpper(config.Log.Level) == LOG_LEVEL_DEBUG {
		configJSON, err := adder.PrettyJSON(config)
		if err == nil {
			slog.Debug("Config loaded:")
			slog.Debug(configJSON)
		}
	}

	return nil
}

func validate() error {
	if config.Agent.Name == "" {
		return errors.New("agent.name must be set")
	}
	if config.Agent.CoordinatorURL == "" {
		return errors.New("agent.coordinator_url must be set")
	}
	if config.Agent.SharedSecret == "" {
		return errors.New("AGENT_SHARED_SECRET must be set")
	}
	return nil
}

func initLogger(logLevel string) {
	var level slog.Level
	switch strings.ToUpper(logLevel) {
	case LOG_LEVEL_ERROR:
		level = slog.LevelError
	case LOG_LEVEL_WARNING:
		level = slog.LevelWarn
	case LOG_LEVEL_INFO:
		level = slog.LevelInfo
	case LOG_LEVEL_DEBUG:
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
