package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port string
	S3   S3Config
}

type S3Config struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	Bucket       string
	PublicBase   string
	UsePathStyle bool
	MaxBytes     int64
	PresignTTL   time.Duration
}

func Load() Config {
	maxB := int64(10 << 20)
	if v := env("S3_MAX_UPLOAD_BYTES", ""); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxB = n
		}
	}
	ttl := 15 * time.Minute
	if v := env("S3_PRESIGN_TTL_SECONDS", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = time.Duration(n) * time.Second
		}
	}
	pathStyle := env("S3_USE_PATH_STYLE", "true") != "false"
	return Config{
		Port: env("MESSAGE_SERVICE_PORT", "8084"),
		S3: S3Config{
			Endpoint:     env("S3_ENDPOINT", ""),
			Region:       env("S3_REGION", "us-east-1"),
			AccessKey:    env("S3_ACCESS_KEY", ""),
			SecretKey:    env("S3_SECRET_KEY", ""),
			Bucket:       env("S3_BUCKET", ""),
			PublicBase:   env("S3_PUBLIC_BASE_URL", ""),
			UsePathStyle: pathStyle,
			MaxBytes:     maxB,
			PresignTTL:   ttl,
		},
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}


