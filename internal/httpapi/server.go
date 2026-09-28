package httpapi

import (
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/events"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
)

type Server struct {
	svc              *service.Service
	mux              *http.ServeMux
	identityVerifier IdentityVerifier
	events           events.Publisher
}

func New(svc *service.Service, identityVerifier IdentityVerifier, publishers ...events.Publisher) *Server {
	publisher := events.Publisher(events.NoopPublisher{})
	if len(publishers) > 0 && publishers[0] != nil {
		publisher = publishers[0]
	}
	s := &Server{
		svc:              svc,
		mux:              http.NewServeMux(),
		identityVerifier: identityVerifier,
		events:           publisher,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	securityHeadersMiddleware(observability.RequestIDMiddleware(observability.LogRequests(apiVersionMiddleware(s.mux)))).ServeHTTP(w, r)
}
