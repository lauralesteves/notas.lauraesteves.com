package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/helpers"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

const MaxConnectionsPerPath = 50

type ConnectionsService interface {
	Connect(ctx context.Context, connectionID, path string) error
	Disconnect(ctx context.Context, connectionID string) error
	HandleMessage(ctx context.Context, connectionID, body string) error
}

type connectionsService struct {
	conns     repositories.ConnectionsRepository
	broadcast *BroadcastService
	now       func() time.Time
}

func NewConnectionsService(conns repositories.ConnectionsRepository, broadcast *BroadcastService, now func() time.Time) ConnectionsService {
	if now == nil {
		now = time.Now
	}
	return &connectionsService{conns: conns, broadcast: broadcast, now: now}
}

func (s *connectionsService) Connect(ctx context.Context, connectionID, path string) error {
	if !helpers.ValidatePath(path) {
		return ErrInvalidPath
	}
	now := s.now().UTC()
	existing, err := s.conns.ListByPath(ctx, path, now)
	if err != nil {
		return err
	}
	if len(existing) >= MaxConnectionsPerPath {
		return ErrTooManyConnections
	}
	err = s.conns.Put(ctx, models.Connection{
		ConnectionID: connectionID,
		Path:         path,
		ConnectedAt:  now,
		ExpiresAt:    now.Add(models.ConnectionTTL),
	})
	if err != nil {
		return err
	}
	if _, err := s.broadcast.Presence(ctx, path, connectionID, true); err != nil {
		slog.Warn("presence on connect", "path", path, "error", err)
	}
	return nil
}

func (s *connectionsService) Disconnect(ctx context.Context, connectionID string) error {
	conn, err := s.conns.Get(ctx, connectionID)
	if err != nil {
		return err
	}
	if err := s.conns.Delete(ctx, connectionID); err != nil {
		return err
	}
	if conn != nil {
		if _, err := s.broadcast.Presence(ctx, conn.Path, connectionID, false); err != nil {
			slog.Warn("presence on disconnect", "path", conn.Path, "error", err)
		}
	}
	return nil
}

func (s *connectionsService) HandleMessage(ctx context.Context, connectionID, body string) error {
	var msg models.ClientMessage
	if err := json.Unmarshal([]byte(body), &msg); err != nil || msg.Action != "ping" {
		return nil
	}
	conn, err := s.conns.Get(ctx, connectionID)
	if err != nil {
		return err
	}
	count := 1
	if conn != nil {
		// Re-announcing presence on every ping lets clients that joined at the same
		// moment converge on the right count (see BroadcastService.Presence).
		if count, err = s.broadcast.Presence(ctx, conn.Path, connectionID, true); err != nil {
			return err
		}
	}
	payload, _ := json.Marshal(models.PongMessage{Type: "pong", ConnectionID: connectionID, Count: count})
	return s.broadcast.Send(ctx, connectionID, payload)
}
