package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
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
	if req.ConnectionType == "" {
		req.ConnectionType = "ssh"
	}
	if req.ConnectionType != "ssh" {
		writeError(w, http.StatusBadRequest, "only ssh connectionType is supported")
		return
	}
	resp, err := s.svc.StartSession(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	slog.InfoContext(r.Context(), "SessionStarted",
		"requestId", observability.RequestID(r.Context()),
		"sessionId", resp.SessionID,
		"userId", req.UserID,
		"hostId", req.HostID,
		"crId", req.CRID,
		"action", "start",
		"mode", "owner",
	)
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
	slog.InfoContext(r.Context(), "SessionAccess",
		"requestId", observability.RequestID(r.Context()),
		"sessionId", sessionID,
		"userId", req.UserID,
		"action", action,
	)
	writeJSON(w, http.StatusOK, resp)
}

func parseSessionPath(path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "sessions" {
		return "", ""
	}
	return parts[1], parts[2]
}
