package main

// Chunking turns curated markdown into indexable passages. Pure logic,
// unit-tested in chunk_test.go. POC granularity (install doc §6
// troubleshooting): one chunk per markdown section, capped at maxChars so a
// single giant section still embeds within the model's input limit. The same
// chunker runs on both the ingest and (implicitly) the query side — the doc's
// rule is "same embedding model, same dims, same chunk size on both sides".

import "strings"

// Chunk is one indexable passage with its provenance (source file + section
// heading), which the query endpoint returns alongside the text.
type Chunk struct {
	Source  string // repo-relative file path, e.g. design-standards/api.md
	Section string // nearest "## heading" above the chunk, "" for front matter
	Text    string // the passage body
}

// chunkMaxChars bounds a chunk's text length.
const chunkMaxChars = 2000

// ChunkMarkdown splits one markdown file's content into chunks.
func ChunkMarkdown(source, content string) []Chunk {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	var chunks []Chunk
	section := ""
	var body []string
	flush := func() {
		text := strings.TrimSpace(strings.Join(body, "\n"))
		body = nil
		if text == "" {
			return
		}
		chunks = append(chunks, splitLong(source, section, text)...)
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "): // H1: file title — starts a new section
			flush()
			section = strings.TrimPrefix(trimmed, "# ")
		case strings.HasPrefix(trimmed, "## "): // H2: section boundary
			flush()
			section = strings.TrimPrefix(trimmed, "## ")
		default:
			body = append(body, line)
		}
	}
	flush()
	return chunks
}

// splitLong splits an oversized section body on paragraph boundaries.
func splitLong(source, section, text string) []Chunk {
	if len(text) <= chunkMaxChars {
		return []Chunk{{Source: source, Section: section, Text: text}}
	}
	paragraphs := strings.Split(text, "\n\n")
	var out []Chunk
	var cur []string
	n := 0
	flushCur := func() {
		if t := strings.TrimSpace(strings.Join(cur, "\n\n")); t != "" {
			out = append(out, Chunk{Source: source, Section: section, Text: t})
		}
		cur = nil
		n = 0
	}
	for _, p := range paragraphs {
		if n > 0 && n+len(p)+2 > chunkMaxChars {
			flushCur()
		}
		cur = append(cur, p)
		n += len(p) + 2
	}
	flushCur()
	return out
}
