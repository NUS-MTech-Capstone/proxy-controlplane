package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type STSClient struct {
	baseURL string
	client  *http.Client
}

type ValidateRequest struct {
	UserID    string `json:"userId"`
	HostID    string `json:"hostId,omitempty"`
	CRID      string `json:"crId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Action    string `json:"action"`
}

type ValidateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type TokenRequest struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	Mode      string `json:"mode"`
}

type TokenResponse struct {
	SessionToken string    `json:"sessionToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func NewSTS(baseURL string) *STSClient {
	return &STSClient{baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (c *STSClient) Validate(ctx context.Context, req ValidateRequest) error {
	var resp ValidateResponse
	if err := c.post(ctx, "/validate", req, &resp); err != nil {
		return err
	}
	if !resp.Allowed {
		return fmt.Errorf("access denied: %s", resp.Reason)
	}
	return nil
}

func (c *STSClient) IssueToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	var resp TokenResponse
	err := c.post(ctx, "/tokens/session", req, &resp)
	return resp, err
}

func (c *STSClient) post(ctx context.Context, path string, req any, resp any) error {
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
		return fmt.Errorf("sts returned status %d", httpResp.StatusCode)
	}
	return json.NewDecoder(httpResp.Body).Decode(resp)
}
