package mailreport

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	reSeparatorLine    = regexp.MustCompile(`^[\s=*_\-~]{4,}$`)
	reURLLine          = regexp.MustCompile(`(?i)^\s*https?://\S+\s*$`)
	reUnsubscribe      = regexp.MustCompile(`(?i)^\s*(unsubscribe|manage (your )?preferences|email preferences)\b`)
	reViewBrowser      = regexp.MustCompile(`(?i)^\s*(view|read|open) (this )?(email|message) (in (your )?)?(browser|web)\b`)
	reReceivingBecause = regexp.MustCompile(`(?i)^\s*you (are receiving|received) this (email|message) because\b`)
	reMobileSig        = regexp.MustCompile(`(?i)^\s*sent from my (iphone|ipad|android|galaxy)\b`)
	reOutlookApp       = regexp.MustCompile(`(?i)^\s*get outlook for (ios|android)\b`)
	reBase64Blob       = regexp.MustCompile(`^[A-Za-z0-9+/=_-]{120,}$`)
	reHTMLTag          = regexp.MustCompile(`<[^>]+>`)
)

const footerLegalScanLines = 25

// DenoiseBody removes common marketing and artifact lines from latest-message plain text.
func DenoiseBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	body = html.UnescapeString(body)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = strings.ReplaceAll(body, "\u00a0", " ")
	body = norm.NFC.String(body)
	body = stripControlChars(body)

	lines := strings.Split(body, "\n")
	lines = dropFooterLegalBlock(lines)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = normalizeLineSpaces(line)
		if line == "" {
			out = append(out, "")
			continue
		}
		if shouldDropNoiseLine(line) {
			continue
		}
		line = replaceBase64Blobs(line)
		line = collapseRepeatedEmoji(line)
		out = append(out, line)
	}
	body = strings.Join(out, "\n")
	body = collapseBlankLines(body, 2)
	body = collapseSeparatorRuns(body)
	return strings.TrimSpace(body)
}

func stripControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' || unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeLineSpaces(line string) string {
	line = strings.ReplaceAll(line, "\u00a0", " ")
	line = strings.ReplaceAll(line, "\u200b", "")
	return strings.TrimRight(line, " \t")
}

func shouldDropNoiseLine(line string) bool {
	trim := strings.TrimSpace(line)
	if trim == "" {
		return false
	}
	if reSeparatorLine.MatchString(trim) {
		return true
	}
	if reUnsubscribe.MatchString(trim) || reViewBrowser.MatchString(trim) || reReceivingBecause.MatchString(trim) {
		return true
	}
	if reMobileSig.MatchString(trim) || reOutlookApp.MatchString(trim) {
		return true
	}
	if reURLLine.MatchString(trim) {
		return true
	}
	if reHTMLTag.MatchString(trim) && strings.Count(trim, "<") == strings.Count(trim, ">") {
		// Residual HTML line; drop rather than pass tags to the model.
		return true
	}
	return false
}

func replaceBase64Blobs(line string) string {
	fields := strings.Fields(line)
	for i, f := range fields {
		if reBase64Blob.MatchString(f) {
			fields[i] = "[encoded_blob]"
		}
	}
	return strings.Join(fields, " ")
}

func collapseRepeatedEmoji(line string) string {
	if line == "" {
		return line
	}
	var b strings.Builder
	var last rune
	var run int
	for _, r := range line {
		if r == last && isEmojiLike(r) {
			run++
			if run > 2 {
				continue
			}
		} else {
			last = r
			run = 1
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isEmojiLike(r rune) bool {
	return r > 0x2600
}

func dropFooterLegalBlock(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	start := len(lines) - footerLegalScanLines
	if start < 0 {
		start = 0
	}
	cut := len(lines)
	for i := start; i < len(lines); i++ {
		low := strings.ToLower(strings.TrimSpace(lines[i]))
		if strings.Contains(low, "confidential") && strings.Contains(low, "intended recipient") {
			cut = i
			break
		}
		if strings.HasPrefix(low, "this email") && strings.Contains(low, "privileged") {
			cut = i
			break
		}
	}
	if cut < len(lines) {
		return lines[:cut]
	}
	return lines
}

func collapseBlankLines(s string, maxRun int) string {
	if maxRun < 1 {
		maxRun = 1
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > maxRun {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func collapseSeparatorRuns(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if reSeparatorLine.MatchString(strings.TrimSpace(line)) {
			if len(out) > 0 && reSeparatorLine.MatchString(strings.TrimSpace(out[len(out)-1])) {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
