package auth

import (
	"os"
	"path/filepath"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/encryption"
)

// ConfigSessionKeyProvider implements SessionKeyProvider using AppConfig and file-based key persistence.
type ConfigSessionKeyProvider struct {
	config            ccc.AppConfig
	encryptionService encryption.EncryptionService
}

// NewConfigSessionKeyProvider creates a new ConfigSessionKeyProvider.
func NewConfigSessionKeyProvider(config ccc.AppConfig, encryptionService encryption.EncryptionService) *ConfigSessionKeyProvider {
	return &ConfigSessionKeyProvider{
		config:            config,
		encryptionService: encryptionService,
	}
}

// GetSigningKey retrieves the signing key, creating and persisting it if necessary.
func (p *ConfigSessionKeyProvider) GetSigningKey() ([]byte, error) {
	keyString, err := p.getOrCreateKey(p.config.SigningKey, "signing_key", p.config.KeyDir)
	if err != nil {
		return nil, err
	}
	return p.encryptionService.ConvertStringToKey(keyString)
}

// GetEncryptionKey retrieves the encryption key, creating and persisting it if necessary.
func (p *ConfigSessionKeyProvider) GetEncryptionKey() ([]byte, error) {
	keyString, err := p.getOrCreateKey(p.config.EncryptionKey, "encryption_key", p.config.KeyDir)
	if err != nil {
		return nil, err
	}
	return p.encryptionService.ConvertStringToKey(keyString)
}

// getOrCreateKey reads a key from config, or creates a new one if it doesn't exist.
// If a key doesn't exist, it generates a new one and persists it to a secure file.
func (p *ConfigSessionKeyProvider) getOrCreateKey(configKey, keyFileName, customKeyDir string) (string, error) {
	// Check if the config already has the key
	if configKey != "" {
		return configKey, nil
	}

	// Try to load from secure file
	keyFilePath := getKeyFilePath(keyFileName, customKeyDir)
	if keyBytes, err := os.ReadFile(keyFilePath); err == nil {
		key := string(keyBytes)
		if key != "" {
			return key, nil
		}
	}

	// Generate a new secure key
	key, err := p.encryptionService.GenerateKey()
	if err != nil {
		return "", ccc.NewInternalError("failed to generate key", err)
	}

	// Persist key to secure file
	if err := persistKeyToFile(keyFilePath, key); err != nil {
		return "", ccc.NewInternalError("failed to persist key", err)
	}

	return key, nil
}

// getKeyFilePath returns the secure file path for storing a key using OS-specific user data directories
func getKeyFilePath(keyFileName, customKeyDir string) string {
	keyDir := customKeyDir
	if keyDir == "" {
		keyDir = filepath.Join(ccc.GetUserDataDir(), "keys")
	}

	// Ensure directory exists
	// Use 0700 to ensure only the owner can access the directory.
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		// Fallback to a local directory if user-specific dir fails
		// This is not ideal for security but provides a fallback.
		keyDir = "./keys"
		os.MkdirAll(keyDir, 0700)
	}
	return filepath.Join(keyDir, keyFileName+".key")
}

// persistKeyToFile saves a key to a secure file with restricted permissions
func persistKeyToFile(filePath, key string) error {
	// Ensure the directory for the key file exists.
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Write with restricted permissions (only owner can read/write)
	err := os.WriteFile(filePath, []byte(key), 0600)
	if err != nil {
		return err
	}
	return nil
}
