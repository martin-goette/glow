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
