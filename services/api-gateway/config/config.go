package config

import "os"

type Config struct{ Port string }

func Load() Config { return Config{Port: env("API_GATEWAY_PORT", "8080")} }
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

