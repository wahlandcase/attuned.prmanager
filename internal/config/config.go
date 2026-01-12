package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Paths   PathsConfig   `toml:"paths"`
	Tickets TicketsConfig `toml:"tickets"`

	// Compiled regex from Tickets.Pattern (not serialized)
	ticketRegex *regexp.Regexp
}

type PathsConfig struct {
	AttunedDir   string `toml:"attuned_dir"`
	FrontendGlob string `toml:"frontend_glob"`
	BackendGlob  string `toml:"backend_glob"`
}

type TicketsConfig struct {
	Pattern   string `toml:"pattern"`
	LinearOrg string `toml:"linear_org"`
}

func DefaultConfig() *Config {
	return &Config{
		Paths: PathsConfig{
			AttunedDir:   "~/Programming/attuned",
			FrontendGlob: "frontend/*",
			BackendGlob:  "backend/*",
		},
		Tickets: TicketsConfig{
			Pattern:   "ATT-[0-9]+",
			LinearOrg: "attuned",
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
		cfg := DefaultConfig()
		if err := cfg.compileRegex(); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := cfg.compileRegex(); err != nil {
				return nil, err
			}
			_ = cfg.Save() // Best effort save
			return cfg, nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := cfg.compileRegex(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) compileRegex() error {
	// Empty pattern = ticket extraction disabled
	if c.Tickets.Pattern == "" {
		c.ticketRegex = nil
		return nil
	}
	re, err := regexp.Compile("(?i)(" + c.Tickets.Pattern + ")")
	if err != nil {
		return fmt.Errorf("invalid tickets.pattern %q: %w", c.Tickets.Pattern, err)
	}
	c.ticketRegex = re
	return nil
}

// TicketRegex returns the compiled ticket pattern regex (nil if disabled)
func (c *Config) TicketRegex() *regexp.Regexp {
	// Safe even if compileRegex() was never called
	return c.ticketRegex
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
