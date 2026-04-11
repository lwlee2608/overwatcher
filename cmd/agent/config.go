package main

import (
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
	CoordinatorURL string            `mapstructure:"coordinator_url"`
	SharedSecret   string            `mapstructure:"shared_secret" mask:"true"`
	PollTimeout    time.Duration     `mapstructure:"poll_timeout"`
	Stacks         map[string]string `mapstructure:"stacks"`
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

func InitConfig() {
	_ = godotenv.Load()

	adder.SetConfigName("application-agent")
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

	if config.Agent.PollTimeout == 0 {
		config.Agent.PollTimeout = 30 * time.Second
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
	if config.Agent.CoordinatorURL == "" {
		panic("agent.coordinator_url must be set")
	}
	if config.Agent.SharedSecret == "" {
		panic("AGENT_SHARED_SECRET must be set")
	}
	if len(config.Agent.Stacks) == 0 {
		panic("agent.stacks must declare at least one stack -> compose-file mapping")
	}
	for name, path := range config.Agent.Stacks {
		if path == "" {
			panic(fmt.Sprintf("agent.stacks[%q]: compose file path is empty", name))
		}
		if _, err := os.Stat(path); err != nil {
			panic(fmt.Sprintf("agent.stacks[%q]: compose file %q not accessible: %v", name, path, err))
		}
	}
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
