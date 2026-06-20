package session_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
)

type ListSessionsFilter struct {
	UserID string
	HostID string
	CRID   string
	Status string
}

type listSessionsResponse struct {
	Sessions []model.Session `json:"sessions"`
}

func (c *SessionManagerClient) ListSessions(ctx context.Context, filter ListSessionsFilter) ([]model.Session, error) {
	query := url.Values{}
	query.Set("userId", filter.UserID)
	query.Set("hostId", filter.HostID)
	query.Set("crId", filter.CRID)
	query.Set("status", filter.Status)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sessions?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	observability.SetRequestHeaders(ctx, httpReq)
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("session-manager returned status %d", httpResp.StatusCode)
	}
	var resp listSessionsResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}
	return resp.Sessions, nil
}
