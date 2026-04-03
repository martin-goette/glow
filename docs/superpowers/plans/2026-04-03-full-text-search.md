# Full-Text Search with Match Context — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend glow's `/` filter to search file body contents (not just filenames) and show the matched line as context in the list.

**Architecture:** Four incremental changes: (1) eager body loading in `localFileToMarkdown`, (2) extended `filterValue` to include body text, (3) match context extraction during filtering, (4) conditional display of match context in list items.

**Tech Stack:** Go, Bubble Tea, sahilm/fuzzy

---

### File Structure

| File | Role |
|------|------|
| `ui/markdown.go` | `markdown` struct — add `matchContext` field, extend `buildFilterValue()` |
| `ui/markdown_test.go` | New — tests for `buildFilterValue()` and `extractMatchContext()` |
| `ui/ui.go` | `localFileToMarkdown()` — add eager body loading |
| `ui/stash.go` | `filterMarkdowns()` — extract match context per result |
| `ui/stashitem.go` | `stashItemView()` — render match context line when present |

---

### Task 1: Add `matchContext` field and extend `buildFilterValue()`

**Files:**
- Modify: `ui/markdown.go:16-40`
- Create: `ui/markdown_test.go`

- [ ] **Step 1: Write tests for the updated `buildFilterValue()` and new `extractMatchContext()`**

Create `ui/markdown_test.go`:

```go
package ui

import (
	"testing"
)

func TestBuildFilterValue_IncludesBody(t *testing.T) {
	md := &markdown{
		Note: "readme.md",
		Body: "# Hello World\nThis is a test document.",
	}
	md.buildFilterValue()

	// filterValue should contain both note and body
	if md.filterValue == "" {
		t.Fatal("expected filterValue to be non-empty")
	}
	if md.filterValue == "readme.md" {
		t.Error("filterValue should include body content, not just Note")
	}
}

func TestBuildFilterValue_EmptyBody(t *testing.T) {
	md := &markdown{
		Note: "readme.md",
		Body: "",
	}
	md.buildFilterValue()

	if md.filterValue == "" {
		t.Fatal("expected filterValue to be non-empty even with empty body")
	}
}

func TestExtractMatchContext_BodyMatch(t *testing.T) {
	md := &markdown{
		Note: "readme.md",
		Body: "# Title\nSome important line here\nAnother line",
	}

	ctx := extractMatchContext(md, "important")
	if ctx == "" {
		t.Error("expected match context from body, got empty string")
	}
	if ctx != "Some important line here" {
		t.Errorf("expected 'Some important line here', got %q", ctx)
	}
}

func TestExtractMatchContext_NoMatch(t *testing.T) {
	md := &markdown{
		Note: "readme.md",
		Body: "# Title\nSome line\nAnother line",
	}

	ctx := extractMatchContext(md, "zzzznotfound")
	if ctx != "" {
		t.Errorf("expected empty context for non-matching query, got %q", ctx)
	}
}

func TestExtractMatchContext_EmptyBody(t *testing.T) {
	md := &markdown{
		Note: "readme.md",
		Body: "",
	}

	ctx := extractMatchContext(md, "readme")
	if ctx != "" {
		t.Errorf("expected empty context for empty body, got %q", ctx)
	}
}
```

- [ ] **Step 2: Run the tests — they should fail (functions don't exist yet)**

Run: `cd /Users/46192/Developer/glow && go test ./ui/ -run "TestBuildFilterValue|TestExtractMatchContext" -v`
Expected: Compilation error — `extractMatchContext` is undefined.

- [ ] **Step 3: Add `matchContext` field to `markdown` struct**

In `ui/markdown.go`, change the struct definition from:

```go
type markdown struct {
	// Full path of a local markdown file. Only relevant to local documents and
	// those that have been stashed in this session.
	localPath string

	// Value we filter against. This exists so that we can maintain positions
	// of filtered items if notes are edited while a filter is active. This
	// field is ephemeral, and should only be referenced during filtering.
	filterValue string

	Body    string
	Note    string
	Modtime time.Time
}
```

to:

```go
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
```

- [ ] **Step 4: Update `buildFilterValue()` to include body content**

In `ui/markdown.go`, change `buildFilterValue()` from:

```go
func (m *markdown) buildFilterValue() {
	note, err := normalize(m.Note)
	if err != nil {
		log.Error("error normalizing", "note", m.Note, "error", err)
		m.filterValue = m.Note
	}

	m.filterValue = note
}
```

to:

```go
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
```

- [ ] **Step 5: Add `extractMatchContext()` function**

Add this function to `ui/markdown.go` (after `buildFilterValue`). It needs the `fuzzy` import and `strings` import.

Add to the import block:

```go
"strings"

"github.com/sahilm/fuzzy"
```

Add the function:

```go
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
```

- [ ] **Step 6: Run the tests — they should pass**

Run: `cd /Users/46192/Developer/glow && go test ./ui/ -run "TestBuildFilterValue|TestExtractMatchContext" -v`
Expected: All 5 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add ui/markdown.go ui/markdown_test.go
git commit -m "feat: extend filterValue to include body, add extractMatchContext"
```

---

### Task 2: Eager body loading in `localFileToMarkdown()`

**Files:**
- Modify: `ui/ui.go:426-432`

- [ ] **Step 1: Update `localFileToMarkdown()` to read file contents**

In `ui/ui.go`, change `localFileToMarkdown` from:

```go
func localFileToMarkdown(cwd string, res gitcha.SearchResult) *markdown {
	return &markdown{
		localPath: res.Path,
		Note:      stripAbsolutePath(res.Path, cwd),
		Modtime:   res.Info.ModTime(),
	}
}
```

to:

```go
func localFileToMarkdown(cwd string, res gitcha.SearchResult) *markdown {
	md := &markdown{
		localPath: res.Path,
		Note:      stripAbsolutePath(res.Path, cwd),
		Modtime:   res.Info.ModTime(),
	}

	data, err := os.ReadFile(res.Path)
	if err != nil {
		log.Debug("error reading file for search index", "path", res.Path, "error", err)
		return md
	}
	md.Body = string(data)

	return md
}
```

- [ ] **Step 2: Verify the app compiles and starts**

Run: `cd /Users/46192/Developer/glow && go build -o /dev/null .`
Expected: Builds successfully.

- [ ] **Step 3: Commit**

```bash
git add ui/ui.go
git commit -m "feat: eagerly load file bodies for full-text search"
```

---

### Task 3: Extract match context during filtering

**Files:**
- Modify: `ui/stash.go:887-910`

- [ ] **Step 1: Update `filterMarkdowns()` to set `matchContext` on each result**

In `ui/stash.go`, change `filterMarkdowns` from:

```go
func filterMarkdowns(m stashModel) tea.Cmd {
	return func() tea.Msg {
		if m.filterInput.Value() == "" || !m.filterApplied() {
			return filteredMarkdownMsg(m.markdowns) // return everything
		}

		targets := []string{}
		mds := m.markdowns

		for _, t := range mds {
			targets = append(targets, t.filterValue)
		}

		ranks := fuzzy.Find(m.filterInput.Value(), targets)
		sort.Stable(ranks)

		filtered := []*markdown{}
		for _, r := range ranks {
			filtered = append(filtered, mds[r.Index])
		}

		return filteredMarkdownMsg(filtered)
	}
}
```

to:

```go
func filterMarkdowns(m stashModel) tea.Cmd {
	return func() tea.Msg {
		if m.filterInput.Value() == "" || !m.filterApplied() {
			for _, md := range m.markdowns {
				md.matchContext = ""
			}
			return filteredMarkdownMsg(m.markdowns) // return everything
		}

		targets := []string{}
		mds := m.markdowns

		for _, t := range mds {
			targets = append(targets, t.filterValue)
		}

		query := m.filterInput.Value()
		ranks := fuzzy.Find(query, targets)
		sort.Stable(ranks)

		filtered := []*markdown{}
		for _, r := range ranks {
			md := mds[r.Index]

			// Check if any matched index falls into the body portion.
			// filterValue is structured as "normalizedNote\nnormalizedBody",
			// so find the separator to determine the boundary.
			sep := strings.Index(md.filterValue, "\n")
			bodyMatch := false
			if sep >= 0 {
				for _, idx := range r.MatchedIndexes {
					if idx > sep {
						bodyMatch = true
						break
					}
				}
			}

			if bodyMatch {
				md.matchContext = extractMatchContext(md, query)
			} else {
				md.matchContext = ""
			}

			filtered = append(filtered, md)
		}

		return filteredMarkdownMsg(filtered)
	}
}
```

- [ ] **Step 2: Verify the app compiles**

Run: `cd /Users/46192/Developer/glow && go build -o /dev/null .`
Expected: Builds successfully.

- [ ] **Step 3: Commit**

```bash
git add ui/stash.go
git commit -m "feat: extract match context from body during filtering"
```

---

### Task 4: Display match context in list items

**Files:**
- Modify: `ui/stashitem.go:85-89`

- [ ] **Step 1: Update `stashItemView()` to show match context when present**

In `ui/stashitem.go`, change the final rendering block from:

```go
	fmt.Fprintf(b, "%s %s%s%s%s\n", gutter, icon, separator, separator, title)
	fmt.Fprintf(b, "%s %s", gutter, date)
	if hasEditedBy {
		fmt.Fprintf(b, " %s", editedBy)
	}
```

to:

```go
	fmt.Fprintf(b, "%s %s%s%s%s\n", gutter, icon, separator, separator, title)

	// Show match context from body instead of date when filtering with a body match
	if isFiltering && md.matchContext != "" {
		context := truncate.StringWithTail(md.matchContext, truncateTo, ellipsis)
		s := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"})
		context = styleFilteredText(context, m.filterInput.Value(), s, s.Underline(true))
		fmt.Fprintf(b, "%s %s", gutter, context)
	} else {
		fmt.Fprintf(b, "%s %s", gutter, date)
		if hasEditedBy {
			fmt.Fprintf(b, " %s", editedBy)
		}
	}
```

Note: `isFiltering` is already defined at line 31 as `m.filterState == filtering`. The `truncate` and `lipgloss` imports are already present in `stashitem.go`'s imports (via `"github.com/muesli/reflow/truncate"` and `"github.com/charmbracelet/lipgloss"`).

- [ ] **Step 2: Also show match context when filter is applied (not just while typing)**

The `isFiltering` variable only covers `filterState == filtering` (actively typing). We also want to show context when `filterState == filterApplied` (user pressed enter). Update the condition:

Change:

```go
	if isFiltering && md.matchContext != "" {
```

to:

```go
	if (isFiltering || m.filterState == filterApplied) && md.matchContext != "" {
```

- [ ] **Step 3: Verify the app compiles**

Run: `cd /Users/46192/Developer/glow && go build -o /dev/null .`
Expected: Builds successfully.

- [ ] **Step 4: Run all tests**

Run: `cd /Users/46192/Developer/glow && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add ui/stashitem.go
git commit -m "feat: display body match context in file listing"
```

---

### Task 5: Manual smoke test

- [ ] **Step 1: Build and run glow in a directory with markdown files**

Run: `cd /Users/46192/Developer/glow && go build -o glow-test . && ./glow-test`

- [ ] **Step 2: Test title-only match**

Press `/`, type a partial filename. Verify:
- The file appears in results with fuzzy highlighting on the title
- The date line shows normally below the title

- [ ] **Step 3: Test body content match**

Press `/`, type a word that appears only inside a file's body (not its filename). Verify:
- The file appears in results
- The second line shows the matched body line with fuzzy highlighting instead of the date

- [ ] **Step 4: Test clearing the filter**

Press `esc`. Verify:
- All files reappear with normal title + date display
- No leftover match context lines

- [ ] **Step 5: Clean up test binary**

Run: `rm /Users/46192/Developer/glow/glow-test`
