package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services"
)

var now = time.Date(2026, 11, 22, 18, 42, 0, 0, time.UTC)

type spyNotifier struct{ excluded []string }

func (s *spyNotifier) NoteChanged(_ context.Context, _ string, _ *models.Note, exclude string) {
	s.excluded = append(s.excluded, exclude)
}

func newController() (*NotesController, *spyNotifier) {
	n := &spyNotifier{}
	svc := services.NewNotesService(repositories.NewMemoryNotesRepository(), n, func() time.Time { return now })
	return NewNotesController(svc), n
}

func decodeJSON(t *testing.T, r Response) map[string]any {
	t.Helper()
	if !strings.HasPrefix(r.Headers["Content-Type"], "application/json") {
		t.Fatalf("content-type = %q", r.Headers["Content-Type"])
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Body), &m); err != nil {
		t.Fatalf("decode %q: %v", r.Body, err)
	}
	return m
}

func put(c *NotesController, path, body string) Response {
	return c.SaveNote(context.Background(), Request{Method: "PUT", Path: path, Body: body, Headers: map[string]string{}})
}

func TestGetMissingNote(t *testing.T) {
	c, _ := newController()
	r := c.GetNote(context.Background(), Request{Path: "foo"})
	if r.Status != 200 {
		t.Fatalf("status %d", r.Status)
	}
	if r.Body != `{"path":"foo","body":"","version":0,"updatedAt":null,"expiresAt":null}` {
		t.Errorf("body = %s", r.Body)
	}
}

func TestGetInvalidPath(t *testing.T) {
	c, _ := newController()
	r := c.GetNote(context.Background(), Request{Path: "api/x"})
	m := decodeJSON(t, r)
	if r.Status != 400 || m["error"] != "invalid_path" || m["message"] == "" {
		t.Errorf("%d %v", r.Status, m)
	}
}

func TestSaveAndGet(t *testing.T) {
	c, n := newController()
	r := c.SaveNote(context.Background(), Request{Path: "faculdade/calculo", Body: `{"body":"texto","baseVersion":0}`, Headers: map[string]string{"x-connection-id": "abc="}})
	if r.Status != 200 {
		t.Fatalf("status %d: %s", r.Status, r.Body)
	}
	if r.Body != `{"path":"faculdade/calculo","version":1,"updatedAt":"2026-11-22T18:42:00Z","expiresAt":"2026-12-07T18:42:00Z"}` {
		t.Errorf("save body = %s", r.Body)
	}
	if len(n.excluded) != 1 || n.excluded[0] != "abc=" {
		t.Errorf("X-Connection-Id not forwarded: %v", n.excluded)
	}

	g := c.GetNote(context.Background(), Request{Path: "faculdade/calculo"})
	if g.Body != `{"path":"faculdade/calculo","body":"texto","version":1,"updatedAt":"2026-11-22T18:42:00Z","expiresAt":"2026-12-07T18:42:00Z"}` {
		t.Errorf("get body = %s", g.Body)
	}
}

func TestPolling(t *testing.T) {
	c, _ := newController()
	put(c, "foo", `{"body":"a"}`)

	same := c.GetNote(context.Background(), Request{Path: "foo", Query: map[string]string{"since": "1"}})
	if same.Body != `{"changed":false,"version":1}` {
		t.Errorf("unchanged = %s", same.Body)
	}
	changed := decodeJSON(t, c.GetNote(context.Background(), Request{Path: "foo", Query: map[string]string{"since": "0"}}))
	if changed["changed"] != true || changed["body"] != "a" || changed["version"] != float64(1) {
		t.Errorf("changed = %v", changed)
	}
	bad := c.GetNote(context.Background(), Request{Path: "foo", Query: map[string]string{"since": "x"}})
	if bad.Status != 400 {
		t.Errorf("since=x status %d", bad.Status)
	}
}

func TestSaveConflictResponse(t *testing.T) {
	c, _ := newController()
	put(c, "foo", `{"body":"a","baseVersion":0}`)
	put(c, "foo", `{"body":"b","baseVersion":1}`)
	r := put(c, "foo", `{"body":"c","baseVersion":1}`)
	m := decodeJSON(t, r)
	if r.Status != 409 || m["error"] != "conflict" || m["message"] == "" {
		t.Fatalf("%d %v", r.Status, m)
	}
	note := m["note"].(map[string]any)
	if note["version"] != float64(2) || note["body"] != "b" || note["path"] != "foo" {
		t.Errorf("note = %v", note)
	}

	forced := put(c, "foo", `{"body":"c","baseVersion":1,"force":true}`)
	if forced.Status != 200 || decodeJSON(t, forced)["version"] != float64(3) {
		t.Errorf("force: %d %s", forced.Status, forced.Body)
	}
}

func TestSaveDeleteResponse(t *testing.T) {
	c, _ := newController()
	put(c, "foo", `{"body":"a"}`)
	r := put(c, "foo", `{"body":"","baseVersion":1}`)
	if r.Status != 200 || r.Body != `{"path":"foo","version":0,"updatedAt":null,"expiresAt":null}` {
		t.Errorf("%d %s", r.Status, r.Body)
	}
}

func TestSaveErrors(t *testing.T) {
	c, _ := newController()
	big, _ := json.Marshal(map[string]string{"body": strings.Repeat("a", models.MaxBodyBytes+1)})
	tests := []struct {
		name, path, body string
		status           int
		code             string
	}{
		{"malformed json", "foo", `{`, 400, "invalid_body"},
		{"missing body", "foo", `{"baseVersion":1}`, 400, "invalid_body"},
		{"body not string", "foo", `{"body":1}`, 400, "invalid_body"},
		{"invalid path", "Foo", `{"body":"a"}`, 400, "invalid_path"},
		{"too large", "foo", string(big), 413, "too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := put(c, tt.path, tt.body)
			if m := decodeJSON(t, r); r.Status != tt.status || m["error"] != tt.code {
				t.Errorf("%d %v", r.Status, m)
			}
		})
	}
}

func TestChildrenResponse(t *testing.T) {
	c, _ := newController()
	put(c, "faculdade/fisica", `{"body":"a"}`)
	put(c, "faculdade/calculo", `{"body":"a"}`)
	r := c.GetChildren(context.Background(), Request{Path: "faculdade"})
	want := `{"path":"faculdade","children":[{"path":"faculdade/calculo","updatedAt":"2026-11-22T18:42:00Z"},{"path":"faculdade/fisica","updatedAt":"2026-11-22T18:42:00Z"}]}`
	if r.Status != 200 || r.Body != want {
		t.Errorf("%d %s", r.Status, r.Body)
	}
	empty := c.GetChildren(context.Background(), Request{Path: "nada"})
	if empty.Body != `{"path":"nada","children":[]}` {
		t.Errorf("empty = %s", empty.Body)
	}
	if bad := c.GetChildren(context.Background(), Request{Path: ""}); bad.Status != 400 {
		t.Errorf("empty path status %d", bad.Status)
	}
}

func TestRaw(t *testing.T) {
	c, _ := newController()
	ctx := context.Background()

	missing := c.GetRaw(ctx, Request{Path: "lista"})
	if missing.Status != 404 || missing.Body != "not found\n" || !strings.HasPrefix(missing.Headers["Content-Type"], "text/plain; charset=utf-8") {
		t.Errorf("missing: %d %q %v", missing.Status, missing.Body, missing.Headers)
	}

	saved := c.SaveRaw(ctx, Request{Path: "lista", Body: "pão, leite"})
	if saved.Status != 200 || saved.Body != "saved v1, expires 2026-12-07T18:42:00Z\n" {
		t.Errorf("saved: %d %q", saved.Status, saved.Body)
	}
	again := c.SaveRaw(ctx, Request{Path: "lista", Body: "ovos\n"})
	if again.Body != "saved v2, expires 2026-12-07T18:42:00Z\n" {
		t.Errorf("last-write-wins: %q", again.Body)
	}

	got := c.GetRaw(ctx, Request{Path: "lista"})
	if got.Status != 200 || got.Body != "ovos\n" {
		t.Errorf("get: %d %q", got.Status, got.Body)
	}
	c.SaveRaw(ctx, Request{Path: "lista", Body: "sem quebra"})
	if got := c.GetRaw(ctx, Request{Path: "lista"}); got.Body != "sem quebra\n" {
		t.Errorf("trailing newline: %q", got.Body)
	}

	tests := []struct {
		name, path, body string
		status           int
		text             string
	}{
		{"invalid path", "raw", "x", 400, "invalid path\n"},
		{"too large", "lista", strings.Repeat("a", models.MaxBodyBytes+1), 413, "too large\n"},
		{"delete", "lista", "", 200, "deleted\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := c.SaveRaw(ctx, Request{Path: tt.path, Body: tt.body})
			if r.Status != tt.status || r.Body != tt.text {
				t.Errorf("%d %q", r.Status, r.Body)
			}
		})
	}
	if r := c.GetRaw(ctx, Request{Path: "lista"}); r.Status != 404 {
		t.Errorf("deleted note status %d", r.Status)
	}
}

func TestLambdaAdapter(t *testing.T) {
	c, n := newController()
	h := LambdaHTTP(c.SaveRaw)
	resp, err := h(context.Background(), events.APIGatewayV2HTTPRequest{
		PathParameters:  map[string]string{"path": "foo"},
		Headers:         map[string]string{"X-Connection-Id": "me"},
		Body:            base64.StdEncoding.EncodeToString([]byte("olá")),
		IsBase64Encoded: true,
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%v %+v", err, resp)
	}
	get, _ := LambdaHTTP(c.GetRaw)(context.Background(), events.APIGatewayV2HTTPRequest{PathParameters: map[string]string{"path": "foo"}})
	if get.Body != "olá\n" {
		t.Errorf("body = %q", get.Body)
	}
	if n.excluded[0] != "me" {
		t.Errorf("header lookup failed: %v", n.excluded)
	}
}
