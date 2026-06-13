package service

import (
	"context"
	"errors"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) RejoinSession(ctx context.Context, sessionID string, req model.SessionAccessRequest) (model.SessionResponse, error) {
	if err := s.sts.Validate(ctx, sts.ValidateRequest{UserID: req.UserID, SessionID: sessionID, Action: "rejoin"}); err != nil {
		return model.SessionResponse{}, err
	}
	session, err := s.sm.GetSession(ctx, sessionID)
	if err != nil {
		return model.SessionResponse{}, err
	}
	if session.Status != "ACTIVE" {
		return model.SessionResponse{}, errors.New("session is not active")
	}
	if session.UserID != req.UserID {
		return model.SessionResponse{}, errors.New("authenticated user does not own the session")
	}

	token, err := s.sts.RequestToken(ctx, sts.TokenRequest{
		SessionID: session.SessionID, UserID: req.UserID, HostID: session.HostID, CRID: session.CRID,
		ConnectionType: session.ConnectionType, Action: "rejoin", Mode: "owner",
	})
	if err != nil {
		return model.SessionResponse{}, err
	}

	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL, ExpiresAt: token.ExpiresAt}, nil
}
