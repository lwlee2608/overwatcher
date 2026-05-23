package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
	"github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/db"
	"github.com/lwlee2608/overwatcher/internal/service/auth"
)

type GitHubConfig struct {
	AppID         int64  `mapstructure:"app_id"`
	WebhookSecret string `mapstructure:"webhook_secret" mask:"true"`
	PrivateKey    string `mapstructure:"private_key" mask:"true"`
}

type AgentConfig struct {
	SharedSecret string `mapstructure:"shared_secret" mask:"true"`
	ReleaseTag   string `mapstructure:"release_tag"`
	PublicURL    string `mapstructure:"public_url"`
}

type DispatchConfig struct {
	InFlightTimeout time.Duration `mapstructure:"in_flight_timeout"`
	MaxAttempts     int           `mapstructure:"max_attempts"`
	SweepInterval   time.Duration `mapstructure:"sweep_interval"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type AuthConfig struct {
	SessionTTL time.Duration           `mapstructure:"session_ttl"`
	Cookie     middleware.CookieConfig `mapstructure:"cookie"`
	Bootstrap  auth.BootstrapConfig    `mapstructure:"bootstrap"`
}

type Config struct {
	Log      LogConfig
	Http     http.Config
	GitHub   GitHubConfig   `mapstructure:"github"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Dispatch DispatchConfig `mapstructure:"dispatch"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Database db.Config      `mapstructure:"database"`
}

var config Config

// InitConfig loads application.yml and overlays env vars.
//
// Convention: application.yml is the source of truth for defaults. Do NOT
// add `if config.X == 0 { config.X = ... }` fallbacks here — that
// duplicates the value across two places and silently drifts. If a field
// needs a default, set it in application.yml. Only runtime-resolved values
// (e.g. hostname lookups) belong in code.
func InitConfig() error {
	_ = godotenv.Load()

	adder.SetConfigName("application")
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
	if config.GitHub.WebhookSecret == "" {
		return errors.New("GITHUB_WEBHOOK_SECRET must be set")
	}
	if config.Agent.SharedSecret == "" {
		return errors.New("AGENT_SHARED_SECRET must be set")
	}
	if config.Database.URL == "" {
		return errors.New("database.url must be set")
	}
	return nil
}
