package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Encryptor handles AES-256-GCM encryption and decryption.
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor creates an Encryptor from a base64-encoded 32-byte key.
func NewEncryptor(keyBase64 string) (*Encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// LoadOrGenerateKey loads an encryption key from a file, or generates a new one
// and writes it to that file if it doesn't exist. Returns a ready-to-use Encryptor.
func LoadOrGenerateKey(keyPath string) (*Encryptor, error) {
	data, err := os.ReadFile(keyPath)
	if err == nil {
		// Key file exists — use it
		return NewEncryptor(strings.TrimSpace(string(data)))
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Generate a new 32-byte key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	keyBase64 := base64.StdEncoding.EncodeToString(key)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write key file with restrictive permissions
	if err := os.WriteFile(keyPath, []byte(keyBase64+"\n"), 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	return NewEncryptor(keyBase64)
}

// Encrypt encrypts plaintext and returns base64(nonce+ciphertext).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64(nonce+ciphertext) string.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid base64: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}
