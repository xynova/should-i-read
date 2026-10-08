package mailreport

import (
	"strings"
	"unicode/utf8"
)

const (
	essenceMaxParagraphs = 2
	essenceMaxRunes      = 800
)

// EssenceBody extractively shortens denoised latest-message body for Author prompts.
func EssenceBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	paras := firstSubstantiveParagraphs(body, essenceMaxParagraphs)
	joined := strings.Join(paras, "\n\n")
	return capAtBoundary(joined, essenceMaxRunes)
}

func firstSubstantiveParagraphs(body string, n int) []string {
	if n <= 0 {
		return nil
	}
	chunks := strings.Split(body, "\n\n")
	out := make([]string, 0, n)
	for _, c := range chunks {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		out = append(out, c)
		if len(out) >= n {
			break
		}
	}
	if len(out) == 0 {
		trim := strings.TrimSpace(body)
		if trim != "" {
			out = []string{trim}
		}
	}
	return out
}

func capAtBoundary(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return strings.TrimSpace(s)
	}
	// Prefer paragraph boundary, then sentence, then hard rune cut.
	if idx := paragraphCut(s, maxRunes); idx > 0 {
		return strings.TrimSpace(s[:idx])
	}
	if idx := sentenceCut(s, maxRunes); idx > 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(truncateRunes(s, maxRunes))
}

func paragraphCut(s string, maxRunes int) int {
	prefix := truncateRunes(s, maxRunes)
	last := strings.LastIndex(prefix, "\n\n")
	if last > 0 {
		return last
	}
	return 0
}

func sentenceCut(s string, maxRunes int) int {
	prefix := truncateRunes(s, maxRunes)
	best := 0
	for i, r := range prefix {
		if r == '.' || r == '!' || r == '?' {
			pos := i + 1
			if pos > best {
				best = pos
			}
		}
	}
	return best
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

// AuthorMessageBody prepares formatItemText output for the Author seat (foundation, denoise, essence).
func AuthorMessageBody(formatted string) string {
	header, body := splitFormatItemText(formatted)
	foundation := FoundationBody(body)
	denoised := DenoiseBody(foundation)
	essence := EssenceBody(denoised)
	if essence == "" {
		fallback := DenoiseBody(foundation)
		if fallback == "" {
			fallback = foundation
		}
		essence = capAtBoundary(fallback, essenceMaxRunes)
	}
	return joinFormatItemText(header, essence)
}
