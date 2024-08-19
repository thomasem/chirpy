package auth

import (
	"golang.org/x/crypto/bcrypt"
)

func PasswordStringToHash(password string) (string, error) {
	hBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hBytes), nil
}

func PasswordMatches(password string, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
