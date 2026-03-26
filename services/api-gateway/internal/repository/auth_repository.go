package repository

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"realtime-chat-system/services/api-gateway/internal/model"
)

type AuthRepository struct {
	validateURL string
	client      *http.Client
}

func NewAuthRepository() *AuthRepository {
	return &AuthRepository{
		validateURL: "http://auth-service:8081/auth/validate",
		client:      &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *AuthRepository) Validate(authHeader string) (*model.Claims, error) {
	req, _ := http.NewRequest(http.MethodGet, r.validateURL, nil)
	req.Header.Set("Authorization", authHeader)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unauthorized")
	}
	var c model.Claims
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return nil, err
	}
	c.Sub = strings.TrimSpace(c.Sub)
	return &c, nil
}

