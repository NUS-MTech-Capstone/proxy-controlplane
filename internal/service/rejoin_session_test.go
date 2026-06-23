package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestRejoinSessionDeniesNonOwnerBeforeIssuingToken(t *testing.T) {
	tokenRequests := 0
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
		case "/tokens/session":
			tokenRequests++
			t.Fatal("RequestToken called for non-owner rejoin")
		default:
			http.NotFound(w, r)
		}
	}))
	defer stsServer.Close()

	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sessions/sess-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(testControlPlaneSession("other-user", "ACTIVE"))
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	if _, err := svc.RejoinSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: "user-1"}); err == nil {
		t.Fatal("RejoinSession() error = nil, want non-owner rejection")
	}
	if tokenRequests != 0 {
		t.Fatalf("tokenRequests = %d, want 0", tokenRequests)
	}
}

func TestRejoinSessionDeniesInactiveBeforeIssuingToken(t *testing.T) {
	tokenRequests := 0
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
		case "/tokens/session":
			tokenRequests++
			t.Fatal("RequestToken called for inactive rejoin")
		default:
			http.NotFound(w, r)
		}
	}))
	defer stsServer.Close()

	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sessions/sess-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(testControlPlaneSession("user-1", "CLOSED"))
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	if _, err := svc.RejoinSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: "user-1"}); err == nil {
		t.Fatal("RejoinSession() error = nil, want inactive rejection")
	}
	if tokenRequests != 0 {
		t.Fatalf("tokenRequests = %d, want 0", tokenRequests)
	}
}

func testControlPlaneSession(userID, status string) model.Session {
	return model.Session{
		SessionID:       "sess-1",
		UserID:          userID,
		HostID:          "host-1",
		CRID:            "cr-1",
		ConnectionType:  "ssh",
		WebsocketURL:    "wss://proxy/sessions/sess-1/ws",
		Status:          status,
		CreatedAt:       time.Date(2026, time.June, 23, 0, 0, 0, 0, time.UTC),
		LastHeartbeatAt: time.Date(2026, time.June, 23, 0, 1, 0, 0, time.UTC),
		LeaseExpiresAt:  time.Date(2026, time.June, 23, 0, 6, 0, 0, time.UTC),
	}
}
