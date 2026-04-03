package ui

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/log"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type markdown struct {
	// Full path of a local markdown file. Only relevant to local documents and
	// those that have been stashed in this session.
	localPath string

	// Value we filter against. This exists so that we can maintain positions
	// of filtered items if notes are edited while a filter is active. This
	// field is ephemeral, and should only be referenced during filtering.
	filterValue string

	// The line from the body that best matched the filter query. Ephemeral,
	// only meaningful during active filtering. Empty when match is title-only.
	matchContext string

	Body    string
	Note    string
	Modtime time.Time
}

// Generate the value we're doing to filter against.
func (m *markdown) buildFilterValue() {
	note, err := normalize(m.Note)
	if err != nil {
		log.Error("error normalizing", "note", m.Note, "error", err)
		note = m.Note
	}

	if m.Body == "" {
		m.filterValue = note
		return
	}

	body, err := normalize(m.Body)
	if err != nil {
		log.Error("error normalizing", "body_len", len(m.Body), "error", err)
		body = m.Body
	}

	m.filterValue = note + "\n" + body
}

// extractMatchContext finds the best matching line from the body for the given
// query. Returns empty string if no body line matches.
func extractMatchContext(md *markdown, query string) string {
	if md.Body == "" {
		return ""
	}

	lines := strings.Split(md.Body, "\n")
	ranks := fuzzy.Find(query, lines)
	if len(ranks) == 0 {
		return ""
	}

	return strings.TrimSpace(lines[ranks[0].Index])
}

func (m markdown) relativeTime() string {
	return relativeTime(m.Modtime)
}

// Normalize text to aid in the filtering process. In particular, we remove
// diacritics, "ö" becomes "o". Note that Mn is the unicode key for nonspacing
// marks.
func normalize(in string) (string, error) {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, in)
	if err != nil {
		return "", fmt.Errorf("error normalizing: %w", err)
	}
	return out, nil
}

// Return the time in a human-readable format relative to the current time.
func relativeTime(then time.Time) string {
	now := time.Now()
	if ago := now.Sub(then); ago < time.Minute {
		return "just now"
	} else if ago < humanize.Week {
		return humanize.CustomRelTime(then, now, "ago", "from now", magnitudes)
	}
	return then.Format("02 Jan 2006 15:04 MST")
}

// Magnitudes for relative time.
var magnitudes = []humanize.RelTimeMagnitude{
	{D: time.Second, Format: "now", DivBy: time.Second},
	{D: 2 * time.Second, Format: "1 second %s", DivBy: 1},
	{D: time.Minute, Format: "%d seconds %s", DivBy: time.Second},
	{D: 2 * time.Minute, Format: "1 minute %s", DivBy: 1},
	{D: time.Hour, Format: "%d minutes %s", DivBy: time.Minute},
	{D: 2 * time.Hour, Format: "1 hour %s", DivBy: 1},
	{D: humanize.Day, Format: "%d hours %s", DivBy: time.Hour},
	{D: 2 * humanize.Day, Format: "1 day %s", DivBy: 1},
	{D: humanize.Week, Format: "%d days %s", DivBy: humanize.Day},
	{D: 2 * humanize.Week, Format: "1 week %s", DivBy: 1},
	{D: humanize.Month, Format: "%d weeks %s", DivBy: humanize.Week},
	{D: 2 * humanize.Month, Format: "1 month %s", DivBy: 1},
	{D: humanize.Year, Format: "%d months %s", DivBy: humanize.Month},
	{D: 18 * humanize.Month, Format: "1 year %s", DivBy: 1},
	{D: 2 * humanize.Year, Format: "2 years %s", DivBy: 1},
	{D: humanize.LongTime, Format: "%d years %s", DivBy: humanize.Year},
	{D: math.MaxInt64, Format: "a long while %s", DivBy: 1},
}
