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
	addr := ":" + cfg.Port
	log.Printf("proxy-controlplane listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.New(svc)))
}
