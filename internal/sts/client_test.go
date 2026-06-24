package sts

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
)

func TestValidateSendsRequestAndRejectsDeniedResponse(t *testing.T) {
	client := NewSTS("http://sts.example")
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/validate" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if req.Header.Get("X-Request-ID") != "req-1" {
			t.Fatalf("X-Request-ID = %q", req.Header.Get("X-Request-ID"))
		}
		var body ValidateRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.UserID != "user-1" || body.HostID != "host-1" || body.Action != "start" {
			t.Fatalf("body = %#v", body)
		}
		return stsJSONResponse(http.StatusOK, `{"allowed":false,"reason":"change request denied"}`), nil
	})

	ctx := observability.ContextWithRequestID(t.Context(), "req-1")
	err := client.Validate(ctx, ValidateRequest{UserID: "user-1", HostID: "host-1", Action: "start"})
	if err == nil || !strings.Contains(err.Error(), "change request denied") {
		t.Fatalf("Validate() error = %v, want denial", err)
	}
}

func TestValidateAutomationTokenAcceptsAllowedAndRejectsDenied(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "allowed", body: `{"allowed":true}`},
		{name: "denied", body: `{"allowed":false,"reason":"expired"}`, wantErr: "expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSTS("http://sts.example")
			client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/tokens/automation/validate" {
					t.Fatalf("path = %s", req.URL.Path)
				}
				return stsJSONResponse(http.StatusOK, tt.body), nil
			})
			err := client.ValidateAutomationToken(t.Context(), "run-1", "token-1")
			if tt.wantErr == "" && err != nil {
				t.Fatalf("ValidateAutomationToken() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("ValidateAutomationToken() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRequestAutomationTokenSendsBearerTokenAndHandlesErrors(t *testing.T) {
	tests := []struct {
		name      string
		transport http.RoundTripper
		wantErr   string
	}{
		{
			name: "success",
			transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("Authorization") != "Bearer workload-token" {
					t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
				}
				var body AutomationTokenRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.RunID != "run-1" || body.OwnerInstanceID != "instance-1" {
					t.Fatalf("body = %#v", body)
				}
				return stsJSONResponse(http.StatusCreated, `{"capabilityToken":"capability"}`), nil
			}),
		},
		{
			name: "status",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return stsJSONResponse(http.StatusForbidden, `{"error":"denied"}`), nil
			}),
			wantErr: "status 403",
		},
		{
			name: "transport",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("dial failed")
			}),
			wantErr: "dial failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSTS("http://sts.example")
			client.client.Transport = tt.transport
			resp, err := client.RequestAutomationToken(t.Context(), "workload-token", AutomationTokenRequest{
				RunID: "run-1", IncidentID: "inc-1", HostID: "host-1", Phase: "diagnose",
				AllowedOperations: []string{"health_check"}, OwnerPodID: "pod-1", OwnerInstanceID: "instance-1",
			})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("RequestAutomationToken() error = %v", err)
				}
				if resp.CapabilityToken != "capability" {
					t.Fatalf("response = %#v", resp)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RequestAutomationToken() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestRequestTokenReportsMalformedJSON(t *testing.T) {
	client := NewSTS("http://sts.example")
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return stsJSONResponse(http.StatusOK, `{`), nil
	})

	_, err := client.RequestToken(t.Context(), TokenRequest{SessionID: "sess-1"})
	if err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("RequestToken() error = %v, want decode error", err)
	}
}

func stsJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
