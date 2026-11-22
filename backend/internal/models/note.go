package models

import "time"

const (
	NoteKind       = "NOTE"
	NoteTTL        = 15 * 24 * time.Hour
	MaxBodyBytes   = 100_000
	MaxInlineBytes = 28_000
)

type Note struct {
	Path      string
	Parent    string
	Body      string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time
}

// IsExpired follows the storage condition `expiresAt < :now`, both in epoch seconds.
func (n *Note) IsExpired(now time.Time) bool {
	return n.ExpiresAt.Unix() < now.Unix()
}

// IsLive reports whether the note exists and has not expired.
func (n *Note) IsLive(now time.Time) bool {
	return n != nil && !n.IsExpired(now)
}

type ChildNote struct {
	Path      string
	UpdatedAt time.Time
}

type RecentNote struct {
	Path      string
	UpdatedAt time.Time
}

// SaveResult is the outcome of a save: Note is nil when the body was empty and the note deleted.
type SaveResult struct {
	Path    string
	Note    *Note
	Deleted bool
}
