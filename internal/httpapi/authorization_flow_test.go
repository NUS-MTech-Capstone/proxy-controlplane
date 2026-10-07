package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestTrustedActorForwardingAndPolicyDenials(t *testing.T) {
	for _, action := range []string{"start", "rejoin", "observe", "discovery"} {
		for _, denyAt := range []string{"", "validate", "token"} {
			if action == "discovery" && denyAt == "token" {
				continue
			}
			t.Run(action+"/"+denyAt, func(t *testing.T) {
				creates, closes, tokens, validations := 0, 0, 0, 0
				sm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == http.MethodPost && r.URL.Path == "/sessions":
						creates++
					case r.URL.Path == "/sessions/sess-1/close":
						closes++
						var body map[string]string
						_ = json.NewDecoder(r.Body).Decode(&body)
						if body["ownerPodId"] != "pod-1" || body["ownerInstanceId"] != "instance-1" {
							t.Error("cleanup missing ownership")
						}
					case r.Method == http.MethodGet && r.URL.Path == "/sessions":
						_, _ = w.Write([]byte(`{"sessions":[]}`))
						return
					case r.Method == http.MethodGet && r.URL.Path == "/sessions/sess-1":
					default:
						t.Errorf("unexpected SM call %s %s", r.Method, r.URL.Path)
					}
					_, _ = w.Write([]byte(`{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","ownerPodId":"pod-1","ownerInstanceId":"instance-1","status":"ACTIVE"}`))
				}))
				defer sm.Close()
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req sts.TokenRequest
					_ = json.NewDecoder(r.Body).Decode(&req)
					if req.UserID != "user-1" || !reflect.DeepEqual(req.ActorGroups, []string{"Employee"}) || req.HostID != "host-1" || req.CRID != "cr-1" {
						t.Error("trusted actor or scope was not forwarded")
					}
					if r.URL.Path == "/validate" {
						validations++
						if denyAt == "validate" {
							http.Error(w, `secret denial`, 403)
							return
						}
						_, _ = w.Write([]byte(`{"allowed":true}`))
						return
					}
					tokens++
					if denyAt == "token" {
						http.Error(w, `secret denial`, 403)
						return
					}
					if action == "observe" && req.Mode != "observer" {
						t.Error("observer became writable")
					}
					_, _ = w.Write([]byte(`{"sessionToken":"test-token"}`))
				}))
				defer upstream.Close()
				server := New(service.New(sts.NewSTS(upstream.URL), session_manager.NewSessionManager(sm.URL)), fakeIdentityVerifier{claims: map[string]any{"sub": "user-1"}})
				path := "/sessions/sess-1/" + action
				method := http.MethodPost
				body := `{"hostId":"forged-host","crId":"forged-cr","userId":"forged-user","actorGroups":["Admin"]}`
				if action == "start" {
					path = "/sessions/start"
					body = `{"hostId":"host-1","crId":"cr-1","userId":"forged-user","actorGroups":["Admin"]}`
				}
				if action == "discovery" {
					path = "/sessions/rejoin-candidates?hostId=host-1&crId=cr-1&actorGroups=Admin"
					method = http.MethodGet
				}
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				req.Header.Set("x-api-version", "1")
				req.Header.Set("x-amzn-oidc-identity", "user-1")
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, req)
				want := 200
				if action == "start" {
					want = 201
				}
				if denyAt != "" {
					want = 403
				}
				if rec.Code != want {
					t.Fatalf("status=%d want=%d", rec.Code, want)
				}
				if validations != 1 {
					t.Error("missing policy check")
				}
				if denyAt == "validate" && (creates != 0 || tokens != 0) {
					t.Error("denial allowed assignment or token issuance")
				}
				if action == "start" && denyAt == "token" && closes != 1 {
					t.Error("failed issuance did not release assignment")
				}
				if action != "start" && closes != 0 {
					t.Error("denial closed existing session")
				}
				if denyAt != "" && (strings.Contains(rec.Body.String(), "sessionToken") || strings.Contains(rec.Body.String(), "secret")) {
					t.Error("denial leaked token or upstream data")
				}
			})
		}
	}
}

func TestPrivilegedCloseUsesVerifiedGroups(t *testing.T) {
	for _, groups := range [][]string{{"Employee"}, {"Admin"}, {"SecurityAnalyst"}} {
		t.Run(strings.Join(groups, ","), func(t *testing.T) {
			closed := false
			sm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/close") {
					closed = true
				}
				_, _ = w.Write([]byte(`{"sessionId":"sess-1","userId":"other-owner","ownerPodId":"pod-1","ownerInstanceId":"instance-1","status":"ACTIVE"}`))
			}))
			defer sm.Close()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"allowed":true}`)) }))
			defer upstream.Close()
			server := New(service.New(sts.NewSTS(upstream.URL), session_manager.NewSessionManager(sm.URL)), fakeIdentityVerifier{claims: map[string]any{"sub": "user-1"}, accessClaims: map[string]any{"sub": "user-1", "cognito:groups": groups}})
			req := httptest.NewRequest(http.MethodPost, "/sessions/sess-1/close", strings.NewReader(`{"actorGroups":["Admin"]}`))
			req.Header.Set("x-api-version", "1")
			req.Header.Set("x-amzn-oidc-identity", "user-1")
			req.Header.Set("x-amzn-oidc-accesstoken", unsignedJWT(t, map[string]any{"cognito:groups": []string{"Admin"}}))
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)
			if closed != (groups[0] != "Employee") {
				t.Error("close used unverified groups")
			}
		})
	}
}
