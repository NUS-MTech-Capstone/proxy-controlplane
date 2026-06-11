package httpapi

import "net/http"

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /sessions/start", s.start)
	s.mux.HandleFunc("POST /sessions/", s.sessionAction)
}
