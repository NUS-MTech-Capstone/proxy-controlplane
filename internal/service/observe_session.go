package service

import (
	"context"
	"errors"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) ObserveSession(ctx context.Context, sessionID string, req model.SessionAccessRequest) (model.SessionResponse, error) {
	session, err := s.sm.GetSession(ctx, sessionID)
	if err != nil {
		return model.SessionResponse{}, err
	}
	if session.Status != "ACTIVE" {
		return model.SessionResponse{}, errors.New("session is not active")
	}

	if err := s.sts.Validate(ctx, sts.ValidateRequest{ActorGroups: req.ActorGroups, UserID: req.UserID, SessionID: sessionID, HostID: session.HostID, CRID: session.CRID, Action: "observe"}); err != nil {
		return model.SessionResponse{}, err
	}

	token, err := s.sts.RequestToken(ctx,
		sts.TokenRequest{ActorGroups: req.ActorGroups,
			SessionID: session.SessionID, UserID: req.UserID, HostID: session.HostID, CRID: session.CRID,
			ConnectionType: session.ConnectionType, Action: "observe", Mode: "observer",
		})
	if err != nil {
		return model.SessionResponse{}, err
	}

	return model.SessionResponse{
		SessionID:    session.SessionID,
		SessionToken: token.SessionToken,
		WebsocketURL: session.WebsocketURL,
		ExpiresAt:    token.ExpiresAt,
	}, nil
}
