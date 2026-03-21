package handlers

import (
	"strings"
	"testing"
)

// TestEncryptDecryptRoundtrip verifies that encrypting then decrypting returns the original plaintext.
// Validates: Requirements 8.3
func TestEncryptDecryptRoundtrip(t *testing.T) {
	passphrase := "test-aes-key-32-bytes-long-passphrase"
	plaintext := `{"access_key_id":"AKIAIOSFODNN7EXAMPLE","secret_access_key":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}`

	ciphertext, err := encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("ciphertext should not equal plaintext")
	}

	decrypted, err := decrypt(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

// TestEncryptProducesUniqueOutputs verifies that encrypting the same plaintext twice produces different ciphertexts (random nonce).
func TestEncryptProducesUniqueOutputs(t *testing.T) {
	passphrase := "my-secret-key"
	plaintext := "same plaintext"

	ct1, err := encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}
	ct2, err := encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	if ct1 == ct2 {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts due to random nonce")
	}
}

// TestDecryptWithWrongKeyFails verifies that decryption with the wrong key returns an error.
func TestDecryptWithWrongKeyFails(t *testing.T) {
	ciphertext, err := encrypt("secret data", "correct-key")
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = decrypt(ciphertext, "wrong-key")
	if err == nil {
		t.Error("expected decryption with wrong key to fail")
	}
}

// TestDecryptInvalidBase64Fails verifies that passing non-base64 data returns an error.
func TestDecryptInvalidBase64Fails(t *testing.T) {
	_, err := decrypt("not-valid-base64!!!", "any-key")
	if err == nil {
		t.Error("expected error for invalid base64 input")
	}
}

// TestDecryptTooShortCiphertextFails verifies that a ciphertext shorter than the nonce size returns an error.
func TestDecryptTooShortCiphertextFails(t *testing.T) {
	import64 := "dGVzdA==" // base64("test") - too short for nonce
	_, err := decrypt(import64, "any-key")
	if err == nil {
		t.Error("expected error for ciphertext shorter than nonce size")
	}
}

// TestEncryptEmptyString verifies that empty string can be encrypted and decrypted.
func TestEncryptEmptyString(t *testing.T) {
	passphrase := "key"
	ciphertext, err := encrypt("", passphrase)
	if err != nil {
		t.Fatalf("encrypt empty string failed: %v", err)
	}
	decrypted, err := decrypt(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("decrypt empty string failed: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

// TestEncryptLargePayload verifies encryption works for large credential payloads.
func TestEncryptLargePayload(t *testing.T) {
	passphrase := "large-payload-key"
	plaintext := strings.Repeat("a", 10000)

	ciphertext, err := encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encrypt large payload failed: %v", err)
	}
	decrypted, err := decrypt(ciphertext, passphrase)
	if err != nil {
		t.Fatalf("decrypt large payload failed: %v", err)
	}
	if decrypted != plaintext {
		t.Error("large payload round-trip failed")
	}
}
