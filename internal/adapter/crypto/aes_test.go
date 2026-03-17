package crypto_test

import (
	"testing"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/crypto"
)

func TestNewAESEncryptor(t *testing.T) {
	_, err := crypto.NewAESEncryptor("my-secret-master-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAESEncryptor_EmptyKey(t *testing.T) {
	_, err := crypto.NewAESEncryptor("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, err := crypto.NewAESEncryptor("test-master-key-roundtrip")
	if err != nil {
		t.Fatalf("NewAESEncryptor: %v", err)
	}

	plaintext := "sk-ant-api03-secret-key-value-1234567890"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("ciphertext should not equal plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted = %q; want %q", decrypted, plaintext)
	}
}

func TestEncrypt_DifferentNonces(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-nonces")
	plaintext := "same-value"

	ct1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	ct2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if ct1 == ct2 {
		t.Error("same plaintext should produce different ciphertexts (random nonce)")
	}
}

func TestDecrypt_CorruptedData(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-corrupt")

	_, err := enc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for corrupted base64")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	enc, _ := crypto.NewAESEncryptor("test-key-short")

	// Valid base64 but too short for nonce + ciphertext
	_, err := enc.Decrypt("AQID")
	if err == nil {
		t.Error("expected error for data too short")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, _ := crypto.NewAESEncryptor("key-one")
	enc2, _ := crypto.NewAESEncryptor("key-two")

	ciphertext, err := enc1.Encrypt("secret-data")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}
