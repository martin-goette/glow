package ui

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed github-markdown.css
var githubMarkdownCSS string

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

func renderMarkdownToHTML(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("error rendering markdown: %w", err)
	}
	return buf.String(), nil
}

var previewTemplate = template.Must(template.New("preview").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
{{.CSS}}
</style>
<style>
body {
	box-sizing: border-box;
	min-width: 200px;
	max-width: 980px;
	margin: 0 auto;
	padding: 45px;
}
</style>
</head>
<body>
<article class="markdown-body">
{{.Body}}
</article>
<script>
const es = new EventSource('/events');
es.addEventListener('reload', () => location.reload());
</script>
</body>
</html>`))

type previewData struct {
	Title string
	CSS   template.CSS
	Body  template.HTML
}

func renderPreviewPage(markdownContent, filename string) (string, error) {
	body, err := renderMarkdownToHTML(markdownContent)
	if err != nil {
		return "", err
	}
	title := filename
	if title == "" {
		title = "Glow Preview"
	}
	var buf bytes.Buffer
	err = previewTemplate.Execute(&buf, previewData{
		Title: title,
		CSS:   template.CSS(githubMarkdownCSS),
		Body:  template.HTML(body),
	})
	if err != nil {
		return "", fmt.Errorf("error executing preview template: %w", err)
	}
	return buf.String(), nil
}

type sseClient struct {
	ch chan struct{}
}

type previewServer struct {
	mu       sync.RWMutex
	html     string
	filename string
	server   *http.Server
	listener net.Listener

	clientsMu sync.Mutex
	clients   []*sseClient
}

func newPreviewServer() *previewServer {
	return &previewServer{}
}

func (s *previewServer) port() int {
	if s.listener == nil {
		return 0
	}
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *previewServer) updateContent(markdownContent, filename string) {
	page, err := renderPreviewPage(markdownContent, filename)
	if err != nil {
		log.Error("error rendering preview page", "error", err)
		return
	}
	s.mu.Lock()
	s.html = page
	s.filename = filename
	s.mu.Unlock()
}

func (s *previewServer) notifyClients() {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for _, c := range s.clients {
		select {
		case c.ch <- struct{}{}:
		default:
		}
	}
}

func (s *previewServer) addClient() *sseClient {
	c := &sseClient{ch: make(chan struct{}, 1)}
	s.clientsMu.Lock()
	s.clients = append(s.clients, c)
	s.clientsMu.Unlock()
	return c
}

func (s *previewServer) removeClient(c *sseClient) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for i, client := range s.clients {
		if client == c {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return
		}
	}
}

func (s *previewServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	html := s.html
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func (s *previewServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	client := s.addClient()
	defer s.removeClient(client)

	for {
		select {
		case <-client.ch:
			fmt.Fprint(w, "event: reload\ndata: updated\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *previewServer) start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = ln
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/events", s.handleSSE)
	s.server = &http.Server{Handler: mux}
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("preview server error", "error", err)
		}
	}()
	return nil
}

func (s *previewServer) stop() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *previewServer) url() string {
	return fmt.Sprintf("http://localhost:%d", s.port())
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

type previewOpenedMsg struct {
	url string
}

type previewErrorMsg struct {
	err error
}

func openPreviewCmd(p *previewServer, markdownContent, filename string) tea.Cmd {
	return func() tea.Msg {
		p.updateContent(markdownContent, filename)
		if p.listener == nil {
			if err := p.start(); err != nil {
				return previewErrorMsg{err}
			}
		}
		if err := openBrowser(p.url()); err != nil {
			return previewErrorMsg{err}
		}
		return previewOpenedMsg{url: p.url()}
	}
}
