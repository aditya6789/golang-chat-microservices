package authjwt

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func SubjectFromRequest(secret []byte, authorizationHeader, queryAccessToken string) (string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(authorizationHeader, "Bearer "))
	if raw == "" {
		raw = strings.TrimSpace(queryAccessToken)
	}
	if raw == "" {
		return "", errors.New("missing token")
	}
	token, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return "", errors.New("invalid token")
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", errors.New("invalid claims")
	}
	return claims.Subject, nil
}
