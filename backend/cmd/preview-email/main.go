// Command preview-email renders the weekly report with sample data into local files.
package main

import (
	"flag"
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/config"
	"github.com/lauralesteves/notas.lauraesteves.com/backend/internal/services/report"
)

var samplePaths = []string{
	"faculdade/calculo",
	"lista-de-compras",
	"faculdade/fisica/lista-3",
	"receitas/bolo-de-cenoura",
	"viagem-2026",
	"trabalho/reuniao-segunda",
	"faculdade",
	"rascunho_post",
	"jogos/copa/bolao",
	"senhas-wifi",
}

func main() {
	out := flag.String("out", ".preview", "output directory")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "cortex_bot_192.png"), report.Avatar, 0o644); err != nil {
		fail(err)
	}

	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	full := report.NewData(now, 1284, 37, config.SiteURL, "notas-weekly-report")
	for i, p := range samplePaths {
		edited := now.Add(-time.Duration(i*7+3) * time.Hour)
		full.Recent = append(full.Recent, report.RecentNote{Path: p, EditedAt: edited.In(report.Location).Format("02/01 15:04")})
	}
	empty := report.NewData(now, 0, 0, config.SiteURL, "notas-weekly-report")

	for name, data := range map[string]report.Data{"weekly_report": full, "weekly_report_empty": empty} {
		data.AvatarSrc = htmltemplate.URL("cortex_bot_192.png")
		html, text, err := report.Render(data)
		if err != nil {
			fail(err)
		}
		for ext, content := range map[string]string{".html": html, ".txt": text} {
			path := filepath.Join(*out, name+ext)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				fail(err)
			}
			fmt.Println(path)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
