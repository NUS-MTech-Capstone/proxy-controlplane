package service

import (
	"context"
	"errors"
	"net/url"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) ObserveSession(ctx context.Context, sessionID string, req model.SessionAccessRequest) (model.SessionResponse, error) {
	if err := s.sts.Validate(ctx,
		sts.ValidateRequest{UserID: req.UserID, SessionID: sessionID, Action: "observe"}); err != nil {
		return model.SessionResponse{}, err
	}
	session, err := s.sm.GetSession(ctx, sessionID)
	if err != nil {
		return model.SessionResponse{}, err
	}
	if session.Status != "ACTIVE" {
		return model.SessionResponse{}, errors.New("session is not active")
	}

	token, err := s.sts.RequestToken(ctx,
		sts.TokenRequest{SessionID: session.SessionID, UserID: req.UserID, Mode: "observer"})
	if err != nil {
		return model.SessionResponse{}, err
	}

	return model.SessionResponse{
		SessionID:    session.SessionID,
		SessionToken: token.SessionToken,
		WebsocketURL: observerWebsocketURL(session.WebsocketURL),
		ExpiresAt:    token.ExpiresAt,
	}, nil
}

func observerWebsocketURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set("mode", "observe")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
