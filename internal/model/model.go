package model

import "time"

type StartSessionRequest struct {
	UserID         string `json:"-"`
	HostID         string `json:"hostId"`
	CRID           string `json:"crId"`
	ConnectionType string `json:"connectionType,omitempty"`
}

type SessionAccessRequest struct {
	UserID string `json:"-"`
}

type SessionResponse struct {
	SessionID    string    `json:"sessionId"`
	SessionToken string    `json:"sessionToken"`
	WebsocketURL string    `json:"websocketUrl"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type Session struct {
	SessionID      string `json:"sessionId"`
	UserID         string `json:"userId"`
	HostID         string `json:"hostId"`
	CRID           string `json:"crId"`
	ConnectionType string `json:"connectionType"`
	WebsocketURL   string `json:"websocketUrl"`
	Status         string `json:"status"`
}

type AutomationRunRequest struct {
	IncidentID        string   `json:"incidentId"`
	HostID            string   `json:"hostId"`
	Phase             string   `json:"phase"`
	AllowedOperations []string `json:"allowedOperations"`
	PlanHash          string   `json:"planHash,omitempty"`
	WorkloadToken     string   `json:"-"`
}

type AutomationTokenRequest struct {
	Phase             string   `json:"phase"`
	AllowedOperations []string `json:"allowedOperations"`
	PlanHash          string   `json:"planHash,omitempty"`
	WorkloadToken     string   `json:"-"`
}

type AutomationRun struct {
	RunID           string    `json:"runId"`
	IncidentID      string    `json:"incidentId"`
	HostID          string    `json:"hostId"`
	OwnerPodID      string    `json:"ownerPodId"`
	OwnerInstanceID string    `json:"ownerInstanceId"`
	OperationsURL   string    `json:"operationsUrl"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type AutomationRunResponse struct {
	AutomationRun
	CapabilityToken string    `json:"capabilityToken"`
	TokenExpiresAt  time.Time `json:"tokenExpiresAt"`
}
