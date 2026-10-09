package httpapi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeIdentityVerifier struct {
	claims       map[string]any
	err          error
	accessClaims map[string]any
	accessErr    error
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (f fakeIdentityVerifier) Verify(context.Context, string) (map[string]any, error) {
	return f.claims, f.err
}

func TestCognitoMiddlewareAddsUserToContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "user-123")
	rec := httptest.NewRecorder()

	handler := cognitoMiddleware(fakeIdentityVerifier{claims: map[string]any{
		"sub": "user-123", "email": "user@example.com",
	}}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := cognitoUserFromContext(r.Context())
		if !ok {
			t.Fatal("Cognito user missing from context")
		}
		if user.Subject != "user-123" {
			t.Fatalf("Subject = %q, want user-123", user.Subject)
		}
		if user.Email != "user@example.com" {
			t.Fatalf("Email = %q, want user@example.com", user.Email)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestCognitoMiddlewareRejectsMissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	cognitoMiddleware(fakeIdentityVerifier{err: errors.New("missing token")}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCognitoMiddlewareRejectsMissingSubject(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	rec := httptest.NewRecorder()

	cognitoMiddleware(fakeIdentityVerifier{claims: map[string]any{
		"email": "user@example.com",
	}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCognitoMiddlewareRejectsIdentityMismatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-amzn-oidc-data", "signed-token")
	req.Header.Set("x-amzn-oidc-identity", "other-user")
	rec := httptest.NewRecorder()

	cognitoMiddleware(fakeIdentityVerifier{claims: map[string]any{
		"sub": "user-123",
	}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestALBJWTVerifierVerifiesSignatureAndHeaderClaims(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	keyClient := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/key-1" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(publicKeyPEM))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	const (
		albARN   = "arn:aws:elasticloadbalancing:ap-southeast-1:123456789012:loadbalancer/app/test/id"
		issuer   = "https://cognito-idp.ap-southeast-1.amazonaws.com/pool"
		clientID = "client-123"
	)
	verifier, err := NewALBJWTVerifier(ALBJWTVerifierConfig{
		Region:           "ap-southeast-1",
		ALBARN:           albARN,
		Issuer:           issuer,
		ClientID:         clientID,
		PublicKeyBaseURL: "https://keys.example.test",
		HTTPClient:       keyClient,
	})
	if err != nil {
		t.Fatal(err)
	}

	token := signedALBToken(t, privateKey, map[string]any{
		"alg":    "ES256",
		"kid":    "key-1",
		"signer": albARN,
		"iss":    issuer,
		"client": clientID,
		"exp":    time.Now().Add(time.Minute).Unix(),
	}, map[string]any{
		"sub":   "user-123",
		"email": "user@example.com",
	})

	claims, err := verifier.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims["sub"] != "user-123" {
		t.Fatalf("sub = %v, want user-123", claims["sub"])
	}
}

func TestALBJWTVerifierRejectsUnexpectedSigner(t *testing.T) {
	verifier, err := NewALBJWTVerifier(ALBJWTVerifierConfig{
		Region:           "ap-southeast-1",
		ALBARN:           "expected",
		Issuer:           "issuer",
		ClientID:         "client",
		PublicKeyBaseURL: "http://127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	header, err := json.Marshal(map[string]any{
		"alg":    "ES256",
		"kid":    "key-1",
		"signer": "unexpected",
		"iss":    "issuer",
		"client": "client",
		"exp":    time.Now().Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString(header) + ".e30.signature"

	if _, err := verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("Verify() succeeded for an unexpected signer")
	}
}

func TestPrivilegedStopRole(t *testing.T) {
	for _, tt := range []struct {
		group   string
		allowed bool
	}{
		{"Admin", true}, {"admin", true}, {"aDmIn", true},
		{"SecurityAnalyst", true}, {"securityanalyst", true},
		{"Auditor", false}, {"", false}, {" admin", false}, {"Admin ", false}, {"SuperAdmin", false},
	} {
		t.Run(tt.group, func(t *testing.T) {
			if got := hasPrivilegedStopRole([]string{tt.group}); got != tt.allowed {
				t.Fatalf("got %v, want %v", got, tt.allowed)
			}
		})
	}
}

func unsignedJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]any{"alg": "none"})
	if err != nil {
		t.Fatal(err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON) + ".signature"
}

func signedALBToken(
	t *testing.T,
	privateKey *ecdsa.PrivateKey,
	header map[string]any,
	claims map[string]any,
) string {
	t.Helper()
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := append(paddedInteger(r, 32), paddedInteger(s, 32)...)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func paddedInteger(value *big.Int, size int) []byte {
	result := make([]byte, size)
	bytes := value.Bytes()
	copy(result[size-len(bytes):], bytes)
	return result
}

func (f fakeIdentityVerifier) VerifyAccess(context.Context, string) (map[string]any, error) {
	if f.accessErr != nil {
		return nil, f.accessErr
	}
	if f.accessClaims != nil {
		return f.accessClaims, nil
	}
	return map[string]any{"sub": f.claims["sub"], "cognito:groups": []string{"Employee"}}, nil
}
