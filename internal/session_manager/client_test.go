package session_manager

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCreateSessionSendsRequestAndPropagatesRequestID(t *testing.T) {
	client := NewSessionManager("http://session-manager.example")
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/sessions" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content-type = %q", req.Header.Get("Content-Type"))
		}
		if req.Header.Get("X-Request-ID") != "req-1" {
			t.Fatalf("X-Request-ID = %q", req.Header.Get("X-Request-ID"))
		}
		var body CreateSessionRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.UserID != "user-1" || body.HostID != "host-1" || body.CRID != "cr-1" || body.ConnectionType != "ssh" {
			t.Fatalf("body = %#v", body)
		}
		return jsonResponse(http.StatusCreated, `{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","websocketUrl":"wss://proxy/sessions/sess-1/ws"}`), nil
	})

	ctx := observability.ContextWithRequestID(t.Context(), "req-1")
	session, err := client.CreateSession(ctx, CreateSessionRequest{
		UserID: "user-1", HostID: "host-1", CRID: "cr-1", ConnectionType: "ssh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.SessionID != "sess-1" || session.WebsocketURL == "" {
		t.Fatalf("session = %#v", session)
	}
}

func TestSessionManagerClientCloseAutomationRunAcceptsEmptySuccessBody(t *testing.T) {
	client := NewSessionManager("http://session-manager.example")
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/automation/runs/run-1/close" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusNoContent, ``), nil
	})

	if err := client.CloseAutomationRun(t.Context(), "run-1"); err != nil {
		t.Fatalf("CloseAutomationRun() error = %v", err)
	}
}

func TestSessionManagerClientAutomationRunEndpoints(t *testing.T) {
	tests := []struct {
		name string
		call func(*SessionManagerClient) error
	}{
		{
			name: "create",
			call: func(client *SessionManagerClient) error {
				run, err := client.CreateAutomationRun(t.Context(), "inc-1", "host-1")
				if err != nil {
					return err
				}
				if run.RunID != "run-1" || run.IncidentID != "inc-1" || run.HostID != "host-1" {
					t.Fatalf("run = %#v", run)
				}
				return nil
			},
		},
		{
			name: "get",
			call: func(client *SessionManagerClient) error {
				run, err := client.GetAutomationRun(t.Context(), "run-1")
				if err != nil {
					return err
				}
				if run.RunID != "run-1" || run.OwnerPodID != "pod-1" {
					t.Fatalf("run = %#v", run)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSessionManager("http://session-manager.example")
			client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch tt.name {
				case "create":
					if req.Method != http.MethodPost || req.URL.Path != "/automation/runs" {
						t.Fatalf("request = %s %s", req.Method, req.URL.Path)
					}
					var body map[string]string
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["incidentId"] != "inc-1" || body["hostId"] != "host-1" {
						t.Fatalf("body = %#v", body)
					}
				case "get":
					if req.Method != http.MethodGet || req.URL.Path != "/automation/runs/run-1" {
						t.Fatalf("request = %s %s", req.Method, req.URL.Path)
					}
				}
				return jsonResponse(http.StatusOK, `{"runId":"run-1","incidentId":"inc-1","hostId":"host-1","ownerPodId":"pod-1","ownerInstanceId":"instance-1","operationsUrl":"http://ops","status":"ACTIVE"}`), nil
			})
			if err := tt.call(client); err != nil {
				t.Fatalf("%s automation run error = %v", tt.name, err)
			}
		})
	}
}

func TestSessionManagerClientListSessionsBuildsFilterQuery(t *testing.T) {
	client := NewSessionManager("http://session-manager.example")
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/sessions" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("userId") != "user-1" || query.Get("hostId") != "host-1" ||
			query.Get("crId") != "cr-1" || query.Get("status") != "ACTIVE" {
			t.Fatalf("query = %s", req.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, `{"sessions":[{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","status":"ACTIVE"}]}`), nil
	})

	sessions, err := client.ListSessions(t.Context(), ListSessionsFilter{
		UserID: "user-1", HostID: "host-1", CRID: "cr-1", Status: "ACTIVE",
	})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].SessionID != "sess-1" {
		t.Fatalf("sessions = %#v", sessions)
	}
}

func TestSessionManagerClientCloseSessionSendsOwnerIdentity(t *testing.T) {
	client := NewSessionManager("http://session-manager.example")
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/sessions/sess-1/close" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["ownerPodId"] != "pod-1" || body["ownerInstanceId"] != "instance-1" {
			t.Fatalf("body = %#v", body)
		}
		return jsonResponse(http.StatusOK, `{"sessionId":"sess-1","ownerPodId":"pod-1","ownerInstanceId":"instance-1","status":"CLOSED"}`), nil
	})

	session, err := client.CloseSession(t.Context(), "sess-1", "pod-1", "instance-1")
	if err != nil {
		t.Fatalf("CloseSession() error = %v", err)
	}
	if session.SessionID != "sess-1" || session.OwnerInstanceID != "instance-1" {
		t.Fatalf("session = %#v", session)
	}
}

func TestSessionManagerClientReportsHTTPAndDecodeErrors(t *testing.T) {
	tests := []struct {
		name      string
		response  *http.Response
		wantError string
	}{
		{
			name:      "non-2xx",
			response:  jsonResponse(http.StatusServiceUnavailable, `{"error":"unavailable"}`),
			wantError: "status 503",
		},
		{
			name:      "malformed json",
			response:  jsonResponse(http.StatusOK, `{`),
			wantError: "unexpected EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSessionManager("http://session-manager.example")
			client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return tt.response, nil
			})
			_, err := client.GetSession(t.Context(), "sess-1")
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("GetSession() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestSessionManagerClientPropagatesTransportError(t *testing.T) {
	client := NewSessionManager("http://session-manager.example")
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})

	_, err := client.ListSessions(context.Background(), ListSessionsFilter{UserID: "user-1"})
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("ListSessions() error = %v, want transport error", err)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
