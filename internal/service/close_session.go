package service

import (
	"context"
	"errors"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) CloseSession(
	ctx context.Context,
	sessionID string,
	req model.SessionAccessRequest,
	privilegedStop bool,
) (model.SessionCandidate, error) {
	if err := s.sts.Validate(ctx, sts.ValidateRequest{
		UserID: req.UserID, SessionID: sessionID, Action: "stop",
	}); err != nil {
		return model.SessionCandidate{}, err
	}

	session, err := s.sm.GetSession(ctx, sessionID)
	if err != nil {
		return model.SessionCandidate{}, err
	}
	if session.Status != "ACTIVE" {
		return model.SessionCandidate{}, errors.New("session is not active")
	}
	if session.UserID != req.UserID && !privilegedStop {
		return model.SessionCandidate{}, errors.New("authenticated user is not allowed to stop the session")
	}
	if session.OwnerPodID == "" || session.OwnerInstanceID == "" {
		return model.SessionCandidate{}, errors.New("session owner metadata is unavailable")
	}

	closed, err := s.sm.CloseSession(ctx, sessionID, session.OwnerPodID, session.OwnerInstanceID)
	if err != nil {
		return model.SessionCandidate{}, err
	}
	return sessionCandidateFrom(closed), nil
}
