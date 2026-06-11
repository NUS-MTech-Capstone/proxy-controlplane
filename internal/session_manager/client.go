package session_manager

import (
	"net/http"
	"time"
)

type SessionManagerClient struct {
	baseURL string
	client  *http.Client
}

func NewSessionManager(baseURL string) *SessionManagerClient {
	return &SessionManagerClient{baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}
