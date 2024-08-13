package auth

import (
	"crypto/rand"
	"encoding/hex"
)

func NewRefreshToken() (string, error) {
	t := make([]byte, 32)
	_, err := rand.Read(t)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(t), nil
}
