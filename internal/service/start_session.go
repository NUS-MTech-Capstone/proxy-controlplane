package service

import (
	"context"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) StartSession(ctx context.Context, req model.StartSessionRequest) (model.SessionResponse, error) {
	if req.ConnectionType == "" {
		req.ConnectionType = "ssh"
	}
	if err := s.sts.Validate(ctx, sts.ValidateRequest{UserID: req.UserID, HostID: req.HostID, CRID: req.CRID, Action: "start"}); err != nil {
		return model.SessionResponse{}, err
	}
	session, err := s.sm.CreateSession(ctx, session_manager.CreateSessionRequest{
		UserID: req.UserID, HostID: req.HostID, CRID: req.CRID, ConnectionType: req.ConnectionType,
	})
	if err != nil {
		return model.SessionResponse{}, err
	}
	token, err := s.sts.RequestToken(ctx, sts.TokenRequest{
		SessionID: session.SessionID, UserID: req.UserID, HostID: session.HostID, CRID: session.CRID,
		ConnectionType: session.ConnectionType, Action: "start", Mode: "owner",
	})
	if err != nil {
		return model.SessionResponse{}, err
	}

	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL, ExpiresAt: token.ExpiresAt}, nil
}
