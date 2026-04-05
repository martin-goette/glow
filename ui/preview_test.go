package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- renderMarkdownToHTML tests ---

func TestRenderMarkdownToHTML_BasicMarkdown(t *testing.T) {
	input := "# Hello World\n\nThis is a **bold** paragraph."
	out, err := renderMarkdownToHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "<h1") {
		t.Errorf("expected <h1> element, got: %s", out)
	}
	if !strings.Contains(out, "Hello World") {
		t.Errorf("expected 'Hello World' in output, got: %s", out)
	}
	if !strings.Contains(out, "<strong>bold</strong>") {
		t.Errorf("expected <strong>bold</strong>, got: %s", out)
	}
}

func TestRenderMarkdownToHTML_GFMTable(t *testing.T) {
	input := `| Name | Age |
| ---- | --- |
| Alice | 30 |
| Bob | 25 |`
	out, err := renderMarkdownToHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "<table>") {
		t.Errorf("expected <table> element, got: %s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected 'Alice' in output, got: %s", out)
	}
	if !strings.Contains(out, "<th>") {
		t.Errorf("expected <th> elements, got: %s", out)
	}
}

func TestRenderMarkdownToHTML_CodeBlockSyntaxHighlighting(t *testing.T) {
	input := "```go\npackage main\n\nfunc main() {}\n```"
	out, err := renderMarkdownToHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// chroma wraps code with a div/span structure; check for code content
	if !strings.Contains(out, "main") {
		t.Errorf("expected 'main' in highlighted output, got: %s", out)
	}
	// chroma-highlighted code uses span elements or a wrapper class
	if !strings.Contains(out, "<code") && !strings.Contains(out, "chroma") {
		t.Errorf("expected code or chroma element in output, got: %s", out)
	}
}

func TestRenderMarkdownToHTML_TaskList(t *testing.T) {
	input := "- [x] Done task\n- [ ] Pending task"
	out, err := renderMarkdownToHTML(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `type="checkbox"`) {
		t.Errorf("expected checkbox input in task list, got: %s", out)
	}
	if !strings.Contains(out, "Done task") {
		t.Errorf("expected 'Done task' text, got: %s", out)
	}
	if !strings.Contains(out, "Pending task") {
		t.Errorf("expected 'Pending task' text, got: %s", out)
	}
}

func TestRenderMarkdownToHTML_EmptyInput(t *testing.T) {
	out, err := renderMarkdownToHTML("")
	if err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
	// empty markdown should produce empty or minimal output
	if strings.Contains(out, "<h1") || strings.Contains(out, "<p>") {
		t.Errorf("expected minimal output for empty input, got: %s", out)
	}
}

// --- renderPreviewPage tests ---

func TestRenderPreviewPage_ContainsExpectedStructure(t *testing.T) {
	page, err := renderPreviewPage("# Test\n\nHello.", "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	checks := []string{
		"<!DOCTYPE html>",
		"<html>",
		"<title>test.md</title>",
		`class="markdown-body"`,
		"Test",
		"Hello",
		"EventSource",
		"/events",
		"markdown-body",
	}
	for _, want := range checks {
		if !strings.Contains(page, want) {
			t.Errorf("expected %q in page output", want)
		}
	}
}

func TestRenderPreviewPage_EmbeddedCSS(t *testing.T) {
	page, err := renderPreviewPage("Hello", "readme.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The embedded github-markdown.css should be inlined
	if !strings.Contains(page, ".markdown-body") {
		t.Errorf("expected embedded CSS with .markdown-body selector")
	}
}

func TestRenderPreviewPage_StdinFallbackTitle(t *testing.T) {
	page, err := renderPreviewPage("# Hello", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(page, "<title>Glow Preview</title>") {
		t.Errorf("expected fallback title 'Glow Preview' when filename is empty, got page with different title")
	}
}

func TestRenderPreviewPage_CustomFilenameTitle(t *testing.T) {
	page, err := renderPreviewPage("content", "myfile.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(page, "<title>myfile.md</title>") {
		t.Errorf("expected title 'myfile.md', got page without that title")
	}
}

// --- previewServer tests ---

func TestPreviewServer_ServesHTML(t *testing.T) {
	s := newPreviewServer()
	s.updateContent("# Hello\n\nWorld.", "test.md")

	if err := s.start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.stop()

	if s.port() == 0 {
		t.Fatal("expected non-zero port after start")
	}

	resp, err := http.Get(s.url())
	if err != nil {
		t.Fatalf("failed to GET server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %q", ct)
	}
}

func TestPreviewServer_SSEContentType(t *testing.T) {
	s := newPreviewServer()
	s.updateContent("# Hello", "test.md")

	// Use httptest to test the SSE handler directly
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/events", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// We need a context with cancel to simulate client disconnect
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.handleSSE(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected text/event-stream content-type, got %q", ct)
	}
}

func TestPreviewServer_NotifyClients(t *testing.T) {
	s := newPreviewServer()

	client := s.addClient()
	defer s.removeClient(client)

	s.notifyClients()

	select {
	case <-client.ch:
		// success: client was notified
	case <-time.After(time.Second):
		t.Error("client was not notified within timeout")
	}
}

func TestPreviewServer_RemoveClient(t *testing.T) {
	s := newPreviewServer()

	c1 := s.addClient()
	c2 := s.addClient()

	s.removeClient(c1)

	s.clientsMu.Lock()
	count := len(s.clients)
	s.clientsMu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 client after removal, got %d", count)
	}

	// Ensure c2 is still present
	s.clientsMu.Lock()
	if s.clients[0] != c2 {
		t.Error("expected c2 to remain after c1 removal")
	}
	s.clientsMu.Unlock()
}

func TestPreviewServer_UpdateContent(t *testing.T) {
	s := newPreviewServer()
	s.updateContent("# Hello", "file.md")

	s.mu.RLock()
	html := s.html
	filename := s.filename
	s.mu.RUnlock()

	if !strings.Contains(html, "Hello") {
		t.Errorf("expected updated HTML to contain 'Hello', got: %s", html)
	}
	if filename != "file.md" {
		t.Errorf("expected filename 'file.md', got %q", filename)
	}
}

func TestPreviewServer_PortZeroBeforeStart(t *testing.T) {
	s := newPreviewServer()
	if s.port() != 0 {
		t.Errorf("expected port 0 before start, got %d", s.port())
	}
}

func TestPreviewServer_URLFormat(t *testing.T) {
	s := newPreviewServer()
	s.updateContent("# Test", "t.md")
	if err := s.start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	defer s.stop()

	url := s.url()
	if !strings.HasPrefix(url, "http://localhost:") {
		t.Errorf("expected URL to start with http://localhost:, got %q", url)
	}
}
