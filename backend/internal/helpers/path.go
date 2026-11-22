package helpers

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	MaxPathLevels = 5
	MaxPathLength = 200
	RootParent    = "#root"
)

var segmentRegex = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

var reservedFirstSegments = map[string]struct{}{
	"api":                  {},
	"ws":                   {},
	"raw":                  {},
	"notes":                {},
	"children":             {},
	"assets":               {},
	"static":               {},
	"favicon.ico":          {},
	"robots.txt":           {},
	"sitemap.xml":          {},
	"manifest.webmanifest": {},
	"humans.txt":           {},
	"ai.txt":               {},
	"well-known":           {},
	"index.html":           {},
	"404":                  {},
}

// IsReservedSegment reports whether s cannot be used as the first segment of a note path.
func IsReservedSegment(s string) bool {
	_, ok := reservedFirstSegments[s]
	return ok
}

// ValidatePath reports whether path is a valid, already normalized note path.
func ValidatePath(path string) bool {
	if path == "" || len(path) > MaxPathLength {
		return false
	}
	segments := strings.Split(path, "/")
	if len(segments) > MaxPathLevels {
		return false
	}
	for _, s := range segments {
		if !segmentRegex.MatchString(s) {
			return false
		}
	}
	return !IsReservedSegment(segments[0])
}

// ParentPath returns the parent of a note path, or RootParent for top-level notes.
func ParentPath(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return RootParent
	}
	return path[:i]
}

var invalidCharRegex = regexp.MustCompile(`[^a-z0-9_/-]`)
var whitespaceRegex = regexp.MustCompile(`\s+`)
var repeatedSlashRegex = regexp.MustCompile(`/{2,}`)

// NormalizePath applies the same normalization the frontend does on a typed URL:
// lowercase, strip accents (NFD), spaces to "-", drop invalid characters and collapse slashes.
// The result may still be invalid (too long, reserved, too deep); check it with ValidatePath.
func NormalizePath(input string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	s, _, err := transform.String(t, input)
	if err != nil {
		s = input
	}
	s = strings.ToLower(s)
	s = whitespaceRegex.ReplaceAllString(strings.TrimSpace(s), "-")
	s = invalidCharRegex.ReplaceAllString(s, "")
	s = repeatedSlashRegex.ReplaceAllString(s, "/")
	return strings.Trim(s, "/")
}
