package model

import "time"

type StartSessionRequest struct {
	UserID string `json:"-"`
	HostID string `json:"hostId"`
	CRID   string `json:"crId"`
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
	SessionID    string `json:"sessionId"`
	UserID       string `json:"userId"`
	HostID       string `json:"hostId"`
	CRID         string `json:"crId"`
	WebsocketURL string `json:"websocketUrl"`
	Status       string `json:"status"`
}
