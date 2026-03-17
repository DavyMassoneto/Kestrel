package crypto_test

import (
	"encoding/base64"
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/crypto"
)

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-empty")

	ciphertext, err := enc.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != "" {
		t.Errorf("decrypted = %q; want empty", decrypted)
	}
}

func TestEncrypt_LongPlaintext(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-long")

	// 1KB of data
	long := make([]byte, 1024)
	for i := range long {
		long[i] = byte(i % 256)
	}
	plaintext := string(long)

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt long: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Error("roundtrip failed for long plaintext")
	}
}

func TestEncrypt_OutputIsValidBase64(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-b64")

	ciphertext, err := enc.Encrypt("test-data")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Errorf("ciphertext is not valid base64: %v", err)
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-tamper")

	ciphertext, err := enc.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Decode, tamper, re-encode
	data, _ := base64.StdEncoding.DecodeString(ciphertext)
	if len(data) > 13 {
		data[13] ^= 0xFF // flip bits in ciphertext portion
	}
	tampered := base64.StdEncoding.EncodeToString(data)

	_, err = enc.Decrypt(tampered)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}
