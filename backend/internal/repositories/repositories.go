package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

// ErrConditionFailed means the write condition (version or expiry) did not hold.
var ErrConditionFailed = errors.New("condition failed")

// WriteCondition is applied to puts and deletes unless Force is set:
// the item must not exist, be expired at Now, or have Version equal to BaseVersion.
type WriteCondition struct {
	BaseVersion int64
	Now         time.Time
	Force       bool
}

type NotesRepository interface {
	// Get returns the stored note (expired or not), or nil when there is none.
	Get(ctx context.Context, path string) (*models.Note, error)
	Put(ctx context.Context, note models.Note, cond WriteCondition) error
	Delete(ctx context.Context, path string, cond WriteCondition) error
	// ListChildren returns direct, non-expired children sorted by path.
	ListChildren(ctx context.Context, parent string, now time.Time) ([]models.ChildNote, error)
	CountActive(ctx context.Context, now time.Time) (int, error)
	CountUpdatedSince(ctx context.Context, since time.Time) (int, error)
	// ListRecent returns up to limit non-expired notes, most recently updated first.
	ListRecent(ctx context.Context, limit int, now time.Time) ([]models.RecentNote, error)
}

type ConnectionsRepository interface {
	Put(ctx context.Context, conn models.Connection) error
	Get(ctx context.Context, connectionID string) (*models.Connection, error)
	Delete(ctx context.Context, connectionID string) error
	// ListByPath returns the non-expired connections of a note path.
	ListByPath(ctx context.Context, path string, now time.Time) ([]models.Connection, error)
}

func conditionHolds(current *models.Note, cond WriteCondition) bool {
	if cond.Force || current == nil {
		return true
	}
	return current.ExpiresAt.Unix() < cond.Now.Unix() || current.Version == cond.BaseVersion
}
