package httpapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/service"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

// Cross-repository test: build sts/cmd/server and set DDM_STS_INTEGRATION_BINARY
// plus DDM_NEST_CONTRACT_FIXTURE to nest-backend/test/fixtures/sts-change-request.json.
// Ordinary standalone repository tests do not require sibling checkouts.
func TestTrustedSTSIntegration(t *testing.T) {
	binary := os.Getenv("DDM_STS_INTEGRATION_BINARY")
	if binary == "" {
		t.Skip("DDM_STS_INTEGRATION_BINARY is not set")
	}
	data, err := os.ReadFile(os.Getenv("DDM_NEST_CONTRACT_FIXTURE"))
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]any
	if err = json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	albKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	accessKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&albKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	keys := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/alb-key" {
			_, _ = w.Write(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
			return
		}
		if r.URL.Path == "/.well-known/jwks.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{rsaJWK(accessKey, "access-key")}})
			return
		}
		http.NotFound(w, r)
	}))
	defer keys.Close()
	verifier, err := NewALBJWTVerifier(ALBJWTVerifierConfig{Region: "test", ALBARN: "test-alb", Issuer: keys.URL, ClientID: "client-1", PublicKeyBaseURL: keys.URL, HTTPClient: keys.Client()})
	if err != nil {
		t.Fatal(err)
	}
	albToken := signedALBToken(t, albKey, map[string]any{"alg": "ES256", "kid": "alb-key", "signer": "test-alb", "iss": keys.URL, "client": "client-1", "exp": time.Now().Add(time.Minute).Unix()}, map[string]any{"sub": "user-1"})
	var mu sync.Mutex
	cr := map[string]any{}
	var lookupCount int
	expireAtIssuance := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		lookupCount++
		if expireAtIssuance && lookupCount == 2 {
			cr["implementationEndAt"] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
		}
		_ = json.NewEncoder(w).Encode(cr)
	}))
	defer upstream.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	log, err := os.Create(filepath.Join(t.TempDir(), "sts.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	cmd := exec.Command(binary)
	cmd.Env = []string{fmt.Sprintf("PORT=%d", port), "JWT_SIGNING_KEY=0123456789abcdef0123456789abcdef", "CHANGE_REQUEST_URL=" + upstream.URL,
		`SERVER_INVENTORY_STATIC_JSON={"host-1":{"address":"test:22","username":"test","credentialSecretId":"test-key","hostPublicKey":"ssh-ed25519 TEST","connectionType":"ssh"}}`}
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	stsURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: time.Second}
	ready := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		resp, err := client.Get(stsURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		t.Fatal("STS did not become healthy")
	}
	creates, closes := 0, 0
	sm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == "POST" && r.URL.Path == "/sessions" {
			creates++
		}
		if strings.HasSuffix(r.URL.Path, "/close") {
			closes++
			var owner map[string]string
			_ = json.NewDecoder(r.Body).Decode(&owner)
			if owner["ownerPodId"] != "pod-1" || owner["ownerInstanceId"] != "instance-1" {
				t.Error("cleanup lost ownership")
			}
		}
		if r.Method == "GET" && r.URL.Path == "/sessions" {
			_, _ = w.Write([]byte(`{"sessions":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","ownerPodId":"pod-1","ownerInstanceId":"instance-1","status":"ACTIVE","websocketUrl":"wss://direct/sessions/sess-1/ws"}`))
	}))
	defer sm.Close()
	tests := []struct {
		name    string
		mutate  func(map[string]any)
		groups  []string
		allowed bool
	}{
		{"valid", nil, []string{"Employee"}, true},
		{"lowercase role", nil, []string{"employee"}, true},
		{"lowercase admin", func(cr map[string]any) { cr["allowedRoles"] = []string{} }, []string{"admin"}, true},
		{"mixed case admin", func(cr map[string]any) { cr["allowedRoles"] = []string{} }, []string{"aDmIn"}, true},
		{"expired", func(cr map[string]any) {
			cr["implementationEndAt"] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		}, []string{"admin"}, false},
		{"future", func(cr map[string]any) {
			cr["implementationStartAt"] = time.Now().Add(time.Minute).UTC().Format(time.RFC3339)
		}, []string{"Employee"}, false},
		{"unauthorized", nil, []string{"Auditor"}, false},
		{"missing scope", func(cr map[string]any) { delete(cr, "hostIds") }, []string{"Employee"}, false},
		{"empty scope", func(cr map[string]any) { cr["hostIds"] = []string{} }, []string{"admin"}, false},
		{"out of scope", func(cr map[string]any) { cr["hostIds"] = []string{"host-2"} }, []string{"Employee"}, false},
		{"expiry during assignment", nil, []string{"Employee"}, false},
	}
	for _, tt := range tests {
		for _, action := range []string{"start", "rejoin", "observe", "discovery"} {
			if tt.name == "expiry during assignment" && action != "start" {
				continue
			}
			t.Run(tt.name+"/"+action, func(t *testing.T) {
				mu.Lock()
				cr = map[string]any{}
				for k, v := range original {
					cr[k] = v
				}
				cr["implementationStartAt"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
				cr["implementationEndAt"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
				if tt.mutate != nil {
					tt.mutate(cr)
				}
				creates, closes, lookupCount = 0, 0, 0
				expireAtIssuance = tt.name == "expiry during assignment"
				mu.Unlock()
				handler := New(service.New(sts.NewSTS(stsURL), session_manager.NewSessionManager(sm.URL)), verifier)
				server := httptest.NewServer(handler)
				defer server.Close()
				path := "/sessions/sess-1/" + action
				method := "POST"
				body := `{"hostId":"forged-host","crId":"forged-cr","userId":"forged","actorGroups":["Admin"]}`
				if action == "start" {
					path = "/sessions/start"
					body = `{"hostId":"host-1","crId":"cr-1","userId":"forged","actorGroups":["Admin"]}`
				}
				if action == "discovery" {
					path = "/sessions/rejoin-candidates?hostId=host-1&crId=cr-1"
					method = "GET"
				}
				req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
				req.Header.Set("x-api-version", "1")
				req.Header.Set("x-amzn-oidc-identity", "user-1")
				req.Header.Set("x-amzn-oidc-data", albToken)
				claims := accessClaims(keys.URL)
				claims["cognito:groups"] = tt.groups
				req.Header.Set("x-amzn-oidc-accesstoken", rsaAccessToken(t, accessKey, "access-key", claims))
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				want := 403
				if tt.allowed {
					want = 200
					if action == "start" {
						want = 201
					}
				}
				if resp.StatusCode != want {
					t.Fatalf("status=%d want=%d", resp.StatusCode, want)
				}
				mu.Lock()
				defer mu.Unlock()
				if !tt.allowed && strings.Contains(string(bodyBytes), "sessionToken") {
					t.Error("denial returned token")
				}
				if !tt.allowed && !expireAtIssuance && creates != 0 {
					t.Error("denial created a session")
				}
				if expireAtIssuance && (creates != 1 || closes != 1) {
					t.Error("expired issuance did not release assignment")
				}
				if tt.allowed && action != "discovery" {
					var response map[string]any
					_ = json.Unmarshal(bodyBytes, &response)
					if response["sessionToken"] == nil || response["websocketUrl"] != "wss://direct/sessions/sess-1/ws" {
						t.Error("valid access lost direct dataplane token/URL")
					}
				}
			})
		}
	}
}
