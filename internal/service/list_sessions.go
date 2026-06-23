package service

import (
	"context"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
)

func (s *Service) ListSessions(ctx context.Context, req model.ListSessionsRequest) (model.ListSessionsResponse, error) {
	status := req.Status
	if status == "" {
		status = "ACTIVE"
	}
	userID := ""
	if req.Mine {
		userID = req.UserID
	}
	sessions, err := s.sm.ListSessions(ctx, session_manager.ListSessionsFilter{
		UserID: userID,
		HostID: req.HostID,
		CRID:   req.CRID,
		Status: status,
	})
	if err != nil {
		return model.ListSessionsResponse{}, err
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
	return model.ListSessionsResponse{Sessions: candidates}, nil
}
