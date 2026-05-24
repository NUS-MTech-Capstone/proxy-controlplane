package service

import (
	"context"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/client"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
)

type Service struct {
	sts *client.STSClient
	sm  *client.SessionManagerClient
}

func New(sts *client.STSClient, sm *client.SessionManagerClient) *Service {
	return &Service{sts: sts, sm: sm}
}

func (s *Service) StartSession(ctx context.Context, req model.StartSessionRequest) (model.SessionResponse, error) {
	if err := s.sts.Validate(ctx, client.ValidateRequest{UserID: req.UserID, HostID: req.HostID, CRID: req.CRID, Action: "start"}); err != nil {
		return model.SessionResponse{}, err
	}
	session, err := s.sm.CreateSession(ctx, client.CreateSessionRequest{UserID: req.UserID, HostID: req.HostID, CRID: req.CRID})
	if err != nil {
		return model.SessionResponse{}, err
	}
	token, err := s.sts.IssueToken(ctx, client.TokenRequest{SessionID: session.SessionID, UserID: req.UserID, Mode: "owner"})
	if err != nil {
		return model.SessionResponse{}, err
	}
	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL, ExpiresAt: token.ExpiresAt}, nil
}

func (s *Service) RejoinSession(ctx context.Context, sessionID string, req model.SessionAccessRequest) (model.SessionResponse, error) {
	if err := s.sts.Validate(ctx, client.ValidateRequest{UserID: req.UserID, SessionID: sessionID, Action: "rejoin"}); err != nil {
		return model.SessionResponse{}, err
	}
	session, err := s.sm.GetSession(ctx, sessionID)
	if err != nil {
		return model.SessionResponse{}, err
	}
	token, err := s.sts.IssueToken(ctx, client.TokenRequest{SessionID: session.SessionID, UserID: req.UserID, Mode: "owner"})
	if err != nil {
		return model.SessionResponse{}, err
	}
	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL, ExpiresAt: token.ExpiresAt}, nil
}

func (s *Service) ObserveSession(ctx context.Context, sessionID string, req model.SessionAccessRequest) (model.SessionResponse, error) {
	if err := s.sts.Validate(ctx, client.ValidateRequest{UserID: req.UserID, SessionID: sessionID, Action: "observe"}); err != nil {
		return model.SessionResponse{}, err
	}
	session, err := s.sm.GetSession(ctx, sessionID)
	if err != nil {
		return model.SessionResponse{}, err
	}
	token, err := s.sts.IssueToken(ctx, client.TokenRequest{SessionID: session.SessionID, UserID: req.UserID, Mode: "observer"})
	if err != nil {
		return model.SessionResponse{}, err
	}
	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL + "?mode=observe", ExpiresAt: token.ExpiresAt}, nil
}
