package service

import (
	"context"
	"errors"
	"time"

	"realtime-chat-system/services/auth-service/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      *repository.UserRepository
	jwtSecret []byte
	jwtTTL    time.Duration
}

func NewAuthService(repo *repository.UserRepository, jwtSecret string, ttlMin int) *AuthService {
	return &AuthService{repo: repo, jwtSecret: []byte(jwtSecret), jwtTTL: time.Duration(ttlMin) * time.Minute}
}

func (s *AuthService) Register(ctx context.Context, email, username, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	user, err := s.repo.Create(ctx, email, username, string(hash))
	if err != nil {
		return "", err
	}
	return s.issueToken(user.ID), nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	return s.issueToken(user.ID), nil
}

func (s *AuthService) Validate(tokenStr string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

func (s *AuthService) issueToken(subject string) string {
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtTTL)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	out, _ := t.SignedString(s.jwtSecret)
	return out
}

