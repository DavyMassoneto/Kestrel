package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// AESEncryptor implements AES-256-GCM encryption/decryption.
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor derives a 256-bit key from masterKey via HKDF (SHA-256).
func NewAESEncryptor(masterKey string) (*AESEncryptor, error) {
	if masterKey == "" {
		return nil, fmt.Errorf("master key cannot be empty")
	}

	hkdfReader := hkdf.New(sha256.New, []byte(masterKey), nil, []byte("kestrel-aes-256-gcm"))
	key := make([]byte, 32) // 256 bits
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	return &AESEncryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM. Output is base64(nonce + ciphertext).
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext (nonce + ciphertext) using AES-256-GCM.
func (e *AESEncryptor) Decrypt(ciphertextB64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
