package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

func connect(t *testing.T, repo *repositories.MemoryConnectionsRepository, path string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		_ = repo.Put(context.Background(), models.Connection{ConnectionID: id, Path: path, ConnectedAt: baseTime, ExpiresAt: baseTime.Add(models.ConnectionTTL)})
	}
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return m
}

func liveNote(body string) *models.Note {
	return &models.Note{Path: "foo", Body: body, Version: 8, UpdatedAt: baseTime, ExpiresAt: baseTime.Add(models.NoteTTL)}
}

func TestNoteMessage(t *testing.T) {
	tests := []struct {
		name     string
		note     *models.Note
		wantType string
		wantBody bool
	}{
		{"small note sends update", liveNote("oi"), "update", true},
		{"note at limit sends update", liveNote(strings.Repeat("a", models.MaxInlineBytes)), "update", true},
		{"large note sends changed", liveNote(strings.Repeat("a", models.MaxInlineBytes+1)), "changed", false},
		{"deleted note sends update v0", nil, "update", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := decode(t, string(NoteMessage("foo", tt.note)))
			if m["type"] != tt.wantType || m["path"] != "foo" {
				t.Fatalf("message = %v", m)
			}
			if _, ok := m["body"]; ok != tt.wantBody {
				t.Errorf("body present = %v, want %v", ok, tt.wantBody)
			}
			if tt.note == nil {
				if m["version"] != float64(0) || m["updatedAt"] != nil || m["expiresAt"] != nil || m["body"] != "" {
					t.Errorf("deleted message = %v", m)
				}
				for _, k := range []string{"updatedAt", "expiresAt"} {
					if _, ok := m[k]; !ok {
						t.Errorf("%s must be present as null", k)
					}
				}
			} else if m["version"] != float64(8) {
				t.Errorf("version = %v", m["version"])
			}
		})
	}
}

func TestNoteChangedSkipsAuthorAndCleansGone(t *testing.T) {
	repo := repositories.NewMemoryConnectionsRepository()
	connect(t, repo, "foo", "author", "a", "b", "gone")
	connect(t, repo, "bar", "other")
	b := &fakeBroadcaster{gone: map[string]bool{"gone": true}}
	svc := NewBroadcastService(repo, b, (&fakeClock{t: baseTime}).Now)

	svc.NoteChanged(context.Background(), "foo", liveNote("oi"), "author")

	if len(b.to("author")) != 0 || len(b.to("other")) != 0 {
		t.Errorf("author or other path received a message: %+v", b.posts)
	}
	for _, id := range []string{"a", "b"} {
		msgs := b.to(id)
		if len(msgs) != 1 || decode(t, msgs[0])["type"] != "update" {
			t.Errorf("%s got %v", id, msgs)
		}
	}
	if c, _ := repo.Get(context.Background(), "gone"); c != nil {
		t.Errorf("gone connection was not deleted")
	}
}

func TestNoteChangedLargeBodySendsChanged(t *testing.T) {
	repo := repositories.NewMemoryConnectionsRepository()
	connect(t, repo, "foo", "a")
	b := &fakeBroadcaster{}
	NewBroadcastService(repo, b, (&fakeClock{t: baseTime}).Now).NoteChanged(context.Background(), "foo", liveNote(strings.Repeat("x", 30_000)), "")
	m := decode(t, b.to("a")[0])
	if m["type"] != "changed" || m["version"] != float64(8) {
		t.Errorf("got %v", m)
	}
}

func TestNoteChangedIgnoresExpiredConnections(t *testing.T) {
	repo := repositories.NewMemoryConnectionsRepository()
	connect(t, repo, "foo", "old")
	b := &fakeBroadcaster{}
	later := baseTime.Add(3 * time.Hour)
	NewBroadcastService(repo, b, func() time.Time { return later }).NoteChanged(context.Background(), "foo", liveNote("x"), "")
	if len(b.posts) != 0 {
		t.Errorf("expired connection received %v", b.posts)
	}
}

func TestBroadcastConcurrencyLimit(t *testing.T) {
	repo := repositories.NewMemoryConnectionsRepository()
	for i := range 40 {
		connect(t, repo, "foo", fmt.Sprintf("c%02d", i))
	}
	b := &slowBroadcaster{}
	NewBroadcastService(repo, b, (&fakeClock{t: baseTime}).Now).NoteChanged(context.Background(), "foo", liveNote("x"), "")
	if b.peak > broadcastConcurrency || b.peak < 2 || b.total != 40 {
		t.Errorf("peak %d, total %d", b.peak, b.total)
	}
}

type slowBroadcaster struct {
	mu                   sync.Mutex
	current, peak, total int
}

func (b *slowBroadcaster) Post(context.Context, string, []byte) error {
	b.mu.Lock()
	b.current++
	b.total++
	b.peak = max(b.peak, b.current)
	b.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	b.mu.Lock()
	b.current--
	b.mu.Unlock()
	return nil
}
