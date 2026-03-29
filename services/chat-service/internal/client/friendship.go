package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type FriendshipClient struct {
	BaseURL string
	HTTP    *http.Client
}

func (f *FriendshipClient) AreFriends(ctx context.Context, a, b string) bool {
	if f == nil || f.BaseURL == "" {
		return false
	}
	client := f.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	base := strings.TrimSuffix(f.BaseURL, "/")
	u := fmt.Sprintf("%s/internal/friendship?user_id=%s&peer_id=%s",
		base, url.QueryEscape(a), url.QueryEscape(b))
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
		Friends bool `json:"friends"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Friends
}
