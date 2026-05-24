package config

import "os"

type Config struct {
	Port              string
	STSURL            string
	SessionManagerURL string
}

func Load() Config {
	return Config{
		Port:              env("PORT", "8080"),
		STSURL:            env("STS_URL", "http://localhost:8082"),
		SessionManagerURL: env("SESSION_MANAGER_URL", "http://localhost:8081"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
