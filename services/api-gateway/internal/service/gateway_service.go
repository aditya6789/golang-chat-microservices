package service

import (
	"net/http/httputil"
	"net/url"

	"realtime-chat-system/services/api-gateway/internal/repository"
)

type GatewayService struct {
	auth *repository.AuthRepository
}

func New(auth *repository.AuthRepository) *GatewayService { return &GatewayService{auth: auth} }
func (s *GatewayService) Validate(authHeader string) error {
	_, err := s.auth.Validate(authHeader)
	return err
}

func (s *GatewayService) Proxy(target string) *httputil.ReverseProxy {
	u, _ := url.Parse(target)
	return httputil.NewSingleHostReverseProxy(u)
}

