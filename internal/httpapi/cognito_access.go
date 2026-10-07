package httpapi

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CognitoAccessVerifier uses only the configured issuer's JWKS, never token URLs.
// Keep a bounded cache and rate-limit unknown-key refreshes to prevent fetch storms.
type CognitoAccessVerifier struct {
	issuer, clientID              string
	client                        *http.Client
	mu                            sync.Mutex
	keys                          map[string]*rsa.PublicKey
	expiresAt, lastUnknownRefresh time.Time
	retryAfter                    time.Time
}

func NewCognitoAccessVerifier(issuer, clientID string, client *http.Client) *CognitoAccessVerifier {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &CognitoAccessVerifier{issuer: issuer, clientID: clientID, client: client}
}

func (v *CognitoAccessVerifier) VerifyAccess(ctx context.Context, token string) (map[string]any, error) {
	invalid := errors.New("invalid Cognito access token")
	if len(token) > 16384 || v.issuer == "" || v.clientID == "" {
		return nil, invalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, invalid
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if decodeJWTPart(parts[0], &header) != nil || header.Alg != "RS256" || header.Kid == "" || len(header.Kid) > 256 {
		return nil, invalid
	}
	var payload struct {
		Issuer    string   `json:"iss"`
		ClientID  string   `json:"client_id"`
		TokenUse  string   `json:"token_use"`
		Subject   string   `json:"sub"`
		ExpiresAt int64    `json:"exp"`
		IssuedAt  int64    `json:"iat"`
		NotBefore int64    `json:"nbf"`
		Groups    []string `json:"cognito:groups"`
	}
	if decodeJWTPart(parts[1], &payload) != nil {
		return nil, invalid
	}
	now := time.Now().Unix()
	if payload.Issuer != v.issuer || payload.ClientID != v.clientID || payload.TokenUse != "access" || strings.TrimSpace(payload.Subject) == "" || payload.ExpiresAt <= now || payload.IssuedAt <= 0 || payload.IssuedAt > now || payload.NotBefore > now {
		return nil, invalid
	}
	for _, group := range payload.Groups {
		if strings.TrimSpace(group) == "" || strings.TrimSpace(group) != group {
			return nil, invalid
		}
	}
	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return nil, invalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, invalid
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return nil, invalid
	}
	if payload.ExpiresAt <= time.Now().Unix() {
		return nil, invalid
	}
	groups := payload.Groups
	if groups == nil {
		groups = []string{}
	}
	return map[string]any{"sub": payload.Subject, "cognito:groups": groups}, nil
}

func (v *CognitoAccessVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	invalid := errors.New("Cognito key unavailable")
	if now.Before(v.expiresAt) {
		if key := v.keys[kid]; key != nil {
			return key, nil
		}
		if now.Sub(v.lastUnknownRefresh) < 30*time.Second {
			return nil, invalid
		}
		v.lastUnknownRefresh = now
	}
	if now.Before(v.retryAfter) {
		return nil, invalid
	}
	v.retryAfter = now.Add(time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(v.issuer, "/")+"/.well-known/jwks.json", nil) // #nosec G704 -- issuer is trusted deployment configuration; no token field controls the URL.
	if err != nil {
		return nil, invalid
	}
	// Prevent redirects from extending the configured trust boundary.
	client := *v.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req) // #nosec G704 -- fixed JWKS path on the configured issuer; redirects are disabled.
	if err != nil {
		return nil, invalid
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, invalid
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return nil, invalid
	}
	var jwks struct {
		Keys []struct{ Kty, Kid, Use, Alg, N, E string }
	}
	if json.Unmarshal(body, &jwks) != nil || len(jwks.Keys) == 0 || len(jwks.Keys) > 32 {
		return nil, invalid
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, item := range jwks.Keys {
		if item.Kty != "RSA" || item.Use != "sig" || item.Alg != "RS256" || item.Kid == "" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(item.N)
		e, errE := base64.RawURLEncoding.DecodeString(item.E)
		if errN != nil || errE != nil || len(n) < 256 || len(n) > 1024 || len(e) == 0 || len(e) > 4 {
			return nil, invalid
		}
		exponent := new(big.Int).SetBytes(e).Int64()
		if exponent < 3 || exponent > 2147483647 || exponent%2 == 0 {
			return nil, invalid
		}
		if _, exists := keys[item.Kid]; exists {
			return nil, invalid
		}
		keys[item.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exponent)}
	}
	v.keys, v.expiresAt = keys, now.Add(time.Hour)
	v.retryAfter = time.Time{}
	if key := keys[kid]; key != nil {
		return key, nil
	}
	v.lastUnknownRefresh = now
	return nil, invalid
}
