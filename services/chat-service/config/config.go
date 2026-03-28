package config

import "os"

type Config struct {
	Port              string
	JWTSecret         string
	MessageServiceURL string
}

func Load() Config {
	return Config{
		Port:              env("CHAT_SERVICE_PORT", "8083"),
		JWTSecret:         env("JWT_SECRET", "super-secret"),
		MessageServiceURL: env("MESSAGE_SERVICE_URL", "http://localhost:8084"),
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

