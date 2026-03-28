package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type MembershipClient struct {
	BaseURL string
	HTTP    *http.Client
}

func (m *MembershipClient) IsMember(ctx context.Context, chatID, userID string) bool {
	if m == nil || m.BaseURL == "" {
		return false
	}
	client := m.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	base := strings.TrimSuffix(m.BaseURL, "/")
	u := fmt.Sprintf("%s/internal/chats/%s/membership?user_id=%s",
		base, url.PathEscape(chatID), url.QueryEscape(userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var out struct {
		Member bool `json:"member"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Member
}
