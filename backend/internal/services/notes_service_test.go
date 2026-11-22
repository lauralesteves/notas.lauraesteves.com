package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

type notesFixture struct {
	repo     *repositories.MemoryNotesRepository
	notifier *recordingNotifier
	clock    *fakeClock
	svc      NotesService
}

func newNotesFixture() *notesFixture {
	f := &notesFixture{
		repo:     repositories.NewMemoryNotesRepository(),
		notifier: &recordingNotifier{},
		clock:    &fakeClock{t: baseTime},
	}
	f.svc = NewNotesService(f.repo, f.notifier, f.clock.Now)
	return f
}

func (f *notesFixture) save(t *testing.T, in SaveInput) models.SaveResult {
	t.Helper()
	res, err := f.svc.Save(context.Background(), in)
	if err != nil {
		t.Fatalf("save %+v: %v", in, err)
	}
	return res
}

func TestSaveNewNote(t *testing.T) {
	f := newNotesFixture()
	res := f.save(t, SaveInput{Path: "faculdade/calculo", Body: "oi", ConnectionID: "me"})

	if res.Note == nil || res.Note.Version != 1 {
		t.Fatalf("want version 1, got %+v", res.Note)
	}
	if !res.Note.ExpiresAt.Equal(baseTime.Add(15 * 24 * time.Hour)) {
		t.Errorf("expiresAt = %v", res.Note.ExpiresAt)
	}
	stored, _ := f.repo.Get(context.Background(), "faculdade/calculo")
	if stored.Parent != "faculdade" || stored.Body != "oi" {
		t.Errorf("stored = %+v", stored)
	}
	if len(f.notifier.calls) != 1 || f.notifier.calls[0].Exclude != "me" {
		t.Errorf("notifier calls = %+v", f.notifier.calls)
	}
}

func TestSaveIncrementsVersion(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "foo", Body: "a"})
	f.clock.t = baseTime.Add(time.Hour)
	res := f.save(t, SaveInput{Path: "foo", Body: "b", BaseVersion: 1})
	if res.Note.Version != 2 {
		t.Fatalf("version = %d", res.Note.Version)
	}
	if !res.Note.CreatedAt.Equal(baseTime) || !res.Note.UpdatedAt.Equal(baseTime.Add(time.Hour)) {
		t.Errorf("dates = %+v", res.Note)
	}
	if !res.Note.ExpiresAt.Equal(baseTime.Add(time.Hour + 15*24*time.Hour)) {
		t.Errorf("expiresAt not renewed: %v", res.Note.ExpiresAt)
	}
}

func TestSaveConflict(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "foo", Body: "a"})
	f.save(t, SaveInput{Path: "foo", Body: "b", BaseVersion: 1})

	_, err := f.svc.Save(context.Background(), SaveInput{Path: "foo", Body: "c", BaseVersion: 1})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want ConflictError, got %v", err)
	}
	if conflict.Current == nil || conflict.Current.Version != 2 || conflict.Current.Body != "b" {
		t.Errorf("current = %+v", conflict.Current)
	}
	if len(f.notifier.calls) != 2 {
		t.Errorf("conflict should not broadcast")
	}
}

func TestSaveForceOverwrites(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "foo", Body: "a"})
	f.save(t, SaveInput{Path: "foo", Body: "b", BaseVersion: 1})
	res := f.save(t, SaveInput{Path: "foo", Body: "c", BaseVersion: 1, Force: true})
	if res.Note.Version != 3 || res.Note.Body != "c" {
		t.Fatalf("got %+v", res.Note)
	}
}

func TestSaveExpiredNoteRestartsAtV1(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "foo", Body: "a"})
	f.save(t, SaveInput{Path: "foo", Body: "b", BaseVersion: 1})
	f.clock.t = baseTime.Add(16 * 24 * time.Hour)

	if n, _ := f.svc.Get(context.Background(), "foo"); n != nil {
		t.Fatalf("expired note should read as nonexistent, got %+v", n)
	}
	res := f.save(t, SaveInput{Path: "foo", Body: "novo", BaseVersion: 99})
	if res.Note.Version != 1 || !res.Note.CreatedAt.Equal(f.clock.t) {
		t.Fatalf("got %+v", res.Note)
	}
}

func TestSaveEmptyBodyDeletes(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "foo", Body: "a"})
	res := f.save(t, SaveInput{Path: "foo", Body: "", BaseVersion: 1, ConnectionID: "me"})
	if !res.Deleted || res.Note != nil {
		t.Fatalf("got %+v", res)
	}
	if n, _ := f.repo.Get(context.Background(), "foo"); n != nil {
		t.Fatalf("note still stored: %+v", n)
	}
	last := f.notifier.calls[len(f.notifier.calls)-1]
	if last.Note != nil || last.Exclude != "me" {
		t.Errorf("delete broadcast = %+v", last)
	}
}

func TestSaveEmptyBodyOnMissingNoteIsNoop(t *testing.T) {
	f := newNotesFixture()
	res := f.save(t, SaveInput{Path: "foo", Body: ""})
	if !res.Deleted || len(f.notifier.calls) != 0 {
		t.Fatalf("got %+v, calls %d", res, len(f.notifier.calls))
	}
}

func TestSaveEmptyBodyConflict(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "foo", Body: "a"})
	f.save(t, SaveInput{Path: "foo", Body: "b", BaseVersion: 1})
	_, err := f.svc.Save(context.Background(), SaveInput{Path: "foo", Body: "", BaseVersion: 1})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestSaveValidation(t *testing.T) {
	f := newNotesFixture()
	tests := []struct {
		name string
		in   SaveInput
		want error
	}{
		{"invalid path", SaveInput{Path: "API", Body: "x"}, ErrInvalidPath},
		{"reserved path", SaveInput{Path: "raw", Body: "x"}, ErrInvalidPath},
		{"too large", SaveInput{Path: "foo", Body: strings.Repeat("a", models.MaxBodyBytes+1)}, ErrTooLarge},
		{"invalid utf8", SaveInput{Path: "foo", Body: "\xff"}, ErrInvalidBody},
		{"negative base", SaveInput{Path: "foo", Body: "x", BaseVersion: -1}, ErrInvalidBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.svc.Save(context.Background(), tt.in); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
	if _, err := f.svc.Save(context.Background(), SaveInput{Path: "foo", Body: strings.Repeat("a", models.MaxBodyBytes)}); err != nil {
		t.Errorf("body at the limit should be accepted: %v", err)
	}
}

// racingRepo simulates another writer winning between the read and the conditional write.
type racingRepo struct {
	*repositories.MemoryNotesRepository
	winner models.Note
}

func (r *racingRepo) Put(ctx context.Context, note models.Note, cond repositories.WriteCondition) error {
	_ = r.MemoryNotesRepository.Put(ctx, r.winner, repositories.WriteCondition{Force: true})
	return r.MemoryNotesRepository.Put(ctx, note, cond)
}

func TestSaveConditionFailedReturnsConflict(t *testing.T) {
	winner := models.Note{Path: "foo", Body: "outro", Version: 1, UpdatedAt: baseTime, ExpiresAt: baseTime.Add(models.NoteTTL)}
	repo := &racingRepo{MemoryNotesRepository: repositories.NewMemoryNotesRepository(), winner: winner}
	svc := NewNotesService(repo, &recordingNotifier{}, (&fakeClock{t: baseTime}).Now)

	_, err := svc.Save(context.Background(), SaveInput{Path: "foo", Body: "meu"})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Current == nil || conflict.Current.Body != "outro" {
		t.Fatalf("want conflict with winner, got %v", err)
	}
}

func TestChildren(t *testing.T) {
	f := newNotesFixture()
	f.save(t, SaveInput{Path: "faculdade/fisica", Body: "x"})
	f.save(t, SaveInput{Path: "faculdade/calculo", Body: "x"})
	f.save(t, SaveInput{Path: "faculdade/calculo/lista", Body: "x"})
	f.save(t, SaveInput{Path: "outra", Body: "x"})
	f.clock.t = baseTime.Add(10 * 24 * time.Hour)
	f.save(t, SaveInput{Path: "faculdade/quimica", Body: "x"})
	f.clock.t = baseTime.Add(16 * 24 * time.Hour)

	children, err := f.svc.Children(context.Background(), "faculdade")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].Path != "faculdade/quimica" {
		t.Fatalf("children = %+v", children)
	}

	f.clock.t = baseTime
	children, _ = f.svc.Children(context.Background(), "faculdade")
	var paths []string
	for _, c := range children {
		paths = append(paths, c.Path)
	}
	if strings.Join(paths, ",") != "faculdade/calculo,faculdade/fisica,faculdade/quimica" {
		t.Errorf("paths = %v", paths)
	}
}
