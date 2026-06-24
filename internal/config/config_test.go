package config

import "testing"

func TestLoadDefaultsAndBuildsCognitoIssuerFromPool(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("COGNITO_USER_POOL_ID", "pool-1")

	cfg := Load()
	if cfg.Port != "8080" || cfg.STSURL != "http://localhost:8082" || cfg.SessionManagerURL != "http://localhost:8081" {
		t.Fatalf("defaults = %#v", cfg)
	}
	if cfg.CognitoIssuer != "https://cognito-idp.us-west-2.amazonaws.com/pool-1" {
		t.Fatalf("CognitoIssuer = %q", cfg.CognitoIssuer)
	}
}

func TestLoadPrefersExplicitValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("STS_URL", "http://sts")
	t.Setenv("SESSION_MANAGER_URL", "http://sm")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("COGNITO_USER_POOL_ID", "pool-1")
	t.Setenv("COGNITO_ISSUER", "https://issuer")
	t.Setenv("ALB_ARN", "alb")
	t.Setenv("COGNITO_CLIENT_ID", "client")

	cfg := Load()
	if cfg.Port != "9090" || cfg.STSURL != "http://sts" || cfg.SessionManagerURL != "http://sm" ||
		cfg.AWSRegion != "us-east-1" || cfg.CognitoIssuer != "https://issuer" ||
		cfg.ALBARN != "alb" || cfg.CognitoClientID != "client" {
		t.Fatalf("Load() = %#v", cfg)
	}
}
