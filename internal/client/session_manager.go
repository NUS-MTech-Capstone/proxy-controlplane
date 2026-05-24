package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
)

type SessionManagerClient struct {
	baseURL string
	client  *http.Client
}

type CreateSessionRequest struct {
	UserID string `json:"userId"`
	HostID string `json:"hostId"`
	CRID   string `json:"crId"`
}

func NewSessionManager(baseURL string) *SessionManagerClient {
	return &SessionManagerClient{baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (c *SessionManagerClient) CreateSession(ctx context.Context, req CreateSessionRequest) (model.Session, error) {
	var resp model.Session
	err := c.post(ctx, "/sessions", req, &resp)
	return resp, err
}

func (c *SessionManagerClient) GetSession(ctx context.Context, sessionID string) (model.Session, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sessions/"+sessionID, nil)
	if err != nil {
		return model.Session{}, err
	}
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return model.Session{}, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return model.Session{}, fmt.Errorf("session-manager returned status %d", httpResp.StatusCode)
	}
	var session model.Session
	return session, json.NewDecoder(httpResp.Body).Decode(&session)
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
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("session-manager returned status %d", httpResp.StatusCode)
	}
	return json.NewDecoder(httpResp.Body).Decode(resp)
}
