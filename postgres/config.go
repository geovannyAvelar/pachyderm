package postgres

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPort is the port a server starts on until the user configures a
// different one.
const DefaultPort = 5432

// minUserPort is the lowest port SetPort accepts. Ports below 1024 need root
// on Linux, which conflicts with pachyderm never requiring elevated
// privileges, so they're rejected rather than left to fail at startup.
const minUserPort = 1024

type config struct {
	Port int `json:"port"`
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

// GetPort returns the configured server port, or DefaultPort if none has
// been set yet.
func GetPort() (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	if cfg.Port == 0 {
		return DefaultPort, nil
	}
	return cfg.Port, nil
}

// SetPort persists the port servers should start on from now on. It doesn't
// affect a server that's already running until it's restarted.
func SetPort(port int) error {
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

	path, err := configFile()
	if err != nil {
		return err
	}

	data, err := json.Marshal(config{Port: port})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
