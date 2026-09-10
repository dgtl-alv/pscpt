package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Addr          string
	DSN           string
	SessionSecret string
	AppURL        string
	Emica         EmicaConfig
}

type EmicaConfig struct {
	BaseURL  string
	DB       string
	Username string
	APIKey   string
	Password string
	Timeout  string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:          getEnv("PSCPT_ADDR", ":8080"),
		DSN:           os.Getenv("PSCPT_DSN"),
		SessionSecret: os.Getenv("PSCPT_SESSION_SECRET"),
		AppURL:        getEnv("PSCPT_APP_URL", "http://localhost:8080"),
		Emica: EmicaConfig{
			BaseURL: os.Getenv("EMICA_BASE_URL"), DB: os.Getenv("EMICA_ODOO_DB"),
			Username: os.Getenv("EMICA_ODOO_USERNAME"), APIKey: os.Getenv("EMICA_API_ACCESS_TOKEN"),
			Password: os.Getenv("EMICA_ODOO_PASSWORD"), Timeout: getEnv("EMICA_TIMEOUT", "45s"),
		},
	}
	var missing []string
	if strings.TrimSpace(cfg.DSN) == "" {
		missing = append(missing, "PSCPT_DSN")
	}
	if strings.TrimSpace(cfg.SessionSecret) == "" {
		missing = append(missing, "PSCPT_SESSION_SECRET")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("required environment variables are unset: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
