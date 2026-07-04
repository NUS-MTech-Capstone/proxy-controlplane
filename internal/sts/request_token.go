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
	SessionID      string `json:"sessionId"`
	UserID         string `json:"userId"`
	HostID         string `json:"hostId"`
	CRID           string `json:"crId"`
	ConnectionType string `json:"connectionType"`
	Action         string `json:"action"`
	Mode           string `json:"mode"`
}

type TokenResponse struct {
	SessionToken string    `json:"sessionToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type AutomationTokenRequest struct {
	RunID             string   `json:"runId"`
	IncidentID        string   `json:"incidentId"`
	HostID            string   `json:"hostId"`
	Phase             string   `json:"phase"`
	AllowedOperations []string `json:"allowedOperations"`
	PlanHash          string   `json:"planHash,omitempty"`
	OwnerPodID        string   `json:"ownerPodId"`
	OwnerInstanceID   string   `json:"ownerInstanceId"`
}

type AutomationTokenResponse struct {
	CapabilityToken string    `json:"capabilityToken"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type AutomationValidateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func (c *Client) RequestToken(ctx context.Context, req TokenRequest) (TokenResponse, error) {
	var resp TokenResponse
	err := c.post(ctx, "/tokens/session", req, &resp)
	return resp, err
}

func (c *Client) ValidateAutomationToken(ctx context.Context, runID, token string) error {
	var resp AutomationValidateResponse
	if err := c.post(ctx, "/tokens/automation/validate", map[string]string{
		"runId": runID, "token": token,
	}, &resp); err != nil {
		return err
	}
	if !resp.Allowed {
		return fmt.Errorf("automation token rejected: %s", resp.Reason)
	}
	return nil
}

func (c *Client) RequestAutomationToken(ctx context.Context, workloadToken string, req AutomationTokenRequest) (AutomationTokenResponse, error) {
	var resp AutomationTokenResponse
	body, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tokens/automation", bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+workloadToken)
	observability.SetRequestHeaders(ctx, httpReq)
	start := time.Now()
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		recordSTSDependency(ctx, "automation_token", time.Since(start), err, 0)
		return resp, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		err := fmt.Errorf("sts returned status %d", httpResp.StatusCode)
		recordSTSDependency(ctx, "automation_token", time.Since(start), err, httpResp.StatusCode)
		return resp, err
	}
	recordSTSDependency(ctx, "automation_token", time.Since(start), nil, httpResp.StatusCode)
	return resp, json.NewDecoder(httpResp.Body).Decode(&resp)
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
		err := fmt.Errorf("sts returned status %d", httpResp.StatusCode)
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
