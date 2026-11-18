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
	// Ports maps a PostgreSQL major version (see MajorVersion) to the port
	// its server starts on. Every release of a major version shares one
	// data directory and so one server, and so one port.
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

func saveConfig(cfg config) error {
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

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	// Write to a temporary file and rename it into place, so a crash
	// mid-write can't leave a truncated config.json behind.
	tmp, err := os.CreateTemp(home, ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// GetPort returns the configured port for a version's major version, or
// DefaultPort if none has been set yet.
func GetPort(version string) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		return 0, err
	}
	if port, ok := cfg.Ports[MajorVersion(version)]; ok {
		return port, nil
	}
	// Older Pachyderm releases keyed ports by release, not major version.
	if port, ok := cfg.Ports[version]; ok {
		return port, nil
	}
	return DefaultPort, nil
}

// migrateLegacyPort moves a port that an older Pachyderm release configured
// for one release (e.g. "16.14.0") to that release's major version, unless
// the major version already has a port of its own.
func migrateLegacyPort(release, major string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	port, ok := cfg.Ports[release]
	if !ok {
		return nil
	}
	if _, exists := cfg.Ports[major]; !exists {
		cfg.Ports[major] = port
	}
	delete(cfg.Ports, release)
	return saveConfig(cfg)
}

// SetPort persists the port a version's server should start on from now on.
// The port applies to every release of the version's major version.
// It doesn't affect a server that's already running until it's restarted.
func SetPort(version string, port int) error {
	if port < minUserPort || port > 65535 {
		return fmt.Errorf("port must be between %d and 65535", minUserPort)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg.Ports == nil {
		cfg.Ports = make(map[string]int)
	}
	cfg.Ports[MajorVersion(version)] = port
	return saveConfig(cfg)
}
