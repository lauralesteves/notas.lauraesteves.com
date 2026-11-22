package repositories

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
)

// MemoryNotesRepository is an in-memory NotesRepository used by the dev server and tests.
type MemoryNotesRepository struct {
	mu    sync.RWMutex
	notes map[string]models.Note
}

func NewMemoryNotesRepository() *MemoryNotesRepository {
	return &MemoryNotesRepository{notes: map[string]models.Note{}}
}

func (r *MemoryNotesRepository) Get(_ context.Context, path string) (*models.Note, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.notes[path]
	if !ok {
		return nil, nil
	}
	return &n, nil
}

func (r *MemoryNotesRepository) Put(_ context.Context, note models.Note, cond WriteCondition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !conditionHolds(r.current(note.Path), cond) {
		return ErrConditionFailed
	}
	r.notes[note.Path] = note
	return nil
}

func (r *MemoryNotesRepository) Delete(_ context.Context, path string, cond WriteCondition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !conditionHolds(r.current(path), cond) {
		return ErrConditionFailed
	}
	delete(r.notes, path)
	return nil
}

func (r *MemoryNotesRepository) current(path string) *models.Note {
	n, ok := r.notes[path]
	if !ok {
		return nil
	}
	return &n
}

func (r *MemoryNotesRepository) ListChildren(_ context.Context, parent string, now time.Time) ([]models.ChildNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []models.ChildNote{}
	for _, n := range r.notes {
		if n.Parent == parent && n.ExpiresAt.Unix() > now.Unix() {
			out = append(out, models.ChildNote{Path: n.Path, UpdatedAt: n.UpdatedAt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (r *MemoryNotesRepository) CountActive(_ context.Context, now time.Time) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, n := range r.notes {
		if n.ExpiresAt.Unix() > now.Unix() {
			count++
		}
	}
	return count, nil
}

func (r *MemoryNotesRepository) CountUpdatedSince(_ context.Context, since time.Time) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, n := range r.notes {
		if !n.UpdatedAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (r *MemoryNotesRepository) ListRecent(_ context.Context, limit int, now time.Time) ([]models.RecentNote, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []models.RecentNote{}
	for _, n := range r.notes {
		if n.ExpiresAt.Unix() > now.Unix() {
			out = append(out, models.RecentNote{Path: n.Path, UpdatedAt: n.UpdatedAt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// MemoryConnectionsRepository is an in-memory ConnectionsRepository used by the dev server and tests.
type MemoryConnectionsRepository struct {
	mu    sync.RWMutex
	conns map[string]models.Connection
}

func NewMemoryConnectionsRepository() *MemoryConnectionsRepository {
	return &MemoryConnectionsRepository{conns: map[string]models.Connection{}}
}

func (r *MemoryConnectionsRepository) Put(_ context.Context, conn models.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conns[conn.ConnectionID] = conn
	return nil
}

func (r *MemoryConnectionsRepository) Get(_ context.Context, connectionID string) (*models.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.conns[connectionID]
	if !ok {
		return nil, nil
	}
	return &c, nil
}

func (r *MemoryConnectionsRepository) Delete(_ context.Context, connectionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, connectionID)
	return nil
}

func (r *MemoryConnectionsRepository) ListByPath(_ context.Context, path string, now time.Time) ([]models.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []models.Connection{}
	for _, c := range r.conns {
		if c.Path == path && c.ExpiresAt.Unix() > now.Unix() {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ConnectionID < out[j].ConnectionID })
	return out, nil
}
