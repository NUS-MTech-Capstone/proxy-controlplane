package config

import "os"

type Config struct {
	Port              string
	STSURL            string
	SessionManagerURL string
	AWSRegion         string
	ALBARN            string
	CognitoIssuer     string
	CognitoClientID   string
}

func Load() Config {
	region := env("AWS_REGION", "ap-southeast-1")
	userPoolID := os.Getenv("COGNITO_USER_POOL_ID")
	issuer := os.Getenv("COGNITO_ISSUER")
	if issuer == "" && userPoolID != "" {
		issuer = "https://cognito-idp." + region + ".amazonaws.com/" + userPoolID
	}
	return Config{
		Port:              env("PORT", "8080"),
		STSURL:            env("STS_URL", "http://localhost:8082"),
		SessionManagerURL: env("SESSION_MANAGER_URL", "http://localhost:8081"),
		AWSRegion:         region,
		ALBARN:            os.Getenv("ALB_ARN"),
		CognitoIssuer:     issuer,
		CognitoClientID:   os.Getenv("COGNITO_CLIENT_ID"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
