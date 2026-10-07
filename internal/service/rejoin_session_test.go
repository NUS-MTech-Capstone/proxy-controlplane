package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestStartSessionDefaultsToSSHAndRequestsOwnerToken(t *testing.T) {
	expiresAt := time.Date(2026, time.June, 23, 1, 0, 0, 0, time.UTC)
	var validateReq sts.ValidateRequest
	var tokenReq sts.TokenRequest
	var createReq session_manager.CreateSessionRequest

	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			if err := json.NewDecoder(r.Body).Decode(&validateReq); err != nil {
				t.Fatalf("decode validate request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
		case "/tokens/session":
			if err := json.NewDecoder(r.Body).Decode(&tokenReq); err != nil {
				t.Fatalf("decode token request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(sts.TokenResponse{SessionToken: "owner-token", ExpiresAt: expiresAt})
		default:
			http.NotFound(w, r)
		}
	}))
	defer stsServer.Close()

	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sessions" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
			t.Fatalf("decode create session request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(testControlPlaneSession("user-1", "ACTIVE"))
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	resp, err := svc.StartSession(t.Context(), model.StartSessionRequest{
		UserID: "user-1", HostID: "host-1", CRID: "cr-1",
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	if !reflect.DeepEqual(validateReq, (sts.ValidateRequest{UserID: "user-1", HostID: "host-1", CRID: "cr-1", Action: "start"})) {
		t.Fatalf("validate request = %#v", validateReq)
	}
	if createReq.ConnectionType != "ssh" {
		t.Fatalf("create session connectionType = %q, want ssh", createReq.ConnectionType)
	}
	if !reflect.DeepEqual(tokenReq, (sts.TokenRequest{
		SessionID: "sess-1", UserID: "user-1", HostID: "host-1", CRID: "cr-1",
		ConnectionType: "ssh", Action: "start", Mode: "owner",
	})) {
		t.Fatalf("token request = %#v", tokenReq)
	}
	if resp.SessionToken != "owner-token" || !resp.ExpiresAt.Equal(expiresAt) ||
		resp.WebsocketURL != "wss://proxy/sessions/sess-1/ws" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestObserveSessionRequestsObserverToken(t *testing.T) {
	expiresAt := time.Date(2026, time.June, 23, 1, 30, 0, 0, time.UTC)
	var validateReqs []sts.ValidateRequest
	var tokenReqs []sts.TokenRequest

	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			var req sts.ValidateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode validate request: %v", err)
			}
			validateReqs = append(validateReqs, req)
			_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
		case "/tokens/session":
			var req sts.TokenRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode token request: %v", err)
			}
			tokenReqs = append(tokenReqs, req)
			_ = json.NewEncoder(w).Encode(sts.TokenResponse{SessionToken: "observer-token", ExpiresAt: expiresAt})
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
		_ = json.NewEncoder(w).Encode(testControlPlaneSession("owner-user", "ACTIVE"))
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	for _, userID := range []string{"observer-user", "owner-user"} {
		resp, err := svc.ObserveSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: userID})
		if err != nil {
			t.Fatalf("ObserveSession(%q) error = %v", userID, err)
		}
		if resp.SessionToken != "observer-token" || !resp.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("response = %#v", resp)
		}
	}

	wantValidate := []sts.ValidateRequest{
		{UserID: "observer-user", SessionID: "sess-1", HostID: "host-1", CRID: "cr-1", Action: "observe"},
		{UserID: "owner-user", SessionID: "sess-1", HostID: "host-1", CRID: "cr-1", Action: "observe"},
	}
	for i, want := range wantValidate {
		if !reflect.DeepEqual(validateReqs[i], want) {
			t.Fatalf("validate request %d = %#v, want %#v", i, validateReqs[i], want)
		}
	}
	wantToken := []sts.TokenRequest{
		{
			SessionID: "sess-1", UserID: "observer-user", HostID: "host-1", CRID: "cr-1",
			ConnectionType: "ssh", Action: "observe", Mode: "observer",
		},
		{
			SessionID: "sess-1", UserID: "owner-user", HostID: "host-1", CRID: "cr-1",
			ConnectionType: "ssh", Action: "observe", Mode: "observer",
		},
	}
	for i, want := range wantToken {
		if !reflect.DeepEqual(tokenReqs[i], want) {
			t.Fatalf("token request %d = %#v, want %#v", i, tokenReqs[i], want)
		}
	}
}

func TestObserveSessionDeniesInactiveBeforeIssuingToken(t *testing.T) {
	tokenRequests := 0
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			_ = json.NewEncoder(w).Encode(sts.ValidateResponse{Allowed: true})
		case "/tokens/session":
			tokenRequests++
			t.Fatal("RequestToken called for inactive observe")
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
		_ = json.NewEncoder(w).Encode(testControlPlaneSession("owner-user", "CLOSED"))
	}))
	defer smServer.Close()

	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	if _, err := svc.ObserveSession(t.Context(), "sess-1", model.SessionAccessRequest{UserID: "observer-user"}); err == nil {
		t.Fatal("ObserveSession() error = nil, want inactive rejection")
	}
	if tokenRequests != 0 {
		t.Fatalf("tokenRequests = %d, want 0", tokenRequests)
	}
}

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
