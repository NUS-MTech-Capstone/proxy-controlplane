package service

import (
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/session_manager"
	"github.com/NUS-MTech-Capstone/proxy-controlplane/internal/sts"
)

type Service struct {
	sts *sts.Client
	sm  *session_manager.SessionManagerClient
}

func New(sts *sts.Client, sm *session_manager.SessionManagerClient) *Service {
	return &Service{sts: sts, sm: sm}
}
