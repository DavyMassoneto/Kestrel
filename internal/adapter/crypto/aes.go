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
	gcm cipher.AEAD
}

// NewAESEncryptor derives a 256-bit key from masterKey via HKDF (SHA-256)
// and pre-builds the AES-GCM cipher. The cipher is reusable and goroutine-safe.
func NewAESEncryptor(masterKey string) (*AESEncryptor, error) {
	if masterKey == "" {
		return nil, fmt.Errorf("master key cannot be empty")
	}

	key := deriveKey(masterKey)

	// aes.NewCipher only fails for key lengths != 16/24/32; deriveKey always returns 32.
	block, _ := aes.NewCipher(key)
	// cipher.NewGCM only fails for non-standard block sizes; AES is always 16.
	gcm, _ := cipher.NewGCM(block)

	return &AESEncryptor{gcm: gcm}, nil
}

// deriveKey produces a 256-bit key from masterKey via HKDF-SHA256.
func deriveKey(masterKey string) []byte {
	r := hkdf.New(sha256.New, []byte(masterKey), nil, []byte("kestrel-aes-256-gcm"))
	key := make([]byte, 32)
	// io.ReadFull from HKDF with SHA-256 always produces 32 bytes for a non-empty input.
	io.ReadFull(r, key)
	return key
}

// Encrypt encrypts plaintext using AES-256-GCM. Output is base64(nonce + ciphertext).
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	// crypto/rand.Reader never fails on supported platforms (Go 1.22+).
	io.ReadFull(rand.Reader, nonce)

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext (nonce + ciphertext) using AES-256-GCM.
func (e *AESEncryptor) Decrypt(ciphertextB64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
