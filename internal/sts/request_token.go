package sts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
)

type TokenRequest struct {
	ActorGroups    []string `json:"actorGroups"`
	SessionID      string   `json:"sessionId"`
	UserID         string   `json:"userId"`
	HostID         string   `json:"hostId"`
	CRID           string   `json:"crId"`
	ConnectionType string   `json:"connectionType"`
	Action         string   `json:"action"`
	Mode           string   `json:"mode"`
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
	observability.SetRequestHeaders(ctx, httpReq)
	start := time.Now()
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		recordSTSDependency(ctx, path, time.Since(start), err, 0)
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		var err error = fmt.Errorf("sts returned status %d", httpResp.StatusCode)
		if httpResp.StatusCode == http.StatusForbidden {
			err = ErrAccessDenied
		}
		recordSTSDependency(ctx, path, time.Since(start), err, httpResp.StatusCode)
		return err
	}
	recordSTSDependency(ctx, path, time.Since(start), nil, httpResp.StatusCode)
	if resp == nil {
		return nil
	}

	return json.NewDecoder(httpResp.Body).Decode(resp)
}

func recordSTSDependency(ctx context.Context, operation string, duration time.Duration, err error, status int) {
	metrics := []observability.Metric{{
		Name:  "STSRequestLatencyMs",
		Unit:  "Milliseconds",
		Value: float64(duration.Milliseconds()),
	}}
	if err != nil {
		metrics = append(metrics, observability.Metric{Name: "STSErrors", Unit: "Count", Value: 1})
		slog.WarnContext(ctx, "STSUnavailable",
			"requestId", observability.RequestID(ctx),
			"operation", operation,
			"durationMs", duration.Milliseconds(),
			"errorKind", observability.ErrorKind(err),
			"status", status,
			"error", err,
		)
	}
	observability.EmitMetric("proxy-controlplane", metrics, map[string]string{"Component": "sts"})
}
