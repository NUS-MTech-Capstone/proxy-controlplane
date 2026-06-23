package session_manager

import (
	"context"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
)

type closeSessionRequest struct {
	OwnerPodID      string `json:"ownerPodId"`
	OwnerInstanceID string `json:"ownerInstanceId"`
}

func (c *SessionManagerClient) CloseSession(
	ctx context.Context,
	sessionID, ownerPodID, ownerInstanceID string,
) (model.Session, error) {
	var session model.Session
	err := c.post(ctx, "/sessions/"+sessionID+"/close", closeSessionRequest{
		OwnerPodID:      ownerPodID,
		OwnerInstanceID: ownerInstanceID,
	}, &session)
	return session, err
}
