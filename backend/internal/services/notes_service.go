package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/helpers"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

type SaveInput struct {
	Path         string
	Body         string
	BaseVersion  int64
	Force        bool
	ConnectionID string
}

type NotesService interface {
	// Get returns the live note, or nil when it does not exist or expired.
	Get(ctx context.Context, path string) (*models.Note, error)
	Save(ctx context.Context, in SaveInput) (models.SaveResult, error)
	Children(ctx context.Context, path string) ([]models.ChildNote, error)
}

type notesService struct {
	notes    repositories.NotesRepository
	notifier Notifier
	now      func() time.Time
}

func NewNotesService(notes repositories.NotesRepository, notifier Notifier, now func() time.Time) NotesService {
	if now == nil {
		now = time.Now
	}
	if notifier == nil {
		notifier = noopNotifier{}
	}
	return &notesService{notes: notes, notifier: notifier, now: now}
}

func (s *notesService) clock() time.Time {
	return s.now().UTC().Truncate(time.Second)
}

func (s *notesService) Get(ctx context.Context, path string) (*models.Note, error) {
	if !helpers.ValidatePath(path) {
		return nil, ErrInvalidPath
	}
	return s.getLive(ctx, path, s.clock())
}

func (s *notesService) getLive(ctx context.Context, path string, now time.Time) (*models.Note, error) {
	n, err := s.notes.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	if !n.IsLive(now) {
		return nil, nil
	}
	return n, nil
}

func (s *notesService) Children(ctx context.Context, path string) ([]models.ChildNote, error) {
	if !helpers.ValidatePath(path) {
		return nil, ErrInvalidPath
	}
	return s.notes.ListChildren(ctx, path, s.clock())
}

func (s *notesService) Save(ctx context.Context, in SaveInput) (models.SaveResult, error) {
	if !helpers.ValidatePath(in.Path) {
		return models.SaveResult{}, ErrInvalidPath
	}
	if len(in.Body) > models.MaxBodyBytes {
		return models.SaveResult{}, ErrTooLarge
	}
	if !utf8.ValidString(in.Body) || in.BaseVersion < 0 {
		return models.SaveResult{}, ErrInvalidBody
	}

	now := s.clock()
	current, err := s.notes.Get(ctx, in.Path)
	if err != nil {
		return models.SaveResult{}, err
	}
	live := current.IsLive(now)
	if live && !in.Force && in.BaseVersion != current.Version {
		return models.SaveResult{}, &ConflictError{BaseVersion: in.BaseVersion, Current: current}
	}

	cond := repositories.WriteCondition{Now: now, Force: in.Force}
	if live {
		cond.BaseVersion = current.Version
	}

	// Whitespace-only counts as empty, so a cleared note does not linger.
	if strings.TrimSpace(in.Body) == "" {
		if current == nil {
			return models.SaveResult{Path: in.Path, Deleted: true}, nil
		}
		if err := s.notes.Delete(ctx, in.Path, cond); err != nil {
			return models.SaveResult{}, s.writeError(ctx, in, now, err)
		}
		if live {
			s.notifier.NoteChanged(ctx, in.Path, nil, in.ConnectionID)
		}
		return models.SaveResult{Path: in.Path, Deleted: true}, nil
	}

	note := models.Note{
		Path:      in.Path,
		Parent:    helpers.ParentPath(in.Path),
		Body:      in.Body,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(models.NoteTTL),
	}
	if live {
		note.Version = current.Version + 1
		note.CreatedAt = current.CreatedAt
	}
	if err := s.notes.Put(ctx, note, cond); err != nil {
		return models.SaveResult{}, s.writeError(ctx, in, now, err)
	}
	s.notifier.NoteChanged(ctx, in.Path, &note, in.ConnectionID)
	return models.SaveResult{Path: in.Path, Note: &note}, nil
}

func (s *notesService) writeError(ctx context.Context, in SaveInput, now time.Time, err error) error {
	if !errors.Is(err, repositories.ErrConditionFailed) {
		return fmt.Errorf("write note: %w", err)
	}
	current, getErr := s.getLive(ctx, in.Path, now)
	if getErr != nil {
		return fmt.Errorf("reload note after conflict: %w", getErr)
	}
	return &ConflictError{BaseVersion: in.BaseVersion, Current: current}
}

type noopNotifier struct{}

func (noopNotifier) NoteChanged(context.Context, string, *models.Note, string) {}
