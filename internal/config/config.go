package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Destination represents an rclone sync destination.
type Destination struct {
	Name         string `yaml:"name"`
	RcloneRemote string `yaml:"rclone_remote"`
}

// DeviceConfig stores per-device settings.
type DeviceConfig struct {
	Nickname string `yaml:"nickname,omitempty"`
	WiFiIP   string `yaml:"wifi_ip,omitempty"`
}

// Config is the top-level configuration.
type Config struct {
	SyncDir      string                  `yaml:"sync_dir"`
	Destinations []Destination           `yaml:"destinations"`
	Devices      map[string]DeviceConfig `yaml:"devices,omitempty"`
	MediaPaths   []string                `yaml:"media_paths"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		SyncDir: filepath.Join(home, "FetchQuest"),
		MediaPaths: []string{
			"/sdcard/Oculus/VideoShots/",
			"/sdcard/Oculus/Screenshots/",
		},
		Devices: make(map[string]DeviceConfig),
	}
}

// ConfigDir returns the config directory path.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "fetchquest")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fetchquest")
}

// ConfigPath returns the config file path.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// Load reads the config file, returning defaults if it doesn't exist.
func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Devices == nil {
		cfg.Devices = make(map[string]DeviceConfig)
	}
	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := ConfigPath()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ExpandSyncDir expands ~ in the sync dir path.
func (c *Config) ExpandSyncDir() string {
	if len(c.SyncDir) > 0 && c.SyncDir[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, c.SyncDir[1:])
	}
	return c.SyncDir
}
