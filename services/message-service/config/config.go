package config

import "os"

type Config struct{ Port string }

func Load() Config { return Config{Port: env("MESSAGE_SERVICE_PORT", "8084")} }
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

