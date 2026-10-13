package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestListSessionsUsesCognitoUserAndSanitizesResponse(t *testing.T) {
	var smQuery = make(map[string]string)
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions" {
			http.NotFound(w, r)
			return
		}
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				smQuery[key] = values[0]
			}
		}
		now := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sessions": []map[string]any{{
				"sessionId":       "sess-1",
				"userId":          "user-123",
				"hostId":          "host-1",
				"crId":            "cr-1",
				"connectionType":  "ssh",
				"websocketUrl":    "wss://proxy/sessions/sess-1/ws",
				"ownerPodId":      "pod-1",
				"ownerInstanceId": "instance-1",
				"status":          "ACTIVE",
				"createdAt":       now,
				"lastHeartbeatAt": now.Add(time.Minute),
				"leaseExpiresAt":  now.Add(2 * time.Minute),
			}},
		})
	}))
	defer smServer.Close()

	svc := service.New(sts.NewSTS("http://127.0.0.1:1"), session_manager.NewSessionManager(smServer.URL))
	server := New(svc, fakeIdentityVerifier{claims: map[string]any{"sub": "user-123"}})
	req := httptest.NewRequest(http.MethodGet, "/sessions?hostId=host-1&crId=cr-1&mine=true", nil)
	req.Header.Set(apiVersionHeader, supportedAPIVersion)
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "user-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)
	assertContractResponse(t, req.Method, req.URL.String(), rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if smQuery["userId"] != "user-123" || smQuery["hostId"] != "host-1" ||
		smQuery["crId"] != "cr-1" || smQuery["status"] != "ACTIVE" {
		t.Fatalf("session-manager query = %#v", smQuery)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"sessionId":"sess-1"`) {
		t.Fatalf("response body = %s, want session", body)
	}
	for _, forbidden := range []string{"websocketUrl", "ownerPodId", "ownerInstanceId", "sessionToken"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response body leaked %s: %s", forbidden, body)
		}
	}
}

func TestListSessionsRejectsMissingCognitoIdentity(t *testing.T) {
	svc := service.New(sts.NewSTS("http://127.0.0.1:1"), session_manager.NewSessionManager("http://127.0.0.1:1"))
	server := New(svc, fakeIdentityVerifier{claims: map[string]any{"sub": "user-123"}})
	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	req.Header.Set(apiVersionHeader, supportedAPIVersion)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)
	assertContractResponse(t, req.Method, req.URL.String(), rec)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
