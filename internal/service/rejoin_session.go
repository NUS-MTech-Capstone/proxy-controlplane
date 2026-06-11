package service

import (
	"context"

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
	token, err := s.sts.RequestToken(ctx, sts.TokenRequest{SessionID: session.SessionID, UserID: req.UserID, Mode: "owner"})
	if err != nil {
		return model.SessionResponse{}, err
	}
	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL, ExpiresAt: token.ExpiresAt}, nil
}
