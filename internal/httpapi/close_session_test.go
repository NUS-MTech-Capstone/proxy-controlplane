package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestCloseSessionAllowsOwner(t *testing.T) {
	var stsRequest sts.ValidateRequest
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&stsRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer stsServer.Close()

	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sessions/sess-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionId": "sess-1", "userId": "user-123", "hostId": "host-1", "crId": "cr-1",
				"connectionType": "ssh", "ownerPodId": "pod-1", "ownerInstanceId": "instance-1",
				"status": "ACTIVE", "createdAt": now, "lastHeartbeatAt": now, "leaseExpiresAt": now.Add(time.Minute),
			})
		case r.Method == http.MethodPost && r.URL.Path == "/sessions/sess-1/close":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionId": "sess-1", "userId": "user-123", "hostId": "host-1", "crId": "cr-1",
				"connectionType": "ssh", "status": "CLOSED", "createdAt": now,
				"lastHeartbeatAt": now, "leaseExpiresAt": now.Add(time.Minute),
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer smServer.Close()

	svc := service.New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	server := New(svc, fakeIdentityVerifier{claims: map[string]any{"sub": "user-123"}})
	req := httptest.NewRequest(http.MethodPost, "/sessions/sess-1/close", strings.NewReader("{}"))
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "user-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if stsRequest.Action != "stop" || stsRequest.SessionID != "sess-1" || stsRequest.UserID != "user-123" {
		t.Fatalf("sts request = %#v", stsRequest)
	}
	if !strings.Contains(rec.Body.String(), `"status":"CLOSED"`) {
		t.Fatalf("response body = %s, want CLOSED session", rec.Body.String())
	}
	for _, forbidden := range []string{"websocketUrl", "ownerPodId", "ownerInstanceId", "sessionToken"} {
		if strings.Contains(rec.Body.String(), forbidden) {
			t.Fatalf("response body leaked %s: %s", forbidden, rec.Body.String())
		}
	}
}

func TestCloseSessionRejectsNonOwnerWithoutPrivilegedRole(t *testing.T) {
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer stsServer.Close()

	now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/sessions/sess-1" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionId": "sess-1", "userId": "owner-123", "hostId": "host-1", "crId": "cr-1",
				"connectionType": "ssh", "ownerPodId": "pod-1", "ownerInstanceId": "instance-1",
				"status": "ACTIVE", "createdAt": now, "lastHeartbeatAt": now, "leaseExpiresAt": now.Add(time.Minute),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer smServer.Close()

	svc := service.New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))
	server := New(svc, fakeIdentityVerifier{claims: map[string]any{"sub": "other-user"}})
	req := httptest.NewRequest(http.MethodPost, "/sessions/sess-1/close", strings.NewReader("{}"))
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "other-user")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}
