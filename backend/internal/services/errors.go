package services

import (
	"errors"
	"fmt"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

var (
	ErrInvalidPath        = errors.New("invalid path")
	ErrInvalidBody        = errors.New("invalid body")
	ErrTooLarge           = fmt.Errorf("body exceeds %d bytes", models.MaxBodyBytes)
	ErrTooManyConnections = errors.New("too many connections on this path")
	// ErrGone is returned by a Broadcaster when the connection no longer exists.
	ErrGone = errors.New("connection gone")
)

// ConflictError carries the current note (nil when it no longer exists).
type ConflictError struct {
	BaseVersion int64
	Current     *models.Note
}

func (e *ConflictError) Error() string {
	var current int64
	if e.Current != nil {
		current = e.Current.Version
	}
	return fmt.Sprintf("baseVersion %d does not match current version %d", e.BaseVersion, current)
}
