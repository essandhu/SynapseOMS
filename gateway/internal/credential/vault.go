package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen  = 16
	nonceLen = 12
)

// KDFParams holds configurable Argon2id parameters.
type KDFParams struct {
	Time    uint32
	Memory  uint32 // in KB
	Threads uint8
	KeyLen  uint32
}

// DefaultKDFParams returns the default Argon2id parameters.
func DefaultKDFParams() KDFParams {
	return KDFParams{Time: 1, Memory: 64 * 1024, Threads: 4, KeyLen: 32}
}

// deriveKeyWithParams derives a key using configurable params.
func deriveKeyWithParams(passphrase string, salt []byte, params KDFParams) []byte {
	return argon2.IDKey([]byte(passphrase), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
}

// deriveKey derives a 256-bit encryption key from the passphrase and salt
// using Argon2id with default parameters.
func deriveKey(passphrase string, salt []byte) []byte {
	return deriveKeyWithParams(passphrase, salt, DefaultKDFParams())
}

// ZeroBytes overwrites a byte slice with zeros.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns the ciphertext (with appended GCM tag) and the nonce.
func encrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce = make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// decrypt decrypts ciphertext using AES-256-GCM with the provided nonce.
func decrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

// generateSalt creates a cryptographically random salt of the standard length.
func generateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}
	return salt, nil
}
