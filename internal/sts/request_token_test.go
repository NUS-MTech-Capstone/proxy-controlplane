package sts

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRequestTokenSendsSessionActionAndTargetIdentity(t *testing.T) {
	client := NewSTS("http://sts.example")
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body TokenRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Action != "rejoin" || body.Mode != "owner" || body.HostID != "host-1" ||
			body.CRID != "cr-1" || body.ConnectionType != "ssh" {
			t.Fatalf("request body = %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"sessionToken":"token"}`)),
		}, nil
	})

	_, err := client.RequestToken(t.Context(), TokenRequest{
		SessionID: "sess-1", UserID: "user-1", HostID: "host-1", CRID: "cr-1",
		ConnectionType: "ssh", Action: "rejoin", Mode: "owner",
	})
	if err != nil {
		t.Fatal(err)
	}
}
