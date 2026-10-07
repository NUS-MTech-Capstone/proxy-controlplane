package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func TestStartReleasesAssignmentAfterCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var closed atomic.Bool
	sm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/close") {
			var owner map[string]string
			_ = json.NewDecoder(r.Body).Decode(&owner)
			if owner["ownerPodId"] != "pod-1" || owner["ownerInstanceId"] != "instance-1" {
				t.Error("missing ownership")
			}
			closed.Store(true)
		}
		_ = json.NewEncoder(w).Encode(model.Session{SessionID: "sess-1", HostID: "host-1", CRID: "cr-1", ConnectionType: "ssh", OwnerPodID: "pod-1", OwnerInstanceID: "instance-1"})
	}))
	defer sm.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/validate" {
			_, _ = w.Write([]byte(`{"allowed":true}`))
			return
		}
		cancel()
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()
	svc := New(sts.NewSTS(upstream.URL), session_manager.NewSessionManager(sm.URL))
	resp, err := svc.StartSession(ctx, model.StartSessionRequest{UserID: "user-1", ActorGroups: []string{"Employee"}, HostID: "host-1", CRID: "cr-1"})
	if err == nil || resp.SessionToken != "" || !closed.Load() {
		t.Fatal("cancellation did not release assignment without issuing token")
	}
}
