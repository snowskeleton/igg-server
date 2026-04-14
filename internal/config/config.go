package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	JWTSecret   string
	BaseURL     string
	Port        string

	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	SMTPMock bool
}

func Load() (*Config, error) {
	c := &Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://igg:igg@localhost:5432/igg?sslmode=disable"),
		JWTSecret:   getenv("JWT_SECRET", ""),
		BaseURL:     getenv("BASE_URL", "http://localhost:8080"),
		Port:        getenv("PORT", "8080"),
		SMTPHost:    getenv("SMTP_HOST", ""),
		SMTPUser:    getenv("SMTP_USER", ""),
		SMTPPass:    getenv("SMTP_PASS", ""),
		SMTPFrom:    getenv("SMTP_FROM", "noreply@example.com"),
		SMTPMock:    getenv("SMTP_MOCK", "true") == "true",
	}

	port, _ := strconv.Atoi(getenv("SMTP_PORT", "587"))
	c.SMTPPort = port

	if c.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
