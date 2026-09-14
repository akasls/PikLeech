package crypto

import (
	"testing"
)

func TestCrypto_EncryptDecrypt(t *testing.T) {
	secret := "my-very-strong-app-secret-12345"
	original := "pikpak_password_token_secret_data"

	encrypted, err := Encrypt(original, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == original {
		t.Fatalf("Encrypted text should not match original")
	}

	decrypted, err := Decrypt(encrypted, secret)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != original {
		t.Fatalf("Expected %s, got %s", original, decrypted)
	}

	// Test with wrong secret
	_, err = Decrypt(encrypted, "wrong-secret")
	if err == nil {
		t.Fatalf("Expected decryption error with wrong secret")
	}
}

func TestCrypto_MaskSecret(t *testing.T) {
	masked := MaskSecret("1234567890abcdef")
	if masked != "123***def" {
		t.Errorf("Expected 123***def, got %s", masked)
	}

	shortMask := MaskSecret("abcd")
	if shortMask != "****" {
		t.Errorf("Expected ****, got %s", shortMask)
	}
}
