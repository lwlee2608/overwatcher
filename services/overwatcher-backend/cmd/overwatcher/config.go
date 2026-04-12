package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
	"github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
)

type GitHubConfig struct {
	AppID         int64  `mapstructure:"app_id"`
	WebhookSecret string `mapstructure:"webhook_secret" mask:"true"`
	PrivateKey    string `mapstructure:"private_key" mask:"true"`
}

type AgentConfig struct {
	SharedSecret string `mapstructure:"shared_secret" mask:"true"`
}

type Config struct {
	Log         LogConfig
	Http        http.Config
	GitHub      GitHubConfig   `mapstructure:"github"`
	Deployments mapping.Config `mapstructure:"deployments"`
	Agent       AgentConfig    `mapstructure:"agent"`
}

var config Config

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
	for i, m := range config.Deployments.Mappings {
		if m.Repo == "" {
			return fmt.Errorf("deployments.mappings[%d]: repo is required", i)
		}
		if m.Stack == "" {
			return fmt.Errorf("deployments.mappings[%d]: stack is required", i)
		}
	}
	return nil
}
