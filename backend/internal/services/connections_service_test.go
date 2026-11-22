package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
