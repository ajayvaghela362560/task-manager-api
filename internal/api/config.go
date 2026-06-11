package api

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   []byte
	TokenTTL    time.Duration
	CORSOrigins []string
	UploadDir   string
	AdminEmails map[string]bool
	MaxUploadMB int64
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func LoadConfig() Config {
	cfg := Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   []byte(os.Getenv("JWT_SECRET")),
		UploadDir:   getenv("UPLOAD_DIR", "./uploads"),
		AdminEmails: map[string]bool{},
		MaxUploadMB: 5,
	}

	ttl, err := time.ParseDuration(getenv("JWT_TTL", "168h"))
	if err != nil {
		log.Fatalf("invalid JWT_TTL: %v", err)
	}
	cfg.TokenTTL = ttl

	for _, o := range strings.Split(getenv("CORS_ORIGIN", "http://localhost:3000"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}

	// Accounts that sign up with one of these emails get the admin role.
	for _, e := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			cfg.AdminEmails[e] = true
		}
	}
	return cfg
}
