// Package config provides loading configuration from YAML into typed structs.
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct for the server.
// You can extend it with database, logging, and other sections.
type Config struct {
	Server ServerConfig `yaml:"server"`
	Store  StoreConfig  `yaml:"store"`
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

// StoreConfig holds in-memory store settings.
type StoreConfig struct {
	MaxItems int `yaml:"max_items"`
}

// LoadConfig loads YAML configuration from the given file path.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
