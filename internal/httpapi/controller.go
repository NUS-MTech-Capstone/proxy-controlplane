package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/events"
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
	req.ActorGroups = user.Groups
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
		writeServiceError(w, err)
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
	s.events.Emit(r.Context(), events.Event{
		EventType: "SessionStarted",
		RequestID: observability.RequestID(r.Context()),
		SessionID: resp.SessionID,
		UserID:    req.UserID,
		HostID:    req.HostID,
		CRID:      req.CRID,
		Action:    "start",
		Mode:      "owner",
		Outcome:   "accepted",
	})
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
	req.ActorGroups = user.Groups
	var (
		resp model.SessionResponse
		err  error
	)
	switch action {
	case "rejoin":
		resp, err = s.svc.RejoinSession(r.Context(), sessionID, req)
	case "observe":
		resp, err = s.svc.ObserveSession(r.Context(), sessionID, req)
	case "close":
		s.closeSession(w, r, sessionID, req)
		return
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	slog.InfoContext(r.Context(), "SessionAccess",
		"requestId", observability.RequestID(r.Context()),
		"sessionId", sessionID,
		"userId", req.UserID,
		"action", action,
	)
	eventType := "UserRejoined"
	mode := "owner"
	if action == "observe" {
		eventType = "ObserverJoined"
		mode = "observer"
	}
	s.events.Emit(r.Context(), events.Event{
		EventType: eventType,
		RequestID: observability.RequestID(r.Context()),
		SessionID: sessionID,
		UserID:    req.UserID,
		Action:    action,
		Mode:      mode,
		Outcome:   "accepted",
	})
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) closeSession(
	w http.ResponseWriter,
	r *http.Request,
	sessionID string,
	req model.SessionAccessRequest,
) {
	session, err := s.svc.CloseSession(
		r.Context(),
		sessionID,
		req,
		hasPrivilegedStopRole(req.ActorGroups),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	slog.InfoContext(r.Context(), "SessionClosed",
		"requestId", observability.RequestID(r.Context()),
		"sessionId", sessionID,
		"userId", req.UserID,
		"action", "close",
	)
	s.events.Emit(r.Context(), events.Event{
		EventType: "SessionClosed",
		RequestID: observability.RequestID(r.Context()),
		SessionID: sessionID,
		UserID:    req.UserID,
		HostID:    session.HostID,
		CRID:      session.CRID,
		Action:    "close",
		Outcome:   "accepted",
	})
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	user, ok := cognitoUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing Cognito identity")
		return
	}
	query := r.URL.Query()
	resp, err := s.svc.ListSessions(r.Context(), model.ListSessionsRequest{
		UserID: user.Subject,
		HostID: query.Get("hostId"),
		CRID:   query.Get("crId"),
		Status: query.Get("status"),
		Mine:   query.Get("mine") == "true",
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	slog.InfoContext(r.Context(), "SessionsListed",
		"requestId", observability.RequestID(r.Context()),
		"userId", user.Subject,
		"hostId", query.Get("hostId"),
		"crId", query.Get("crId"),
		"status", query.Get("status"),
		"mine", query.Get("mine"),
	)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) listRejoinCandidates(w http.ResponseWriter, r *http.Request) {
	user, ok := cognitoUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing Cognito identity")
		return
	}
	req := model.RejoinCandidatesRequest{
		ActorGroups: user.Groups,
		UserID:      user.Subject,
		HostID:      r.URL.Query().Get("hostId"),
		CRID:        r.URL.Query().Get("crId"),
	}
	if req.HostID == "" || req.CRID == "" {
		writeError(w, http.StatusBadRequest, "hostId and crId are required")
		return
	}
	resp, err := s.svc.ListRejoinCandidates(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	slog.InfoContext(r.Context(), "RejoinCandidatesListed",
		"requestId", observability.RequestID(r.Context()),
		"userId", req.UserID,
		"hostId", req.HostID,
		"crId", req.CRID,
		"action", "rejoin",
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
