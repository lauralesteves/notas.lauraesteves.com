package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

func newConnFixture() (*repositories.MemoryConnectionsRepository, *fakeBroadcaster, ConnectionsService) {
	repo := repositories.NewMemoryConnectionsRepository()
	b := &fakeBroadcaster{gone: map[string]bool{}}
	clock := (&fakeClock{t: baseTime}).Now
	return repo, b, NewConnectionsService(repo, NewBroadcastService(repo, b, clock), clock)
}

func TestConnectRejectsInvalidPath(t *testing.T) {
	_, _, svc := newConnFixture()
	for _, p := range []string{"", "API", "raw/x", "a//b"} {
		if err := svc.Connect(context.Background(), "c1", p); !errors.Is(err, ErrInvalidPath) {
			t.Errorf("Connect(%q) = %v", p, err)
		}
	}
}

func TestConnectAndDisconnectBroadcastPresence(t *testing.T) {
	repo, b, svc := newConnFixture()
	ctx := context.Background()
	_ = svc.Connect(ctx, "a", "foo")
	_ = svc.Connect(ctx, "b", "foo")
	_ = svc.Connect(ctx, "x", "bar")

	if msgs := b.to("b"); len(msgs) != 0 {
		t.Errorf("joining connection should not get its own presence: %v", msgs)
	}
	msgs := b.to("a")
	if len(msgs) != 1 {
		t.Fatalf("a got %v", msgs)
	}
	if m := decode(t, msgs[0]); m["type"] != "presence" || m["count"] != float64(2) || m["path"] != "foo" {
		t.Errorf("presence = %v", m)
	}

	_ = svc.Disconnect(ctx, "b")
	msgs = b.to("a")
	if m := decode(t, msgs[len(msgs)-1]); m["count"] != float64(1) {
		t.Errorf("presence after leave = %v", m)
	}
	if c, _ := repo.Get(ctx, "b"); c != nil {
		t.Errorf("b not deleted")
	}
	if len(b.to("x")) != 0 {
		t.Errorf("other path received presence")
	}
}

func TestConnectCapPerPath(t *testing.T) {
	_, _, svc := newConnFixture()
	for i := range MaxConnectionsPerPath {
		if err := svc.Connect(context.Background(), fmt.Sprint(i), "foo"); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Connect(context.Background(), "extra", "foo"); !errors.Is(err, ErrTooManyConnections) {
		t.Errorf("got %v", err)
	}
}

func TestPingAnswersPong(t *testing.T) {
	_, b, svc := newConnFixture()
	ctx := context.Background()
	_ = svc.Connect(ctx, "a", "foo")
	_ = svc.Connect(ctx, "b=", "foo")

	if err := svc.HandleMessage(ctx, "b=", `{"action":"ping"}`); err != nil {
		t.Fatal(err)
	}
	msgs := b.to("b=")
	if len(msgs) != 1 {
		t.Fatalf("got %v", msgs)
	}
	m := decode(t, msgs[0])
	if m["type"] != "pong" || m["connectionId"] != "b=" || m["count"] != float64(2) {
		t.Errorf("pong = %v", m)
	}

	_ = svc.HandleMessage(ctx, "b=", `{"action":"other"}`)
	_ = svc.HandleMessage(ctx, "b=", `not json`)
	if len(b.to("b=")) != 1 {
		t.Errorf("unknown messages should be ignored")
	}
}

func TestPingOnGoneConnectionDeletesIt(t *testing.T) {
	repo, b, svc := newConnFixture()
	ctx := context.Background()
	_ = svc.Connect(ctx, "a", "foo")
	b.gone["a"] = true
	if err := svc.HandleMessage(ctx, "a", `{"action":"ping"}`); !errors.Is(err, ErrGone) {
		t.Errorf("got %v", err)
	}
	if c, _ := repo.Get(ctx, "a"); c != nil {
		t.Errorf("gone connection kept")
	}
}

// laggingConns simulates the eventually consistent byPath GSI: ListByPath returns
// a fixed snapshot instead of the current table contents.
type laggingConns struct {
	*repositories.MemoryConnectionsRepository
	snapshot []models.Connection
}

func (l *laggingConns) ListByPath(context.Context, string, time.Time) ([]models.Connection, error) {
	return l.snapshot, nil
}

func newLaggingFixture(snapshot ...string) (*laggingConns, *fakeBroadcaster, ConnectionsService) {
	repo := &laggingConns{MemoryConnectionsRepository: repositories.NewMemoryConnectionsRepository()}
	for _, id := range snapshot {
		c := models.Connection{ConnectionID: id, Path: "foo", ConnectedAt: baseTime, ExpiresAt: baseTime.Add(models.ConnectionTTL)}
		_ = repo.Put(context.Background(), c)
		repo.snapshot = append(repo.snapshot, c)
	}
	b := &fakeBroadcaster{gone: map[string]bool{}}
	clock := (&fakeClock{t: baseTime}).Now
	return repo, b, NewConnectionsService(repo, NewBroadcastService(repo, b, clock), clock)
}

func TestConnectCountsJoinerMissingFromIndex(t *testing.T) {
	_, b, svc := newLaggingFixture("a")
	_ = svc.Connect(context.Background(), "b", "foo")
	msgs := b.to("a")
	if len(msgs) != 1 || decode(t, msgs[0])["count"] != float64(2) {
		t.Errorf("a should see 2 people, got %v", msgs)
	}
}

func TestDisconnectIgnoresLeaverStillInIndex(t *testing.T) {
	_, b, svc := newLaggingFixture("a", "b")
	_ = svc.Disconnect(context.Background(), "b")
	if len(b.to("b")) != 0 {
		t.Errorf("leaver should not receive presence")
	}
	msgs := b.to("a")
	if len(msgs) != 1 || decode(t, msgs[0])["count"] != float64(1) {
		t.Errorf("a should see 1 person, got %v", msgs)
	}
}

func TestPingReannouncesPresenceToOthers(t *testing.T) {
	_, b, svc := newLaggingFixture("a")
	ctx := context.Background()
	_ = svc.Connect(ctx, "b", "foo")
	_ = svc.HandleMessage(ctx, "b", `{"action":"ping"}`)

	pong := b.to("b")
	if len(pong) != 1 || decode(t, pong[0])["type"] != "pong" || decode(t, pong[0])["count"] != float64(2) {
		t.Errorf("b pong = %v", pong)
	}
	msgs := b.to("a")
	if len(msgs) != 2 || decode(t, msgs[1])["type"] != "presence" || decode(t, msgs[1])["count"] != float64(2) {
		t.Errorf("a should get presence again on b's ping, got %v", msgs)
	}
}
