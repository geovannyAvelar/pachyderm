package postgres

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPort is the port a server starts on until the user configures a
// different one for that version.
const DefaultPort = 5432

// minUserPort is the lowest port SetPort accepts. Ports below 1024 need root
// on Linux, which conflicts with pachyderm never requiring elevated
// privileges, so they're rejected rather than left to fail at startup.
const minUserPort = 1024

type config struct {
	// Ports maps a PostgreSQL version to the port its server starts on.
	Ports map[string]int `json:"ports"`
}

func configFile() (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.json"), nil
}

func loadConfig() (config, error) {
	path, err := configFile()
	if err != nil {
		return config{}, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config{}, nil
	}
	if err != nil {
		return config{}, err
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

// GetPort returns the configured port for a version, or DefaultPort if none
// has been set yet.
func GetPort(version string) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	if port, ok := cfg.Ports[version]; ok {
		return port, nil
	}
	return DefaultPort, nil
}

// SetPort persists the port a version's server should start on from now on.
// It doesn't affect a server that's already running until it's restarted.
func SetPort(version string, port int) error {
	if port < minUserPort || port > 65535 {
		return fmt.Errorf("port must be between %d and 65535", minUserPort)
	}

	home, err := HomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg.Ports == nil {
		cfg.Ports = make(map[string]int)
	}
	cfg.Ports[version] = port

	path, err := configFile()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
