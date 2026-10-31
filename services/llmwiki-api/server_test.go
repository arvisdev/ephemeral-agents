package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseQueryParams(t *testing.T) {
	t.Run("valid with defaults", func(t *testing.T) {
		q, err := ParseQueryParams(url.Values{"q": {"design standards for APIs"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.K != 5 || q.Query != "design standards for APIs" {
			t.Errorf("got %+v", q)
		}
	})

	t.Run("explicit k", func(t *testing.T) {
		q, err := ParseQueryParams(url.Values{"q": {"docker skills"}, "k": {"3"}})
		if err != nil || q.K != 3 {
			t.Errorf("got %+v err=%v", q, err)
		}
	})

	t.Run("missing q rejected", func(t *testing.T) {
		_, err := ParseQueryParams(url.Values{"k": {"3"}})
		if err == nil || !strings.Contains(err.Error(), "q is required") {
			t.Errorf("want q-required error, got %v", err)
		}
	})

	t.Run("blank q rejected", func(t *testing.T) {
		_, err := ParseQueryParams(url.Values{"q": {"   "}})
		if err == nil {
			t.Error("want error for blank q")
		}
	})

	t.Run("k bounds", func(t *testing.T) {
		_, err := ParseQueryParams(url.Values{"q": {"x"}, "k": {"0"}})
		if err == nil || !strings.Contains(err.Error(), "k must be") {
			t.Errorf("k=0: want error, got %v", err)
		}
		_, err = ParseQueryParams(url.Values{"q": {"x"}, "k": {"51"}})
		if err == nil {
			t.Error("k=51: want error")
		}
		_, err = ParseQueryParams(url.Values{"q": {"x"}, "k": {"abc"}})
		if err == nil || !strings.Contains(err.Error(), "integer") {
			t.Errorf("k=abc: want integer error, got %v", err)
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		q, err := ParseQueryParams(url.Values{"q": {"  naming  "}})
		if err != nil || q.Query != "naming" {
			t.Errorf("got %+v err=%v", q, err)
		}
	})
}

func TestChunkMarkdown(t *testing.T) {
	doc := "# API Standards\nintro text\n## Naming\nUse kebab-case.\nmore naming text\n## Errors\nReturn RFC 7807 bodies.\n"
	chunks := ChunkMarkdown("design-standards/api.md", doc)
	if len(chunks) != 3 {
		t.Fatalf("want 3 chunks (front matter, Naming, Errors), got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Section != "API Standards" || !strings.Contains(chunks[0].Text, "intro text") {
		t.Errorf("chunk 0 = %+v", chunks[0])
	}
	if chunks[1].Section != "Naming" || !strings.Contains(chunks[1].Text, "kebab-case") {
		t.Errorf("chunk 1 = %+v", chunks[1])
	}
	if chunks[2].Section != "Errors" || !strings.Contains(chunks[2].Text, "RFC 7807") {
		t.Errorf("chunk 2 = %+v", chunks[2])
	}
	for i, c := range chunks {
		if c.Source != "design-standards/api.md" {
			t.Errorf("chunk %d source = %q", i, c.Source)
		}
		if strings.TrimSpace(c.Text) == "" {
			t.Errorf("chunk %d is empty", i)
		}
	}
}

func TestChunkMarkdownEmpty(t *testing.T) {
	if got := ChunkMarkdown("a.md", "\n\n   \n"); len(got) != 0 {
		t.Errorf("want no chunks for blank doc, got %+v", got)
	}
}

func TestChunkMarkdownSplitsLongSections(t *testing.T) {
	para := strings.Repeat("word ", 60) + "\n\n" // ~300 chars per paragraph
	var b strings.Builder
	b.WriteString("## Big\n")
	for i := 0; i < 10; i++ {
		b.WriteString(para)
	}
	chunks := ChunkMarkdown("x.md", b.String())
	if len(chunks) < 2 {
		t.Fatalf("want long section split into multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c.Text) > chunkMaxChars {
			t.Errorf("chunk exceeds max: %d chars", len(c.Text))
		}
		if c.Section != "Big" {
			t.Errorf("section = %q, want Big", c.Section)
		}
	}
}

func TestChunkMarkdownCRLF(t *testing.T) {
	chunks := ChunkMarkdown("w.md", "# T\r\nbody\r\n## S\r\nmore\r\n")
	if len(chunks) != 2 {
		t.Fatalf("want 2 chunks, got %d", len(chunks))
	}
}
