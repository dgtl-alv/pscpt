package config

import "os"

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

func Load() Config {
	return Config{
		Addr:          getEnv("PSCPT_ADDR", ":8080"),
		DSN:           getEnv("PSCPT_DSN", "pscpt:pscpt@tcp(127.0.0.1:3308)/pscpt?parseTime=true&multiStatements=true"),
		SessionSecret: getEnv("PSCPT_SESSION_SECRET", "dev-secret-change-me"),
		AppURL:        getEnv("PSCPT_APP_URL", "http://localhost:8080"),
		Emica: EmicaConfig{
			BaseURL:  getEnv("EMICA_BASE_URL", ""),
			DB:       getEnv("EMICA_ODOO_DB", ""),
			Username: getEnv("EMICA_ODOO_USERNAME", ""),
			APIKey:   getEnv("EMICA_ODOO_API_KEY", getEnv("EMICA_API_ACCESS_TOKEN", "")),
			Password: getEnv("EMICA_ODOO_PASSWORD", ""),
			Timeout:  getEnv("EMICA_TIMEOUT", "45s"),
		},
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
