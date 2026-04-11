package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
	"github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/service"
)

type GitHubConfig struct {
	AppID         int64  `mapstructure:"app_id"`
	WebhookSecret string `mapstructure:"webhook_secret" mask:"true"`
	PrivateKey    string `mapstructure:"private_key" mask:"true"`
}

type Config struct {
	Log         LogConfig
	Http        http.Config
	GitHub      GitHubConfig              `mapstructure:"github"`
	Deployments service.DeploymentsConfig `mapstructure:"deployments"`
}

var config Config

func InitConfig() {
	_ = godotenv.Load()

	adder.SetConfigName("application")
	adder.AddConfigPath(".")
	adder.SetConfigType("yaml")
	adder.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	adder.AutomaticEnv()

	if err := adder.ReadInConfig(); err != nil {
		panic(err)
	}

	if err := adder.Unmarshal(&config); err != nil {
		panic(err)
	}

	validate()

	initLogger(config.Log.Level)

	if strings.ToUpper(config.Log.Level) == LOG_LEVEL_DEBUG {
		configJSON, err := adder.PrettyJSON(config)
		if err == nil {
			slog.Debug("Config loaded:")
			slog.Debug(configJSON)
		}
	}
}

func validate() {
	if config.GitHub.WebhookSecret == "" {
		panic("GITHUB_WEBHOOK_SECRET must be set")
	}
	for i, m := range config.Deployments.Mappings {
		if m.Repo == "" {
			panic(fmt.Sprintf("deployments.mappings[%d]: repo is required", i))
		}
		if m.Stack == "" {
			panic(fmt.Sprintf("deployments.mappings[%d]: stack is required", i))
		}
	}
}
