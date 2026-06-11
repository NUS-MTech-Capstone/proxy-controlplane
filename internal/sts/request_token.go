package sts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TokenRequest struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	Mode      string `json:"mode"`
}

type TokenResponse struct {
	SessionToken string    `json:"sessionToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

func (c *Client) RequestToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	var resp TokenResponse
	err := c.post(ctx, "/tokens/session", req, &resp)
	return resp, err
}

func (c *Client) post(ctx context.Context, path string, req any, resp any) error {
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
