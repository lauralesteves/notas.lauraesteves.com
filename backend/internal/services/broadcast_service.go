package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

const broadcastConcurrency = 10

// Broadcaster delivers a payload to one WebSocket connection. It returns ErrGone when the
// connection does not exist anymore.
type Broadcaster interface {
	Post(ctx context.Context, connectionID string, payload []byte) error
}

// Notifier is what the notes service needs to tell connected clients about a change.
type Notifier interface {
	NoteChanged(ctx context.Context, path string, note *models.Note, excludeConnectionID string)
}

type BroadcastService struct {
	conns       repositories.ConnectionsRepository
	broadcaster Broadcaster
	now         func() time.Time
}

func NewBroadcastService(conns repositories.ConnectionsRepository, broadcaster Broadcaster, now func() time.Time) *BroadcastService {
	if now == nil {
		now = time.Now
	}
	return &BroadcastService{conns: conns, broadcaster: broadcaster, now: now}
}

// NoteMessage builds the broadcast payload for a saved note (nil note means deleted).
func NoteMessage(path string, note *models.Note) []byte {
	var msg any
	switch {
	case note == nil:
		msg = models.UpdateMessage{Type: "update", Path: path}
	case len(note.Body) <= models.MaxInlineBytes:
		dto := models.NewNoteDTO(path, note)
		msg = models.UpdateMessage{Type: "update", Path: path, Version: dto.Version, Body: dto.Body, UpdatedAt: dto.UpdatedAt, ExpiresAt: dto.ExpiresAt}
	default:
		msg = models.ChangedMessage{Type: "changed", Path: path, Version: note.Version}
	}
	b, _ := json.Marshal(msg)
	return b
}

func (s *BroadcastService) NoteChanged(ctx context.Context, path string, note *models.Note, excludeConnectionID string) {
	conns, err := s.conns.ListByPath(ctx, path, s.now())
	if err != nil {
		slog.Error("list connections for broadcast", "path", path, "error", err)
		return
	}
	s.send(ctx, conns, excludeConnectionID, NoteMessage(path, note))
}

// Presence sends the connection count of path to everyone on it except actorID.
// The byPath GSI is eventually consistent, so a connection that just joined may be
// missing from the query and one that just left may still be there: actorID is
// forced in (present=true) or out (present=false) before counting.
func (s *BroadcastService) Presence(ctx context.Context, path, actorID string, present bool) (int, error) {
	conns, err := s.conns.ListByPath(ctx, path, s.now())
	if err != nil {
		return 0, err
	}
	others := make([]models.Connection, 0, len(conns))
	for _, c := range conns {
		if c.ConnectionID != actorID {
			others = append(others, c)
		}
	}
	count := len(others)
	if present {
		count++
	}
	payload, _ := json.Marshal(models.PresenceMessage{Type: "presence", Path: path, Count: count})
	s.send(ctx, others, "", payload)
	return count, nil
}

// Send posts a payload to a single connection, cleaning it up when it is gone.
func (s *BroadcastService) Send(ctx context.Context, connectionID string, payload []byte) error {
	err := s.broadcaster.Post(ctx, connectionID, payload)
	if errors.Is(err, ErrGone) {
		if delErr := s.conns.Delete(ctx, connectionID); delErr != nil {
			slog.Error("delete gone connection", "connectionId", connectionID, "error", delErr)
		}
	}
	return err
}

func (s *BroadcastService) send(ctx context.Context, conns []models.Connection, exclude string, payload []byte) {
	sem := make(chan struct{}, broadcastConcurrency)
	var wg sync.WaitGroup
	for _, c := range conns {
		if c.ConnectionID == exclude {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := s.Send(ctx, id, payload); err != nil && !errors.Is(err, ErrGone) {
				slog.Warn("post to connection", "connectionId", id, "error", err)
			}
		}(c.ConnectionID)
	}
	wg.Wait()
}
