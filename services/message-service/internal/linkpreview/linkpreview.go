package linkpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	htext "html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Preview is Open Graph / fallbacks for a URL.
type Preview struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

type entry struct {
	p     Preview
	expAt time.Time
}

// Service fetches and caches link metadata (rate limiting is enforced in the HTTP handler).
type Service struct {
	mu      sync.Mutex
	cache   map[string]entry
	maxEnt  int
	ttl     time.Duration
	client  *http.Client
	maxBody int64
}

// NewService builds a preview fetcher with in-memory TTL cache.
func NewService(ttl time.Duration, maxCacheEntries int) *Service {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if maxCacheEntries <= 0 {
		maxCacheEntries = 2000
	}
	return &Service{
		cache:  make(map[string]entry),
		maxEnt: maxCacheEntries,
		ttl:    ttl,
		client: &http.Client{
			Timeout: 12 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 4 {
					return errors.New("too many redirects")
				}
				return safeRequestURL(req.URL)
			},
		},
		maxBody: 512 * 1024,
	}
}

func cacheKey(raw string) string {
	u, err := normalizeURL(raw)
	if err != nil {
		return ""
	}
	h := sha256.Sum256([]byte(u.String()))
	return hex.EncodeToString(h[:])
}

func normalizeURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 2048 {
		return nil, errors.New("url too long")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http(s) URLs")
	}
	if u.Host == "" {
		return nil, errors.New("missing host")
	}
	u.Fragment = ""
	return u, nil
}

// safeRequestURL blocks SSRF to private networks and loopback.
func safeRequestURL(u *url.URL) error {
	host := u.Hostname()
	if host == "" {
		return errors.New("invalid host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return errors.New("host not allowed")
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return errors.New("host could not be resolved")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return errors.New("host resolves to a non-public address")
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
		return false
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	// IPv4-mapped IPv6 etc.
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 || ip4[0] == 127 {
			return false
		}
	}
	return true
}

// Fetch returns OG metadata for a public http(s) URL.
func (s *Service) Fetch(ctx context.Context, raw string) (Preview, error) {
	var zero Preview
	u, err := normalizeURL(raw)
	if err != nil {
		return zero, err
	}
	if err := safeRequestURL(u); err != nil {
		return zero, err
	}
	key := cacheKey(raw)
	if key == "" {
		return zero, errors.New("bad url")
	}

	s.mu.Lock()
	if e, ok := s.cache[key]; ok && time.Now().Before(e.expAt) {
		out := e.p
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("User-Agent", "OrbitChat-LinkPreview/1.0 (+https://github.com/)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return zero, fmt.Errorf("http %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "html") && !strings.Contains(strings.ToLower(ct), "text/") {
		return zero, errors.New("not an HTML page")
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBody))
	if err != nil {
		return zero, err
	}
	p := parseHTML(body, u)
	p.URL = u.String()
	if p.Title == "" && p.Description == "" && p.Image == "" {
		return zero, errors.New("no preview metadata")
	}

	s.mu.Lock()
	if len(s.cache) >= s.maxEnt {
		s.evictExpiredLocked()
		if len(s.cache) >= s.maxEnt {
			for k := range s.cache {
				delete(s.cache, k)
				break
			}
		}
	}
	s.cache[key] = entry{p: p, expAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()

	return p, nil
}

func (s *Service) evictExpiredLocked() {
	now := time.Now()
	for k, e := range s.cache {
		if now.After(e.expAt) {
			delete(s.cache, k)
		}
	}
}

func parseHTML(body []byte, page *url.URL) Preview {
	var p Preview
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return p
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "meta":
				var prop, name, itemprop, content string
				for _, a := range n.Attr {
					switch a.Key {
					case "property":
						prop = a.Val
					case "name":
						name = a.Val
					case "itemprop":
						itemprop = a.Val
					case "content":
						content = a.Val
					}
				}
				content = htext.UnescapeString(strings.TrimSpace(content))
				if content == "" {
					break
				}
				switch prop {
				case "og:title":
					p.Title = content
				case "og:description":
					p.Description = content
				case "og:image":
					p.Image = absolutize(page, content)
				case "og:site_name":
					p.SiteName = content
				case "og:url":
					if u, err := url.Parse(content); err == nil {
						if nu, err := page.Parse(u.String()); err == nil {
							p.URL = nu.String()
						}
					}
				}
				if p.Title == "" && (name == "twitter:title" || itemprop == "name") {
					p.Title = content
				}
				if p.Description == "" && (name == "twitter:description" || itemprop == "description") {
					p.Description = content
				}
				if p.Image == "" && name == "twitter:image" {
					p.Image = absolutize(page, content)
				}
				if p.Description == "" && name == "description" {
					p.Description = content
				}
			case "title":
				if p.Title == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					p.Title = htext.UnescapeString(strings.TrimSpace(n.FirstChild.Data))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return p
}

func absolutize(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}
