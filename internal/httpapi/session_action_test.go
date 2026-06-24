package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestSessionActionRejoinAndObserve(t *testing.T) {
	now := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
	sm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sessions/sess-1" {
			t.Fatalf("unexpected session-manager request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","websocketUrl":"wss://proxy/sessions/sess-1/ws","status":"ACTIVE"}`))
	}))
	defer sm.Close()
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/validate":
			_, _ = w.Write([]byte(`{"allowed":true}`))
		case "/tokens/session":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sessionToken":"session-token","expiresAt":"` + now.Format(time.RFC3339) + `"}`))
		default:
			t.Fatalf("unexpected sts request %s", r.URL.Path)
		}
	}))
	defer stsServer.Close()
	server := New(service.New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(sm.URL)), fakeIdentityVerifier{
		claims: map[string]any{"sub": "user-1"},
	})

	for _, path := range []string{"/sessions/sess-1/rejoin", "/sessions/sess-1/observe"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("x-amzn-oidc-data", "signed-token")
		req.Header.Set("x-amzn-oidc-identity", "user-1")
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d body=%s", path, rec.Code, http.StatusOK, rec.Body.String())
		}
	}
}

func TestSessionActionValidationErrors(t *testing.T) {
	server := New(service.New(sts.NewSTS("http://127.0.0.1:1"), session_manager.NewSessionManager("http://127.0.0.1:1")), fakeIdentityVerifier{
		claims: map[string]any{"sub": "user-1"},
	})
	req := httptest.NewRequest(http.MethodPost, "/sessions/sess-1/unknown", strings.NewReader(`{}`))
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "user-1")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	req = httptest.NewRequest(http.MethodPost, "/sessions/sess-1/rejoin", strings.NewReader(`{`))
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "user-1")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
