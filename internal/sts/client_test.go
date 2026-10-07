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
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Validate() error = %v, want denial", err)
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
