package main

import (
	"strings"
	"log/slog"

	"github.com/joho/godotenv"
	"github.com/lwlee2608/adder"
	"github.com/lwlee2608/overwatcher/internal/api/http"
)

type GitHubConfig struct {
	AppID         int64  `mapstructure:"app_id"`
	WebhookSecret string `mapstructure:"webhook_secret" mask:"true"`
	PrivateKey    string `mapstructure:"private_key" mask:"true"`
}

type Config struct {
	Log    LogConfig
	Http   http.Config
	GitHub GitHubConfig `mapstructure:"github"`
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
}
