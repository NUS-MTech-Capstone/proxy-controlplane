package service

import (
	"context"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) ListRejoinCandidates(ctx context.Context, req model.RejoinCandidatesRequest) (model.RejoinCandidatesResponse, error) {
	if err := s.sts.Validate(ctx, sts.ValidateRequest{ActorGroups: req.ActorGroups,
		UserID: req.UserID,
		HostID: req.HostID,
		CRID:   req.CRID,
		Action: "rejoin",
	}); err != nil {
		return model.RejoinCandidatesResponse{}, err
	}
	sessions, err := s.sm.ListSessions(ctx, session_manager.ListSessionsFilter{
		UserID: req.UserID,
		HostID: req.HostID,
		CRID:   req.CRID,
		Status: "ACTIVE",
	})
	if err != nil {
		return model.RejoinCandidatesResponse{}, err
	}
	candidates := make([]model.SessionCandidate, 0, len(sessions))
	for _, session := range sessions {
		candidates = append(candidates, model.SessionCandidate{
			SessionID:       session.SessionID,
			UserID:          session.UserID,
			HostID:          session.HostID,
			CRID:            session.CRID,
			ConnectionType:  session.ConnectionType,
			Status:          session.Status,
			CreatedAt:       session.CreatedAt,
			LastHeartbeatAt: session.LastHeartbeatAt,
			LeaseExpiresAt:  session.LeaseExpiresAt,
		})
	}
	return model.RejoinCandidatesResponse{Sessions: candidates}, nil
}
