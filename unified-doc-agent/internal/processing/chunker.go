package processing

import (
	"regexp"
	"strings"
)

// ChunkText splits into paragraph chunks and limits size.
func ChunkText(text string) []string {
	re := regexp.MustCompile(`\n{2,}`)
	paras := re.Split(text, -1)
	var out []string
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// further split very long paragraphs into ~1000-char chunks with overlap
		out = append(out, splitLong(p, 1000, 200)...)
	}
	return out
}

func splitLong(s string, max, overlap int) []string {
	if len(s) <= max {
		return []string{s}
	}
	var res []string
	for i := 0; i < len(s); i += (max - overlap) {
		end := i + max
		if end > len(s) {
			end = len(s)
		}
		res = append(res, strings.TrimSpace(s[i:end]))
		if end == len(s) {
			break
		}
	}
	return res
}
