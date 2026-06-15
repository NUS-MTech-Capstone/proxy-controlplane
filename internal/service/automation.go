package service

import (
	"context"
	"errors"

	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/model"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

func (s *Service) StartAutomationRun(ctx context.Context, req model.AutomationRunRequest) (model.AutomationRunResponse, error) {
	if req.Phase != "diagnose" {
		return model.AutomationRunResponse{}, errors.New("new automation runs must start in diagnose phase")
	}
	run, err := s.sm.CreateAutomationRun(ctx, req.IncidentID, req.HostID)
	if err != nil {
		return model.AutomationRunResponse{}, err
	}
	token, err := s.issueAutomationToken(ctx, run, req.WorkloadToken, req.Phase, req.AllowedOperations, req.PlanHash)
	if err != nil {
		_ = s.sm.CloseAutomationRun(ctx, run.RunID)
		return model.AutomationRunResponse{}, err
	}
	return model.AutomationRunResponse{
		AutomationRun: run, CapabilityToken: token.CapabilityToken, TokenExpiresAt: token.ExpiresAt,
	}, nil
}

func (s *Service) IssueAutomationToken(ctx context.Context, runID string, req model.AutomationTokenRequest) (model.AutomationRunResponse, error) {
	run, err := s.sm.GetAutomationRun(ctx, runID)
	if err != nil {
		return model.AutomationRunResponse{}, err
	}
	if run.Status != "ACTIVE" {
		return model.AutomationRunResponse{}, errors.New("automation run is not active")
	}
	token, err := s.issueAutomationToken(ctx, run, req.WorkloadToken, req.Phase, req.AllowedOperations, req.PlanHash)
	if err != nil {
		return model.AutomationRunResponse{}, err
	}
	return model.AutomationRunResponse{
		AutomationRun: run, CapabilityToken: token.CapabilityToken, TokenExpiresAt: token.ExpiresAt,
	}, nil
}

func (s *Service) CloseAutomationRun(ctx context.Context, runID, capabilityToken string) error {
	if err := s.sts.ValidateAutomationToken(ctx, runID, capabilityToken); err != nil {
		return err
	}
	return s.sm.CloseAutomationRun(ctx, runID)
}

func (s *Service) issueAutomationToken(
	ctx context.Context,
	run model.AutomationRun,
	workloadToken, phase string,
	operations []string,
	planHash string,
) (sts.AutomationTokenResponse, error) {
	return s.sts.RequestAutomationToken(ctx, workloadToken, sts.AutomationTokenRequest{
		RunID: run.RunID, IncidentID: run.IncidentID, HostID: run.HostID,
		Phase: phase, AllowedOperations: operations, PlanHash: planHash,
		OwnerPodID: run.OwnerPodID, OwnerInstanceID: run.OwnerInstanceID,
	})
}
