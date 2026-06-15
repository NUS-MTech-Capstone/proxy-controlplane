package httpapi

import "net/http"

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s.mux.Handle("POST /sessions/start", cognitoMiddleware(s.identityVerifier, http.HandlerFunc(s.start)))
	s.mux.Handle("POST /sessions/", cognitoMiddleware(s.identityVerifier, http.HandlerFunc(s.sessionAction)))
	s.mux.HandleFunc("POST /automation/runs/start", s.startAutomationRun)
	s.mux.HandleFunc("POST /automation/runs/", s.automationRunAction)
}
