package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/observability"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) StartSession(ctx context.Context, req model.StartSessionRequest) (model.SessionResponse, error) {
	if req.ConnectionType == "" {
		req.ConnectionType = "ssh"
	}
	if err := s.sts.Validate(ctx, sts.ValidateRequest{ActorGroups: req.ActorGroups, UserID: req.UserID, HostID: req.HostID, CRID: req.CRID, Action: "start"}); err != nil {
		recordSessionStartFailed(ctx, req, "sts_validate", err)
		return model.SessionResponse{}, err
	}
	session, err := s.sm.CreateSession(ctx, session_manager.CreateSessionRequest{
		UserID: req.UserID, HostID: req.HostID, CRID: req.CRID, ConnectionType: req.ConnectionType,
	})
	if err != nil {
		recordSessionStartFailed(ctx, req, "assignment", err)
		return model.SessionResponse{}, err
	}
	token, err := s.sts.RequestToken(ctx, sts.TokenRequest{ActorGroups: req.ActorGroups,
		SessionID: session.SessionID, UserID: req.UserID, HostID: session.HostID, CRID: session.CRID,
		ConnectionType: session.ConnectionType, Action: "start", Mode: "owner",
	})
	if err != nil {
		recordSessionStartFailed(ctx, req, "token", err)
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if session.OwnerPodID != "" && session.OwnerInstanceID != "" {
			if _, closeErr := s.sm.CloseSession(cleanupCtx, session.SessionID, session.OwnerPodID, session.OwnerInstanceID); closeErr != nil {
				slog.WarnContext(ctx, "SessionAssignmentCleanupFailed", "sessionId", session.SessionID, "errorKind", observability.ErrorKind(closeErr))
			}
		} else {
			slog.WarnContext(ctx, "SessionAssignmentCleanupFailed", "sessionId", session.SessionID, "errorKind", "missing_ownership")
		}
		return model.SessionResponse{}, err
	}

	return model.SessionResponse{SessionID: session.SessionID, SessionToken: token.SessionToken, WebsocketURL: session.WebsocketURL, ExpiresAt: token.ExpiresAt}, nil
}

func recordSessionStartFailed(ctx context.Context, req model.StartSessionRequest, stage string, err error) {
	observability.EmitCount("proxy-controlplane", "SessionStartFailures", map[string]string{
		"Component": "session_start",
		"Reason":    stage,
	})
	slog.WarnContext(ctx, "SessionStartFailed",
		"requestId", observability.RequestID(ctx),
		"userId", req.UserID,
		"hostId", req.HostID,
		"crId", req.CRID,
		"stage", stage,
		"errorKind", observability.ErrorKind(err),
		"error", err,
	)
}
