package service

import "testing"

func TestIssueToken(t *testing.T) {
	svc := &AuthService{jwtSecret: []byte("secret")}
	token := svc.issueToken("user-1")
	if token == "" {
		t.Fatal("expected token, got empty string")
	}
}

