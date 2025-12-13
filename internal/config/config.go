package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Paths   PathsConfig   `toml:"paths"`
	Tickets TicketsConfig `toml:"tickets"`
}

type PathsConfig struct {
	AttunedDir   string `toml:"attuned_dir"`
	FrontendGlob string `toml:"frontend_glob"`
	BackendGlob  string `toml:"backend_glob"`
}

type TicketsConfig struct {
	Pattern string `toml:"pattern"`
}

func DefaultConfig() *Config {
	return &Config{
		Paths: PathsConfig{
			AttunedDir:   "~/Programming/attuned",
			FrontendGlob: "frontend/*",
			BackendGlob:  "backend/*",
		},
		Tickets: TicketsConfig{
			Pattern: "ATT-[0-9]+",
		},
	}
}

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "attuned-release.toml"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			_ = cfg.Save() // Best effort save
			return cfg, nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) AttunedPath() string {
	return expandTilde(c.Paths.AttunedDir)
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
