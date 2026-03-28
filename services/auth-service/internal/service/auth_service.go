package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"realtime-chat-system/services/auth-service/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo         *repository.UserRepository
	refreshRepo  *repository.RefreshRepository
	jwtSecret    []byte
	jwtTTL       time.Duration
	refreshTTL   time.Duration
}

func NewAuthService(repo *repository.UserRepository, refreshRepo *repository.RefreshRepository, jwtSecret string, ttlMin, refreshDays int) *AuthService {
	return &AuthService{
		repo:        repo,
		refreshRepo: refreshRepo,
		jwtSecret:   []byte(jwtSecret),
		jwtTTL:      time.Duration(ttlMin) * time.Minute,
		refreshTTL:  time.Duration(refreshDays) * 24 * time.Hour,
	}
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func (s *AuthService) Register(ctx context.Context, email, username, password string) (*TokenPair, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.Create(ctx, email, username, string(hash))
	if err != nil {
		return nil, err
	}
	return s.issuePair(ctx, user.ID)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return s.issuePair(ctx, user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, errors.New("missing refresh token")
	}
	h := hashRefresh(refreshToken)
	userID, _, err := s.refreshRepo.UserIDByHash(ctx, h)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	_ = s.refreshRepo.DeleteByHash(ctx, h)
	return s.issuePair(ctx, userID)
}

func (s *AuthService) issuePair(ctx context.Context, userID string) (*TokenPair, error) {
	access := s.issueToken(userID)
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	refresh := hex.EncodeToString(raw)
	if err := s.refreshRepo.Save(ctx, userID, hashRefresh(refresh), time.Now().Add(s.refreshTTL)); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func hashRefresh(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
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

