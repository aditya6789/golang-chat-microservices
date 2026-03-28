package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port             string
	JWTSecret        string
	JWTTTLMinute     int
	RefreshTTLDays   int
}

func Load() Config {
	ttl, _ := strconv.Atoi(getEnv("JWT_TTL_MINUTES", "60"))
	refDays, _ := strconv.Atoi(getEnv("JWT_REFRESH_TTL_DAYS", "30"))
	return Config{
		Port:           getEnv("AUTH_SERVICE_PORT", "8081"),
		JWTSecret:      getEnv("JWT_SECRET", "super-secret"),
		JWTTTLMinute:   ttl,
		RefreshTTLDays: refDays,
	}
}

func getEnv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

