package services

import (
	"context"
	"sync"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

var baseTime = time.Date(2026, 11, 22, 18, 42, 0, 0, time.UTC)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }

type notification struct {
	Path    string
	Note    *models.Note
	Exclude string
}

type recordingNotifier struct {
	mu    sync.Mutex
	calls []notification
}

func (n *recordingNotifier) NoteChanged(_ context.Context, path string, note *models.Note, exclude string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, notification{path, note, exclude})
}

type post struct {
	ID      string
	Payload string
}

type fakeBroadcaster struct {
	mu    sync.Mutex
	posts []post
	gone  map[string]bool
}

func (b *fakeBroadcaster) Post(_ context.Context, id string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.gone[id] {
		return ErrGone
	}
	b.posts = append(b.posts, post{id, string(payload)})
	return nil
}

func (b *fakeBroadcaster) to(id string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, p := range b.posts {
		if p.ID == id {
			out = append(out, p.Payload)
		}
	}
	return out
}
