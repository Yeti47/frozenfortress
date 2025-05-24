package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all configuration values for the CLI application
type Config struct {
	// Database configuration
	DatabasePath string `json:"databasePath"`

	// Verbose output configuration
	Verbose bool `json:"verbose"`
}

// LoadFromFile loads configuration from a JSON file
// If the file doesn't exist or can't be read, returns default configuration
func LoadFromFile(configPath string) (*Config, error) {
	// Default configuration
	config := &Config{
		DatabasePath: "./frozenfortress.db",
		Verbose:      false,
	}

	// If no config path provided, use default config file
	if configPath == "" {
		configPath = "./config.json"
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, create it with default values
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal default config: %w", err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to create config file %s: %w", configPath, err)
		}

		return config, nil
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DatabasePath == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	return nil
}

// String returns a string representation of the config (excluding sensitive data)
func (c *Config) String() string {
	return fmt.Sprintf("Config{DatabasePath: %s, Verbose: %t}",
		c.DatabasePath, c.Verbose)
}
