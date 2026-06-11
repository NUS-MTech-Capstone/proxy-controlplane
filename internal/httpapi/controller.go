package httpapi

import (
	"log"
	"net/http"
	"strings"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
)

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var req model.StartSessionRequest
	if !decode(w, r, &req) {
		return
	}
	user, ok := cognitoUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing Cognito identity")
		return
	}
	req.UserID = user.Subject
	if req.HostID == "" || req.CRID == "" {
		writeError(w, http.StatusBadRequest, "hostId and crId are required")
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
	user, ok := cognitoUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing Cognito identity")
		return
	}
	req.UserID = user.Subject
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
