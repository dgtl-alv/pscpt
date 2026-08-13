package config

import "os"

type Config struct {
	Addr          string
	DSN           string
	SessionSecret string
	AppURL        string
}

func Load() Config {
	return Config{
		Addr:          getEnv("PSCPT_ADDR", ":8080"),
		DSN:           getEnv("PSCPT_DSN", "pscpt:pscpt@tcp(127.0.0.1:3308)/pscpt?parseTime=true&multiStatements=true"),
		SessionSecret: getEnv("PSCPT_SESSION_SECRET", "dev-secret-change-me"),
		AppURL:        getEnv("PSCPT_APP_URL", "http://localhost:8080"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
