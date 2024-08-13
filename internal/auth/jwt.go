package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	MaxExpiresInSeconds = 60 * 60 * 24 // 1 day
)

func NewJWT(subject string, secret []byte, expiresInSeconds int) (string, error) {
	if expiresInSeconds == 0 || expiresInSeconds > MaxExpiresInSeconds {
		expiresInSeconds = MaxExpiresInSeconds
	}
	expireDuration := time.Duration(expiresInSeconds) * time.Second
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(expireDuration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})
	s, err := token.SignedString(secret)
	return s, err
}

func GetClaimsFromJWT(jwtString string, secret []byte) (*jwt.RegisteredClaims, error) {
	parser := jwt.NewParser()
	kf := func(t *jwt.Token) (interface{}, error) {
		return secret, nil
	}
	token, err := parser.ParseWithClaims(jwtString, &jwt.RegisteredClaims{}, kf)
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(*jwt.RegisteredClaims)
	return claims, nil
}
