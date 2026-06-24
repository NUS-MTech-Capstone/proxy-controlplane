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

func TestAutomationHTTPValidationAndLifecycle(t *testing.T) {
	now := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
	sm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/automation/runs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"runId":"run-1","incidentId":"inc-1","hostId":"host-1","ownerPodId":"pod-1","ownerInstanceId":"instance-1","operationsUrl":"http://ops","status":"ACTIVE"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/automation/runs/run-1":
			_, _ = w.Write([]byte(`{"runId":"run-1","incidentId":"inc-1","hostId":"host-1","ownerPodId":"pod-1","ownerInstanceId":"instance-1","operationsUrl":"http://ops","status":"ACTIVE"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/automation/runs/run-1/close":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected session-manager request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer sm.Close()
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tokens/automation":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"capabilityToken":"capability","expiresAt":"` + now.Format(time.RFC3339) + `"}`))
		case "/tokens/automation/validate":
			_, _ = w.Write([]byte(`{"allowed":true}`))
		default:
			t.Fatalf("unexpected sts request %s", r.URL.Path)
		}
	}))
	defer stsServer.Close()
	server := New(service.New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(sm.URL)), fakeIdentityVerifier{})

	doAutomation(t, server, http.MethodPost, "/automation/runs/start", `{}`, "", http.StatusBadRequest)
	doAutomation(t, server, http.MethodPost, "/automation/runs/start", `{"incidentId":"inc-1","hostId":"host-1","phase":"diagnose","allowedOperations":["health_check"]}`, "Bearer workload", http.StatusCreated)
	doAutomation(t, server, http.MethodPost, "/automation/runs/run-1/token", `{"phase":"diagnose","allowedOperations":["health_check"]}`, "Bearer workload", http.StatusCreated)
	doAutomation(t, server, http.MethodPost, "/automation/runs/run-1/close", `{}`, "", http.StatusUnauthorized)
	doAutomation(t, server, http.MethodPost, "/automation/runs/run-1/close", `{}`, "Bearer capability", http.StatusNoContent)
	doAutomation(t, server, http.MethodPost, "/automation/runs/run-1/other", `{}`, "Bearer capability", http.StatusNotFound)
	doAutomation(t, server, http.MethodPost, "/automation/bad/run-1/close", `{}`, "Bearer capability", http.StatusNotFound)
}

func TestAutomationHTTPBadGateway(t *testing.T) {
	server := New(service.New(sts.NewSTS("http://127.0.0.1:1"), session_manager.NewSessionManager("http://127.0.0.1:1")), fakeIdentityVerifier{})
	doAutomation(t, server, http.MethodPost, "/automation/runs/start", `{"incidentId":"inc-1","hostId":"host-1","phase":"diagnose","allowedOperations":["health_check"]}`, "Bearer workload", http.StatusBadGateway)
}

func TestAutomationBearerTokenAndPathParsing(t *testing.T) {
	if got := bearerToken("Bearer token-1"); got != "token-1" {
		t.Fatalf("bearerToken() = %q", got)
	}
	if got := bearerToken("Basic token-1"); got != "" {
		t.Fatalf("bearerToken() = %q, want empty", got)
	}
	runID, action := parseAutomationPath("/automation/runs/run-1/token")
	if runID != "run-1" || action != "token" {
		t.Fatalf("parseAutomationPath() = %q/%q", runID, action)
	}
}

func doAutomation(t *testing.T, server *Server, method, path, body, auth string, wantStatus int) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	server.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d body=%s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
}
