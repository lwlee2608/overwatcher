package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultURL = "https://overwatcher-web-production.up.railway.app"

// CLI configuration is independent of the coordinator's adder configuration.
type config struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "owctl", "config.yaml"), nil
}

func loadConfig() (config, error) {
	cfg := config{URL: defaultURL}
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, errors.New("invalid owctl config YAML")
		}
	}
	if value := os.Getenv("OVERWATCHER_URL"); value != "" {
		cfg.URL = value
	}
	if value := os.Getenv("OVERWATCHER_API_KEY"); value != "" {
		cfg.APIKey = value
	}
	return cfg, nil
}

func saveConfig(cfg config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	// Atomic replacement also avoids following an existing config-file symlink.
	file, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
