package models

import "time"

// Endpoint represents a webhook relay endpoint.
// Each endpoint has a unique handle that forms the public URL: POST /hook/<handle>
// Incoming webhooks to that URL are forwarded to TargetURL.
type Endpoint struct {
	Handle      string    `json:"handle"`
	Workspace   string    `json:"workspace"`
	TargetURL   string    `json:"target_url"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Delivery represents a received webhook and its forwarding attempt.
type Delivery struct {
	Handle         string    `json:"handle"`
	EndpointHandle string    `json:"endpoint_handle"`
	Workspace      string    `json:"workspace"`
	Method         string    `json:"method"`
	Body           string    `json:"body"`
	StatusCode     int       `json:"status_code"`
	ResponseBody   string    `json:"response_body"`
	Delivered      bool      `json:"delivered"`
	CreatedAt      time.Time `json:"created_at"`
}

// Workspace represents a tenant in the system.
type Workspace struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Plan   string `json:"plan"`
}

// Token represents an auth token.
type Token struct {
	Value     string    `json:"value"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
