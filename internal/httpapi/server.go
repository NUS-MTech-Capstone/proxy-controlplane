package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
)

type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

func New(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.Path)
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	s.mux.HandleFunc("POST /sessions/start", s.start)
	s.mux.HandleFunc("POST /sessions/", s.sessionAction)
}

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var req model.StartSessionRequest
	if !decode(w, r, &req) {
		return
	}
	if req.UserID == "" || req.HostID == "" || req.CRID == "" {
		writeError(w, http.StatusBadRequest, "userId, hostId and crId are required")
		return
	}
	resp, err := s.svc.StartSession(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	log.Printf("SessionStarted sessionId=%s userId=%s hostId=%s", resp.SessionID, req.UserID, req.HostID)
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) sessionAction(w http.ResponseWriter, r *http.Request) {
	sessionID, action := parseSessionPath(r.URL.Path)
	var req model.SessionAccessRequest
	if !decode(w, r, &req) {
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "userId is required")
		return
	}
	var (
		resp model.SessionResponse
		err  error
	)
	switch action {
	case "rejoin":
		resp, err = s.svc.RejoinSession(r.Context(), sessionID, req)
	case "observe":
		resp, err = s.svc.ObserveSession(r.Context(), sessionID, req)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	log.Printf("SessionAccess action=%s sessionId=%s userId=%s", action, sessionID, req.UserID)
	writeJSON(w, http.StatusOK, resp)
}

func parseSessionPath(path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "sessions" {
		return "", ""
	}
	return parts[1], parts[2]
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}
