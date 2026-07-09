package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestCloseSessionUsesOwnerInstanceAndAllowsPrivilegedStop(t *testing.T) {
	var validateReq sts.ValidateRequest
	var closedPath string
	var closeReq map[string]string
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&validateReq); err != nil {
			t.Fatalf("decode validate: %v", err)
		}
		_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
	}))
	defer stsServer.Close()
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sessions/sess-1":
			_ = json.NewEncoder(w).Encode(closeableSession("owner-user", "ACTIVE"))
		case r.Method == http.MethodPost && r.URL.Path == "/sessions/sess-1/close":
			closedPath = r.URL.Path
			if err := json.NewDecoder(r.Body).Decode(&closeReq); err != nil {
				t.Fatalf("decode close: %v", err)
			}
			_ = json.NewEncoder(w).Encode(closeableSession("owner-user", "CLOSED"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	resp, err := svc.CloseSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: "admin-user"}, true)
	if err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if validateReq != (sts.ValidateRequest{UserID: "admin-user", SessionID: "sess-1", Action: "stop"}) {
		t.Fatalf("validate request = %#v", validateReq)
	}
	if closedPath != "/sessions/sess-1/close" || closeReq["ownerPodId"] != "pod-1" || closeReq["ownerInstanceId"] != "instance-1" {
		t.Fatalf("closedPath=%q closeReq=%#v", closedPath, closeReq)
	}
	if resp.Status != "CLOSED" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCloseSessionRejectsBeforeCloseForInvalidStateAndOwner(t *testing.T) {
	tests := []struct {
		name       string
		session    model.Session
		userID     string
		privileged bool
	}{
		{name: "inactive", session: closeableSession("user-1", "CLOSED"), userID: "user-1"},
		{name: "non owner", session: closeableSession("owner-user", "ACTIVE"), userID: "other-user"},
		{name: "missing ownership", session: model.Session{SessionID: "sess-1", UserID: "user-1", Status: "ACTIVE"}, userID: "user-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closeCalls := 0
			stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
			}))
			defer stsServer.Close()
			smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet:
					_ = json.NewEncoder(w).Encode(tt.session)
				case r.Method == http.MethodPost && r.URL.Path == "/sessions/sess-1/close":
					closeCalls++
					t.Fatal("CloseSession called after local rejection")
				default:
					http.NotFound(w, r)
				}
			}))
			defer smServer.Close()

			svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
			if _, err := svc.CloseSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: tt.userID}, tt.privileged); err == nil {
				t.Fatal("CloseSession() error = nil")
			}
			if closeCalls != 0 {
				t.Fatalf("closeCalls = %d", closeCalls)
			}
		})
	}
}

func closeableSession(userID, status string) model.Session {
	session := testControlPlaneSession(userID, status)
	session.OwnerPodID = "pod-1"
	session.OwnerInstanceID = "instance-1"
	return session
}

func TestCloseSessionStopsOnSTSDenialBeforeSessionLookup(t *testing.T) {
	sessionLookups := 0
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: false, Reason: "denied"})
	}))
	defer stsServer.Close()
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionLookups++
		t.Fatal("session-manager should not be called after STS denial")
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	if _, err := svc.CloseSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: "user-1"}, false); err == nil {
		t.Fatal("CloseSession() accepted STS denial")
	}
	if sessionLookups != 0 {
		t.Fatalf("sessionLookups = %d", sessionLookups)
	}
}
