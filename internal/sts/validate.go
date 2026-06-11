package sts

import (
	"context"
	"fmt"
)

type ValidateRequest struct {
	UserID    string `json:"userId"`
	HostID    string `json:"hostId,omitempty"`
	CRID      string `json:"crId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Action    string `json:"action"`
}

type ValidateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

func (c *Client) Validate(ctx context.Context, req ValidateRequest) error {
	var resp ValidateResponse
	if err := c.post(ctx, "/validate", req, &resp); err != nil {
		return err
	}
	if !resp.Allowed {
		return fmt.Errorf("access denied: %s", resp.Reason)
	}
	return nil
}
