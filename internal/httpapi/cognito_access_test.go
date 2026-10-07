package httpapi

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func rsaAccessToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "kid": kid})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func rsaJWK(key *rsa.PrivateKey, kid string) map[string]string {
	return map[string]string{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid, "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
}
func accessClaims(issuer string) map[string]any {
	return map[string]any{"iss": issuer, "client_id": "client-1", "token_use": "access", "sub": "user-1", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "cognito:groups": []string{"Employee"}}
}
func TestCognitoAccessSignatureAndClaims(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			t.Error("unexpected JWKS path")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{rsaJWK(key, "key-1")}})
	}))
	defer jwks.Close()
	verifier := NewCognitoAccessVerifier(jwks.URL, "client-1", jwks.Client())
	cases := []struct {
		name  string
		field string
		value any
	}{
		{"wrong issuer", "iss", "https://untrusted.invalid"},
		{"wrong client", "client_id", "other"},
		{"ID token", "token_use", "id"},
		{"expired", "exp", time.Now().Add(-time.Second).Unix()},
		{"missing expiry", "exp", nil},
		{"future issue", "iat", time.Now().Add(time.Hour).Unix()},
		{"future nbf", "nbf", time.Now().Add(time.Hour).Unix()},
		{"missing subject", "sub", ""},
		{"invalid groups", "cognito:groups", []any{1, "Admin"}},
		{"blank group", "cognito:groups", []string{"Admin", " "}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			claims := accessClaims(jwks.URL)
			claims[tt.field] = tt.value
			if _, err := verifier.VerifyAccess(t.Context(), rsaAccessToken(t, key, "key-1", claims)); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	for name, token := range map[string]string{
		"forged":      rsaAccessToken(t, otherKey, "key-1", accessClaims(jwks.URL)),
		"unsigned":    unsignedJWT(t, accessClaims(jwks.URL)),
		"unknown key": rsaAccessToken(t, key, "missing", accessClaims(jwks.URL)),
		"missing":     "", "oversized": strings.Repeat("a", 16385),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := verifier.VerifyAccess(t.Context(), token); err == nil {
				t.Fatal("invalid signature accepted")
			}
		})
	}
	claims, err := verifier.VerifyAccess(t.Context(), rsaAccessToken(t, key, "key-1", accessClaims(jwks.URL)))
	if err != nil || claims["sub"] != "user-1" {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func TestCognitoJWKSCacheRotationAndFailure(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	var calls atomic.Int32
	var rotated atomic.Bool
	var fail atomic.Bool
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			http.Error(w, "unavailable", 500)
			return
		}
		keys := []any{rsaJWK(key, "key-1")}
		if rotated.Load() {
			keys = append(keys, rsaJWK(key, "key-2"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	defer jwks.Close()
	verifier := NewCognitoAccessVerifier(jwks.URL, "client-1", jwks.Client())
	token := rsaAccessToken(t, key, "key-1", accessClaims(jwks.URL))
	for range 2 {
		if _, err := verifier.VerifyAccess(t.Context(), token); err != nil {
			t.Fatal(err)
		}
	}
	var workers sync.WaitGroup
	for range 10 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := verifier.VerifyAccess(t.Context(), token); err != nil {
				t.Error("concurrent cached validation failed")
			}
		}()
	}
	workers.Wait()
	if calls.Load() != 1 {
		t.Fatal("known key was not cached")
	}
	rotated.Store(true)
	if _, err := verifier.VerifyAccess(t.Context(), rsaAccessToken(t, key, "key-2", accessClaims(jwks.URL))); err != nil {
		t.Fatalf("rotation failed: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatal("rotation did not refresh")
	}
	for range 2 {
		if _, err := verifier.VerifyAccess(t.Context(), rsaAccessToken(t, key, "missing", accessClaims(jwks.URL))); err == nil {
			t.Fatal("unknown key allowed")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("unknown-key fetch was not bounded")
	}
	fail.Store(true)
	verifier.expiresAt = time.Time{}
	if _, err := verifier.VerifyAccess(t.Context(), token); err == nil {
		t.Fatal("expired cache bypassed JWKS failure")
	}
}

type combinedTestVerifier struct {
	fakeIdentityVerifier
	access *CognitoAccessVerifier
}

func (v combinedTestVerifier) VerifyAccess(ctx context.Context, token string) (map[string]any, error) {
	return v.access.VerifyAccess(ctx, token)
}

func TestMiddlewareBindsVerifiedAccessSubject(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{rsaJWK(key, "key-1")}})
	}))
	defer jwks.Close()
	verifier := combinedTestVerifier{fakeIdentityVerifier: fakeIdentityVerifier{claims: map[string]any{"sub": "user-1"}}, access: NewCognitoAccessVerifier(jwks.URL, "client-1", jwks.Client())}
	for _, subject := range []string{"user-1", "other-user"} {
		t.Run(subject, func(t *testing.T) {
			claims := accessClaims(jwks.URL)
			claims["sub"] = subject
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("x-amzn-oidc-identity", "user-1")
			req.Header.Set("x-amzn-oidc-accesstoken", rsaAccessToken(t, key, "key-1", claims))
			rec := httptest.NewRecorder()
			cognitoMiddleware(verifier, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, _ := cognitoUserFromContext(r.Context())
				if len(user.Groups) != 1 || user.Groups[0] != "Employee" {
					t.Error("verified groups missing")
				}
				w.WriteHeader(204)
			})).ServeHTTP(rec, req)
			want := 204
			if subject != "user-1" {
				want = 401
			}
			if rec.Code != want {
				t.Fatalf("status=%d want=%d", rec.Code, want)
			}
		})
	}
}
