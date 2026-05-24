package main

import (
	"log"
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/client"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/config"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/httpapi"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
)

func main() {
	cfg := config.Load()
	svc := service.New(client.NewSTS(cfg.STSURL), client.NewSessionManager(cfg.SessionManagerURL))
	addr := ":" + cfg.Port
	log.Printf("proxy-controlplane listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.New(svc)))
}
