package chatgptauth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const pkceRandomBytes = 32

func newPKCE(rand io.Reader) (verifier string, challenge string, err error) {
	verifier, err = randomURLSafe(rand, pkceRandomBytes)
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(hash[:]), nil
}

func randomURLSafe(rand io.Reader, size int) (string, error) {
	if rand == nil {
		return "", fmt.Errorf("random source unavailable")
	}
	b := make([]byte, size)
	if _, err := io.ReadFull(rand, b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
