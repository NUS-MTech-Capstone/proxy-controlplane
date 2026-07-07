package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestStartRejectsAgentConnectionTypeUntilStreamingTransportExists(t *testing.T) {
	svc := service.New(sts.NewSTS("http://127.0.0.1:1"), session_manager.NewSessionManager("http://127.0.0.1:1"))
	server := New(svc, fakeIdentityVerifier{claims: map[string]any{"sub": "user-123"}})
	req := httptest.NewRequest(http.MethodPost, "/sessions/start", strings.NewReader(`{
		"hostId":"host-1",
		"crId":"cr-1",
		"connectionType":"agent"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(apiVersionHeader, supportedAPIVersion)
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "user-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "only ssh connectionType is supported") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
