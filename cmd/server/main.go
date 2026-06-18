package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/config"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/httpapi"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func main() {
	observability.ConfigureLogger("proxy-controlplane")
	cfg := config.Load()
	svc := service.New(sts.NewSTS(cfg.STSURL), session_manager.NewSessionManager(cfg.SessionManagerURL))
	identityVerifier, err := httpapi.NewALBJWTVerifier(httpapi.ALBJWTVerifierConfig{
		Region:   cfg.AWSRegion,
		ALBARN:   cfg.ALBARN,
		Issuer:   cfg.CognitoIssuer,
		ClientID: cfg.CognitoClientID,
	})
	if err != nil {
		slog.Error("identity verifier setup failed", "error", err)
		os.Exit(1)
	}
	addr := ":" + cfg.Port
	slog.Info("server_listening", "addr", addr)
	if err := http.ListenAndServe(addr, httpapi.New(svc, identityVerifier)); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
