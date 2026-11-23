package report

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"time"
	_ "time/tzdata"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/external/ses"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/helpers"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/repositories"
)

const (
	RecentLimit  = 10
	AvatarCID    = "cortex@lauraesteves.com"
	reportWindow = 7 * 24 * time.Hour
	senderName   = "Cortex"
)

//go:embed templates/weekly_report.html.tmpl
var htmlSource string

//go:embed templates/weekly_report.txt.tmpl
var textSource string

//go:embed templates/cortex_bot_192.png
var Avatar []byte

var (
	htmlTemplate = htmltemplate.Must(htmltemplate.New("weekly_report.html").Parse(htmlSource))
	textTemplate = texttemplate.Must(texttemplate.New("weekly_report.txt").Parse(textSource))
	Location     = mustLoadLocation("America/Sao_Paulo")
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// Count prints in pt-BR format but stays falsy at zero inside templates.
type Count int

func (c Count) String() string { return helpers.FormatNumberBR(int(c)) }

type RecentNote struct {
	Path     string
	EditedAt string
}

type Data struct {
	Subject    string
	Preview    string
	AvatarSrc  htmltemplate.URL
	Now        string
	Total      Count
	EditedWeek Count
	Recent     []RecentNote
	SiteURL    string
	Host       string
}

type Config struct {
	From    string
	To      string
	SiteURL string
	Host    string
}

type Service struct {
	notes  repositories.NotesRepository
	sender ses.EmailSender
	cfg    Config
	now    func() time.Time
}

func NewService(notes repositories.NotesRepository, sender ses.EmailSender, cfg Config, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{notes: notes, sender: sender, cfg: cfg, now: now}
}

// Build collects the report numbers and the latest edited notes.
func (s *Service) Build(ctx context.Context) (Data, error) {
	now := s.now()
	total, err := s.notes.CountActive(ctx, now)
	if err != nil {
		return Data{}, fmt.Errorf("count active notes: %w", err)
	}
	edited, err := s.notes.CountUpdatedSince(ctx, now.Add(-reportWindow))
	if err != nil {
		return Data{}, fmt.Errorf("count edited notes: %w", err)
	}
	recent, err := s.notes.ListRecent(ctx, RecentLimit, now)
	if err != nil {
		return Data{}, fmt.Errorf("list recent notes: %w", err)
	}
	data := NewData(now, total, edited, s.cfg.SiteURL, s.cfg.Host)
	for _, n := range recent {
		data.Recent = append(data.Recent, RecentNote{Path: n.Path, EditedAt: n.UpdatedAt.In(Location).Format("02/01 15:04")})
	}
	return data, nil
}

// NewData fills the template fields that do not depend on the note list.
func NewData(now time.Time, total, edited int, siteURL, host string) Data {
	local := now.In(Location)
	return Data{
		Subject:    local.Format("2006-01-02") + " Notas da Semana",
		Preview:    preview(total, edited),
		AvatarSrc:  htmltemplate.URL("cid:" + AvatarCID),
		Now:        local.Format("02/01/2006 15:04"),
		Total:      Count(total),
		EditedWeek: Count(edited),
		SiteURL:    strings.TrimRight(siteURL, "/"),
		Host:       host,
	}
}

func preview(total, edited int) string {
	notes := "notas"
	if total == 1 {
		notes = "nota"
	}
	editedWord := "editadas"
	if edited == 1 {
		editedWord = "editada"
	}
	return fmt.Sprintf("%s %s no banco, %s %s na semana", helpers.FormatNumberBR(total), notes, helpers.FormatNumberBR(edited), editedWord)
}

// Render returns the HTML and plain text bodies.
func Render(data Data) (string, string, error) {
	var html, text bytes.Buffer
	if err := htmlTemplate.Execute(&html, data); err != nil {
		return "", "", fmt.Errorf("render html: %w", err)
	}
	if err := textTemplate.Execute(&text, data); err != nil {
		return "", "", fmt.Errorf("render text: %w", err)
	}
	return html.String(), text.String(), nil
}

// BuildEmail renders the report into an email with the avatar attached inline.
func (s *Service) BuildEmail(data Data) (ses.Email, error) {
	html, text, err := Render(data)
	if err != nil {
		return ses.Email{}, err
	}
	return ses.Email{
		From:    fmt.Sprintf("%s <%s>", senderName, s.cfg.From),
		To:      []string{s.cfg.To},
		Subject: data.Subject,
		HTML:    html,
		Text:    text,
		Inline: []ses.InlineImage{{
			ContentID:   AvatarCID,
			ContentType: "image/png",
			Data:        Avatar,
		}},
	}, nil
}

func (s *Service) Send(ctx context.Context) error {
	data, err := s.Build(ctx)
	if err != nil {
		return err
	}
	email, err := s.BuildEmail(data)
	if err != nil {
		return err
	}
	return s.sender.Send(ctx, email)
}
