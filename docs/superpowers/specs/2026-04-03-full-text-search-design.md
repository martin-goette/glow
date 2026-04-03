# Full-Text Search with Match Context

## Overview

Extend glow's `/` filter in the stash (file listing) view to search file contents, not just filenames. When a match comes from the body of a file, display the matched line as context in the list item.

## Current Behavior

- `/` activates a fuzzy filter that matches against `markdown.Note` (the relative file path)
- File bodies are loaded lazily, only when a file is opened for viewing
- List items display: title (filename) + relative timestamp
- `filterValue` is built from `Note` only (`buildFilterValue()` in `ui/markdown.go`)
- Filtering runs async in a `tea.Cmd` goroutine via `filterMarkdowns()` in `ui/stash.go`
- `fuzzy.Find` from `sahilm/fuzzy v0.1.1` returns ranked matches with `MatchedIndexes`

## Design

### 1. Eager Body Loading

Modify `localFileToMarkdown()` in `ui/ui.go` to read file contents and populate `Body` immediately during startup file discovery. This replaces the current lazy-load pattern for the listing path.

The existing `loadLocalMarkdown()` command (used when opening a file for rendering) remains unchanged.

**Trade-off**: Adds startup time and memory proportional to total file size. Acceptable for typical markdown collections (dozens to low hundreds of files, KB-sized). If this becomes a bottleneck, background loading can be added later.

### 2. Extended Filter Value

Modify `buildFilterValue()` in `ui/markdown.go`:

- Concatenate `Note + "\n" + Body` into `filterValue`
- This allows `fuzzy.Find` to match against both filename and content in a single pass
- The `\n` separator between Note and Body allows determining which region matched by comparing match indexes against `len(Note)`

### 3. Match Context Extraction

Add a `matchContext` field to the `markdown` struct. This is ephemeral (like `filterValue`), only meaningful during active filtering.

In `filterMarkdowns()` (`ui/stash.go`), after getting fuzzy matches:

1. For each matched markdown, check if any `MatchedIndexes` fall past `len(Note)` (i.e., the match is in the body portion)
2. If body match: split `Body` by newlines, run `fuzzy.Find` per-line to find the best matching line, store it in `matchContext`
3. If title-only match: set `matchContext` to empty string
4. When filter is cleared, `matchContext` is naturally ignored (not displayed)

### 4. List Item Display

Modify `stashItemView()` in `ui/stashitem.go`:

- When filtering and `md.matchContext` is non-empty, replace the date line with the truncated match context line
- Apply the same fuzzy highlight styling used for titles (dimmed base text, underlined matched characters)
- When not filtering or match is title-only, show the date line as usual

Visual change:

```
# Title match (unchanged)
|  my-document.md        <- title with fuzzy highlights
|  2 hours ago           <- date as usual

# Body match (new)
|  my-document.md        <- title without highlights
|  ...the matched line.  <- body context with fuzzy highlights
```

### 5. Unchanged

- Help text, key bindings (`/` to filter, `esc` to clear)
- Filter state machine (`unfiltered`, `filtering`, `filterApplied`)
- Pagination logic
- Sort order
- Section cycling

## Files Modified

| File | Change |
|------|--------|
| `ui/markdown.go` | Add `matchContext` field, extend `buildFilterValue()` to include `Body` |
| `ui/ui.go` | Read file contents in `localFileToMarkdown()` |
| `ui/stash.go` | Update `filterMarkdowns()` to extract match context |
| `ui/stashitem.go` | Conditionally render match context line instead of date |
