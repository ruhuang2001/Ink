package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type Box struct {
	key []byte
}

func NewBox(raw string) (*Box, error) {
	if raw == "" {
		return nil, fmt.Errorf("secret key is required")
	}

	key, err := decodeKey(raw)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secret key must decode to 32 bytes")
	}

	return &Box{key: key}, nil
}

func decodeKey(raw string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}

	return nil, fmt.Errorf("secret key must be base64 or 32 raw bytes")
}

func (b *Box) Encrypt(plaintext string) ([]byte, []byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	return gcm.Seal(nil, nonce, []byte(plaintext), nil), nonce, nil
}

func (b *Box) Decrypt(ciphertext []byte, nonce []byte) (string, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("invalid secret nonce length")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
