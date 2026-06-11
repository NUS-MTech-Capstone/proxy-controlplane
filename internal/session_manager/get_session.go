package session_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
)

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
