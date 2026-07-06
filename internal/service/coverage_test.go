package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestListSessionsDefaultsStatusAndMapsCandidates(t *testing.T) {
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query()
		if query.Get("status") != "ACTIVE" || query.Get("userId") != "user-1" || query.Get("hostId") != "host-1" {
			t.Fatalf("query = %s", req.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sessions":[{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","status":"ACTIVE"}]}`))
	}))
	defer smServer.Close()
	svc := New(sts.NewSTS("http://127.0.0.1:1"), session_manager.NewSessionManager(smServer.URL))

	resp, err := svc.ListSessions(t.Context(), model.ListSessionsRequest{UserID: "user-1", HostID: "host-1", Mine: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sessions) != 1 || resp.Sessions[0].SessionID != "sess-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestListRejoinCandidatesValidatesBeforeListing(t *testing.T) {
	stsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/validate" {
			t.Fatalf("sts path = %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"allowed":true}`))
	}))
	defer stsServer.Close()
	smServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("status") != "ACTIVE" {
			t.Fatalf("query = %s", req.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessions":[{"sessionId":"sess-1","userId":"user-1","hostId":"host-1","crId":"cr-1","connectionType":"ssh","status":"ACTIVE"}]}`))
	}))
	defer smServer.Close()
	svc := New(sts.NewSTS(stsServer.URL), session_manager.NewSessionManager(smServer.URL))

	resp, err := svc.ListRejoinCandidates(t.Context(), model.RejoinCandidatesRequest{UserID: "user-1", HostID: "host-1", CRID: "cr-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sessions) != 1 {
		t.Fatalf("response = %#v", resp)
	}
}
