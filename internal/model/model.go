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
	SessionID       string    `json:"sessionId"`
	UserID          string    `json:"userId"`
	HostID          string    `json:"hostId"`
	CRID            string    `json:"crId"`
	ConnectionType  string    `json:"connectionType"`
	OwnerPodID      string    `json:"ownerPodId,omitempty"`
	OwnerInstanceID string    `json:"ownerInstanceId,omitempty"`
	WebsocketURL    string    `json:"websocketUrl,omitempty"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
	LeaseExpiresAt  time.Time `json:"leaseExpiresAt"`
}

type RejoinCandidatesRequest struct {
	UserID string `json:"-"`
	HostID string `json:"hostId"`
	CRID   string `json:"crId"`
}

type SessionCandidate struct {
	SessionID       string    `json:"sessionId"`
	UserID          string    `json:"userId,omitempty"`
	HostID          string    `json:"hostId"`
	CRID            string    `json:"crId"`
	ConnectionType  string    `json:"connectionType"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	LastHeartbeatAt time.Time `json:"lastHeartbeatAt"`
	LeaseExpiresAt  time.Time `json:"leaseExpiresAt"`
}

type ListSessionsRequest struct {
	UserID string `json:"-"`
	HostID string `json:"hostId,omitempty"`
	CRID   string `json:"crId,omitempty"`
	Status string `json:"status,omitempty"`
	Mine   bool   `json:"mine,omitempty"`
}

type ListSessionsResponse struct {
	Sessions []SessionCandidate `json:"sessions"`
}

type RejoinCandidatesResponse struct {
	Sessions []SessionCandidate `json:"sessions"`
}
