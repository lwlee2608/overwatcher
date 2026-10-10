package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func isolatedConfig(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OVERWATCHER_URL", "")
	t.Setenv("OVERWATCHER_API_KEY", "")
}

func TestConfig(t *testing.T) {
	isolatedConfig(t)
	cfg, err := loadConfig()
	require.NoError(t, err)
	require.Equal(t, defaultURL, cfg.URL)
	require.Empty(t, cfg.APIKey)
	require.NoError(t, saveConfig(config{URL: "http://localhost:1234", APIKey: "owk_secret"}))
	path, err := configPath()
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, "owk_secret", cfg.APIKey)
	require.Equal(t, "http://localhost:1234", cfg.URL)
	require.NoError(t, os.Chmod(path, 0644))
	require.NoError(t, saveConfig(config{URL: "http://localhost:1235", APIKey: "owk_replaced"}))
	info, err = os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	t.Setenv("OVERWATCHER_URL", "http://localhost:9876")
	t.Setenv("OVERWATCHER_API_KEY", "owk_environment")
	cfg, err = loadConfig()
	require.NoError(t, err)
	require.Equal(t, "http://localhost:9876", cfg.URL)
	require.Equal(t, "owk_environment", cfg.APIKey)
}

func TestConfigSymlinkReplacement(t *testing.T) {
	isolatedConfig(t)
	target := filepath.Join(t.TempDir(), "untouched")
	require.NoError(t, os.WriteFile(target, []byte("keep"), 0644))
	path, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.Symlink(target, path))
	require.NoError(t, saveConfig(config{URL: defaultURL, APIKey: "secret"}))
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "keep", string(data))
}

func TestCommandValidation(t *testing.T) {
	isolatedConfig(t)
	for _, args := range [][]string{{"project", "get"}, {"project", "list", "extra"}, {"version", "extra"}, {"login", "extra"}, {"project", "list"}, {"login"}} {
		cmd := newRootCommand()
		cmd.SetArgs(args)
		cmd.SetIn(bytes.NewBufferString("secret\n"))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		require.Error(t, cmd.Execute())
		require.NotContains(t, out.String(), "secret")
	}
}

func TestMalformedConfigDoesNotExposeKey(t *testing.T) {
	isolatedConfig(t)
	path, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("api_key: [secret"), 0600))
	_, err = loadConfig()
	require.EqualError(t, err, "invalid owctl config YAML")
}
