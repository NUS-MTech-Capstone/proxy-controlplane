package httpapi

import (
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/api"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("GET /openapi.yaml", serveYAML(api.OpenAPI))

	s.mux.Handle("POST /sessions/start", cognitoMiddleware(s.identityVerifier, http.HandlerFunc(s.start)))
	s.mux.Handle("GET /sessions", cognitoMiddleware(s.identityVerifier, http.HandlerFunc(s.listSessions)))
	s.mux.Handle("GET /sessions/rejoin-candidates", cognitoMiddleware(s.identityVerifier, http.HandlerFunc(s.listRejoinCandidates)))
	s.mux.Handle("POST /sessions/", cognitoMiddleware(s.identityVerifier, http.HandlerFunc(s.sessionAction)))
}
