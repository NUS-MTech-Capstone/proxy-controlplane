package main

import (
	"log"
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/config"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/httpapi"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func main() {
	cfg := config.Load()
	svc := service.New(sts.NewSTS(cfg.STSURL), session_manager.NewSessionManager(cfg.SessionManagerURL))
	identityVerifier, err := httpapi.NewALBJWTVerifier(httpapi.ALBJWTVerifierConfig{
		Region:   cfg.AWSRegion,
		ALBARN:   cfg.ALBARN,
		Issuer:   cfg.CognitoIssuer,
		ClientID: cfg.CognitoClientID,
	})
	if err != nil {
		log.Fatal(err)
	}
	addr := ":" + cfg.Port
	log.Printf("proxy-controlplane listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.New(svc, identityVerifier)))
}
