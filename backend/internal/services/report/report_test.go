package report

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/external/ses"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/models"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

var now = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

type captureSender struct{ sent []ses.Email }

func (c *captureSender) Send(_ context.Context, e ses.Email) error {
	c.sent = append(c.sent, e)
	return nil
}

func seed(t *testing.T, repo *repositories.MemoryNotesRepository, path string, updated time.Time) {
	t.Helper()
	err := repo.Put(context.Background(), models.Note{Path: path, Body: "segredo", Version: 1, UpdatedAt: updated, ExpiresAt: updated.Add(models.NoteTTL)}, repositories.WriteCondition{Force: true})
	if err != nil {
		t.Fatal(err)
	}
}

func newService(repo repositories.NotesRepository, sender ses.EmailSender) *Service {
	return NewService(repo, sender, Config{From: "cortex@lauraesteves.com", To: "laura@example.com", SiteURL: "https://notas.lauraesteves.com", Host: "notas-weekly-report"}, func() time.Time { return now })
}

func TestBuildWithNotes(t *testing.T) {
	repo := repositories.NewMemoryNotesRepository()
	for i := range 12 {
		seed(t, repo, "nota-"+string(rune('a'+i)), now.Add(-time.Duration(i)*time.Hour))
	}
	seed(t, repo, "antiga", now.Add(-10*24*time.Hour))
	seed(t, repo, "vencida", now.Add(-20*24*time.Hour))

	data, err := newService(repo, nil).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if data.Total != 13 || data.EditedWeek != 12 || len(data.Recent) != 10 {
		t.Fatalf("total %d, edited %d, recent %d", data.Total, data.EditedWeek, len(data.Recent))
	}
	if data.Recent[0].Path != "nota-a" || data.Recent[0].EditedAt != "23/11 09:00" {
		t.Errorf("first recent = %+v", data.Recent[0])
	}
	if data.Subject != "Servidores | Notas da Semana: 2026-11-23" || data.Now != "23/11/2026 09:00" {
		t.Errorf("subject %q, now %q", data.Subject, data.Now)
	}
}

func TestRenderWithNotes(t *testing.T) {
	data := NewData(now, 1284, 37, "https://notas.lauraesteves.com/", "notas-weekly-report")
	data.Recent = []RecentNote{{Path: "faculdade/calculo", EditedAt: "22/11 18:42"}}
	html, text, err := Render(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<title>Servidores | Notas da Semana: 2026-11-23</title>`,
		`src="https://avatar.lauraesteves.com/cortex.webp"`,
		`>1.284</p>`,
		`color:#00f0f9;">37</p>`,
		`href="https://notas.lauraesteves.com/faculdade/calculo"`,
		`22/11 18:42`,
		`Enviado em 23/11/2026 09:00 por notas-weekly-report`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Contains(html, "Nenhuma nota editada") {
		t.Errorf("html shows empty state")
	}
	for _, want := range []string{"Notas no banco:      1.284", "Editadas na semana:  37", "  22/11 18:42  /faculdade/calculo"} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q:\n%s", want, text)
		}
	}
}

func TestRenderEmpty(t *testing.T) {
	html, text, err := Render(NewData(now, 0, 0, "https://notas.lauraesteves.com", "h"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Nenhuma nota editada nesta semana.") || !strings.Contains(text, "Nenhuma nota editada nesta semana.") {
		t.Errorf("empty state missing")
	}
	if !strings.Contains(html, `color:#ffffff;">0</p>`) {
		t.Errorf("zero edited should not be highlighted")
	}
	if strings.Contains(html, "<th") {
		t.Errorf("table rendered for empty report")
	}
}

func TestSendBuildsEmailWithAvatarByURL(t *testing.T) {
	repo := repositories.NewMemoryNotesRepository()
	seed(t, repo, "foo", now.Add(-time.Hour))
	sender := &captureSender{}
	if err := newService(repo, sender).Send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d", len(sender.sent))
	}
	e := sender.sent[0]
	if e.From != "Cortex <cortex@lauraesteves.com>" || e.To[0] != "laura@example.com" || e.Subject != "Servidores | Notas da Semana: 2026-11-23" {
		t.Errorf("email header = %q %v %q", e.From, e.To, e.Subject)
	}
	if strings.Contains(e.HTML, "segredo") || strings.Contains(e.Text, "segredo") {
		t.Errorf("note body leaked into the email")
	}

	in, err := ses.BuildInput(e, now)
	if err != nil {
		t.Fatal(err)
	}
	if in.Content.Simple != nil || in.Content.Raw == nil {
		t.Fatalf("expected raw content, got %+v", in.Content)
	}
	assertNoEmbeddedImage(t, in.Content.Raw.Data)
}

// assertNoEmbeddedImage checks the message is only text + HTML, with the avatar
// loaded by URL: Gmail lists any embedded image part as a "noname" attachment.
func assertNoEmbeddedImage(t *testing.T, raw []byte) {
	t.Helper()
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := msg.Header.Get("Subject"); got != "Servidores | Notas da Semana: 2026-11-23" {
		t.Errorf("subject = %q", got)
	}
	mt, params, _ := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if mt != "multipart/alternative" {
		t.Fatalf("top content type = %q", mt)
	}
	alt := multipart.NewReader(msg.Body, params["boundary"])
	var types []string
	for {
		p, err := alt.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		mt, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		types = append(types, mt)
		body, _ := io.ReadAll(quotedprintable.NewReader(p))
		if mt == "text/html" && !strings.Contains(string(body), `src="`+AvatarURL+`"`) {
			t.Errorf("html part missing the avatar URL")
		}
	}
	if strings.Join(types, ",") != "text/plain,text/html" {
		t.Errorf("parts = %v, want text/plain,text/html", types)
	}
}
