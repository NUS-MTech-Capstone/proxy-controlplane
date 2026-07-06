package session_manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
)

type CreateSessionRequest struct {
	UserID         string `json:"userId"`
	HostID         string `json:"hostId"`
	CRID           string `json:"crId"`
	ConnectionType string `json:"connectionType"`
}

func (c *SessionManagerClient) CreateSession(ctx context.Context, req CreateSessionRequest) (model.Session, error) {
	var resp model.Session
	err := c.post(ctx, "/sessions", req, &resp)
	return resp, err
}

func (c *SessionManagerClient) post(ctx context.Context, path string, req any, resp any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	observability.SetRequestHeaders(ctx, httpReq)
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("session-manager returned status %d", httpResp.StatusCode)
	}
	if resp == nil {
		return nil
	}
	return json.NewDecoder(httpResp.Body).Decode(resp)
}
