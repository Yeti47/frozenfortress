package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// Constants for encryption parameters
const (
	iterationCount = 10000 // PBKDF2 iterations
	keyLength      = 32    // 256 bits for AES-256
)

// DefaultEncryptionService provides encryption, decryption, and hashing capabilities
type DefaultEncryptionService struct {
	// No fields needed since we're using constants
}

// NewDefaultEncryptionService creates a new instance of DefaultEncryptionService
func NewDefaultEncryptionService() *DefaultEncryptionService {
	return &DefaultEncryptionService{}
}

// Hash implements the Hasher interface by creating a hash from input string
func (s *DefaultEncryptionService) Hash(input string) (output string, salt string, err error) {
	// Generate a random salt
	saltBytes, saltString, err := s.GenerateSalt()

	if err != nil {
		return "", "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Generate hash using PBKDF2
	hash := pbkdf2.Key([]byte(input), saltBytes, iterationCount, keyLength, sha256.New)

	hashString := hex.EncodeToString(hash)

	return hashString, saltString, nil
}

// VerifyHash verifies if the input string matches the hash
func (s *DefaultEncryptionService) VerifyHash(input string, hash string, salt string) (isValid bool, err error) {
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return false, fmt.Errorf("invalid salt: %w", err)
	}

	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return false, fmt.Errorf("invalid hash: %w", err)
	}

	// Generate hash using PBKDF2
	generatedHash := pbkdf2.Key([]byte(input), saltBytes, iterationCount, keyLength, sha256.New)

	return subtle.ConstantTimeCompare(generatedHash, hashBytes) == 1, nil
}

// Encrypt encrypts plaintext using the provided key
func (s *DefaultEncryptionService) Encrypt(plainText string, key string) (cipherText string, err error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil || len(keyBytes) != keyLength {
		return "", errors.New("invalid encryption key")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create a new GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create a nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and seal
	sealed := gcm.Seal(nonce, nonce, []byte(plainText), nil)

	// Encode to hex for storage
	return hex.EncodeToString(sealed), nil
}

// Decrypt decrypts ciphertext using the provided key
func (s *DefaultEncryptionService) Decrypt(cipherText string, key string) (plainText string, err error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil || len(keyBytes) != keyLength {
		return "", errors.New("invalid encryption key")
	}

	cipherBytes, err := hex.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("invalid cipher text: %w", err)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create a new GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimal length
	if len(cipherBytes) < gcm.NonceSize() {
		return "", errors.New("cipher text too short")
	}

	// Extract nonce and ciphertext
	nonce := cipherBytes[:gcm.NonceSize()]
	sealedData := cipherBytes[gcm.NonceSize():]

	// Decrypt
	decrypted, err := gcm.Open(nil, nonce, sealedData, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(decrypted), nil
}

// GenerateKey generates a new random encryption key
func (s *DefaultEncryptionService) GenerateKey() (key string, err error) {

	keyBytes := make([]byte, keyLength) // 32 bytes for AES-256
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

	return hex.EncodeToString(keyBytes), nil
}

// GenerateSalt generates a new random salt
func (s *DefaultEncryptionService) GenerateSalt() (saltBytes []byte, salt string, err error) {

	saltBytes = make([]byte, 16) // 128 bits
	if _, err := io.ReadFull(rand.Reader, saltBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate salt: %w", err)
	}

	return saltBytes, hex.EncodeToString(saltBytes), nil
}

// GenerateRandomBytes generates random bytes of the specified length
func (s *DefaultEncryptionService) GenerateRandomBytes(length int) (randomBytes []byte, err error) {
	if length <= 0 {
		return nil, errors.New("length must be positive")
	}

	randomBytes = make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return randomBytes, nil
}

// GenerateKeyFromPassword generates a key from a password using PBKDF2
func (s *DefaultEncryptionService) GenerateKeyFromPassword(password string, salt string) (key string, err error) {
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		return "", fmt.Errorf("invalid salt: %w", err)
	}

	keyBytes := pbkdf2.Key([]byte(password), saltBytes, iterationCount, keyLength, sha256.New)

	return hex.EncodeToString(keyBytes), nil
}

// ConvertKeyToString converts a byte array key to a hex string
func (s *DefaultEncryptionService) ConvertKeyToString(key []byte) (keyString string, err error) {
	if len(key) != keyLength {
		return "", errors.New("invalid key length")
	}

	return hex.EncodeToString(key), nil
}

// ConvertStringToKey converts a hex string key to a byte array
func (s *DefaultEncryptionService) ConvertStringToKey(keyString string) (key []byte, err error) {
	key, err = hex.DecodeString(keyString)
	if err != nil {
		return nil, fmt.Errorf("invalid key string: %w", err)
	}

	if len(key) != keyLength {
		return nil, errors.New("invalid key length")
	}

	return key, nil
}
