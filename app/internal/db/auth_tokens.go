package db

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func randomURLToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
