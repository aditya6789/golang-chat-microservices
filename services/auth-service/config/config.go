package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	JWTSecret    string
	JWTTTLMinute int
}

func Load() Config {
	ttl, _ := strconv.Atoi(getEnv("JWT_TTL_MINUTES", "60"))
	return Config{
		Port:         getEnv("AUTH_SERVICE_PORT", "8081"),
		JWTSecret:    getEnv("JWT_SECRET", "super-secret"),
		JWTTTLMinute: ttl,
	}
}

func getEnv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

