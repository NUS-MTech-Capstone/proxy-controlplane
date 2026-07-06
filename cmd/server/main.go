package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/config"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/events"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/httpapi"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func main() {
	ctx := context.Background()
	observability.ConfigureLogger("proxy-controlplane")
	cfg := config.Load()
	svc := service.New(sts.NewSTS(cfg.STSURL), session_manager.NewSessionManager(cfg.SessionManagerURL))
	eventPublisher, err := events.NewPublisher(ctx, cfg.EventPublisher, cfg.AWSRegion, cfg.EventsTopicARN)
	if err != nil {
		slog.Error("metadata event publisher setup failed", "error", err)
		os.Exit(1)
	}
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
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(svc, identityVerifier, eventPublisher),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
