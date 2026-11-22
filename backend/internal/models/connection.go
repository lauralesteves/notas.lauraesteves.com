package models

import "time"

const ConnectionTTL = 2 * time.Hour

type Connection struct {
	ConnectionID string
	Path         string
	ConnectedAt  time.Time
	ExpiresAt    time.Time
}
