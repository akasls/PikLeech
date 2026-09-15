package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

var (
	ErrDecryptionFailed = errors.New("decryption failed: invalid ciphertext or secret")
)

const LegacyDefaultSecret = "pikpak-default-secret-key-32-chars!!"

// DecryptWithFallback tries decrypting with primary secret; if it fails and primary secret is not legacy, tries legacy secret.
func DecryptWithFallback(ciphertextB64, primarySecret string) (string, error) {
	if ciphertextB64 == "" {
		return "", nil
	}
	res, err := Decrypt(ciphertextB64, primarySecret)
	if err == nil {
		return res, nil
	}
	if primarySecret != LegacyDefaultSecret {
		if fallbackRes, fallbackErr := Decrypt(ciphertextB64, LegacyDefaultSecret); fallbackErr == nil {
			return fallbackRes, nil
		}
	}
	return "", err
}

// DeriveKey derives a 32-byte AES key from any secret string using SHA-256.
func DeriveKey(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// Encrypt encrypts plaintext with AES-GCM using the provided secret.
// Returns base64 encoded string containing nonce + ciphertext.
func Encrypt(plaintext, secret string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := DeriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded AES-GCM ciphertext using the provided secret.
func Decrypt(ciphertextB64, secret string) (string, error) {
	if ciphertextB64 == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}

	key := DeriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecryptionFailed
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// MaskSecret masks sensitive credentials for logging (e.g. "abc***xyz").
func MaskSecret(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	if len(s) <= 8 {
		return s[:2] + "****" + s[len(s)-2:]
	}
	prefix := s[:3]
	suffix := s[len(s)-3:]
	return prefix + "***" + suffix
}

// MaskMagnet masks magnet link infohash for logging.
func MaskMagnet(url string) string {
	if !strings.HasPrefix(url, "magnet:") {
		if len(url) > 20 {
			return url[:15] + "..."
		}
		return url
	}
	idx := strings.Index(url, "xt=urn:btih:")
	if idx == -1 {
		return "magnet:?xt=***"
	}
	hashPart := url[idx+12:]
	amp := strings.Index(hashPart, "&")
	if amp != -1 {
		hashPart = hashPart[:amp]
	}
	if len(hashPart) > 8 {
		return "magnet:?xt=urn:btih:" + hashPart[:4] + "***" + hashPart[len(hashPart)-4:]
	}
	return "magnet:?xt=urn:btih:***"
}
