package helpers

import (
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	seg64 := strings.Repeat("a", 64)
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"simple", "foo", true},
		{"nested", "faculdade/calculo", true},
		{"digits underscore dash", "a_b-1/2", true},
		{"five levels", "a/b/c/d/e", true},
		{"segment 64 chars", seg64, true},
		{"empty", "", false},
		{"six levels", "a/b/c/d/e/f", false},
		{"segment 65 chars", seg64 + "a", false},
		{"over 200 chars", strings.Join([]string{seg64, seg64, seg64, "abcdefghi"}, "/"), false},
		{"exactly 200 chars", strings.Join([]string{seg64, seg64, seg64, "abcdefgh"[:5]}, "/"), true},
		{"leading slash", "/foo", false},
		{"trailing slash", "foo/", false},
		{"double slash", "foo//bar", false},
		{"uppercase", "Foo", false},
		{"accent", "café", false},
		{"space", "foo bar", false},
		{"dot", "foo.txt", false},
		{"reserved api", "api", false},
		{"reserved ws nested", "ws/foo", false},
		{"reserved raw", "raw", false},
		{"reserved notes", "notes/x", false},
		{"reserved children", "children", false},
		{"reserved assets", "assets", false},
		{"reserved static", "static", false},
		{"reserved well-known", "well-known", false},
		{"reserved 404", "404", false},
		{"reserved only first segment", "foo/api", true},
		{"not reserved prefix", "apis", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePath(tt.path); got != tt.want {
				t.Errorf("ValidatePath(%q) = %v, want %v (len %d)", tt.path, got, tt.want, len(tt.path))
			}
		})
	}
}

func TestReservedSegments(t *testing.T) {
	for _, s := range []string{"api", "ws", "raw", "notes", "children", "assets", "static", "favicon.ico", "robots.txt", "sitemap.xml", "manifest.webmanifest", "humans.txt", "ai.txt", "well-known", "index.html", "404"} {
		if !IsReservedSegment(s) {
			t.Errorf("%q should be reserved", s)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Faculdade/Cálculo", "faculdade/calculo"},
		{"Lista de Compras", "lista-de-compras"},
		{"  ação  rápida ", "acao-rapida"},
		{"/foo//bar/", "foo/bar"},
		{"a!@#b$%c", "abc"},
		{"São Paulo/Coração", "sao-paulo/coracao"},
		{"já_está-ok", "ja_esta-ok"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizePath(tt.in); got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParentPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"foo", RootParent},
		{"faculdade/calculo", "faculdade"},
		{"a/b/c", "a/b"},
	}
	for _, tt := range tests {
		if got := ParentPath(tt.in); got != tt.want {
			t.Errorf("ParentPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatNumberBR(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{{0, "0"}, {7, "7"}, {999, "999"}, {1284, "1.284"}, {1000000, "1.000.000"}, {-1284, "-1.284"}}
	for _, tt := range tests {
		if got := FormatNumberBR(tt.in); got != tt.want {
			t.Errorf("FormatNumberBR(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
