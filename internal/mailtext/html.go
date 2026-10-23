package mailtext

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

var (
	reHTMLComment  = regexp.MustCompile(`(?is)<!--.*?-->`)
	reHTMLMetaTag  = regexp.MustCompile(`(?is)<(meta|link|base|img|input)\b[^>]*/?>`)
	htmlMetaBlocks = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`),
		regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>`),
		regexp.MustCompile(`(?is)<head\b[^>]*>.*?</head\s*>`),
		regexp.MustCompile(`(?is)<noscript\b[^>]*>.*?</noscript\s*>`),
		regexp.MustCompile(`(?is)<title\b[^>]*>.*?</title\s*>`),
		regexp.MustCompile(`(?is)<svg\b[^>]*>.*?</svg\s*>`),
		regexp.MustCompile(`(?is)<template\b[^>]*>.*?</template\s*>`),
	}
	reHTMLBreak    = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHTMLBlockEnd = regexp.MustCompile(`(?i)</(p|div|tr|li|h[1-6]|table|section|article)\s*>`)
	reHTMLTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	reCSSAtRule    = regexp.MustCompile(`(?i)^@(font-face|media|import|charset|keyframes)\b`)
	reCSSProperty  = regexp.MustCompile(`(?i)^(font-family|font-style|font-weight|font-size|src|mso-[a-z-]+)\s*:`)
)

// StripHTML turns an HTML fragment into visible text. Style, script, head,
// comments, and other markup metadata are dropped rather than kept as inner text.
func StripHTML(s string) string {
	s = reHTMLComment.ReplaceAllString(s, "\n")
	for _, re := range htmlMetaBlocks {
		s = re.ReplaceAllString(s, "\n")
	}
	s = reHTMLMetaTag.ReplaceAllString(s, "\n")
	s = reHTMLBreak.ReplaceAllString(s, "\n")
	s = reHTMLBlockEnd.ReplaceAllString(s, "\n")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = dropMarkupMetadataLines(s)
	return strings.TrimSpace(collapseMarkupSpace(s))
}

func dropMarkupMetadataLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if isMarkupMetadataLine(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isMarkupMetadataLine(line string) bool {
	trim := strings.TrimSpace(line)
	if trim == "" {
		return false
	}
	if trim == "{" || trim == "}" || trim == "*/" || strings.HasPrefix(trim, "/*") {
		return true
	}
	if reCSSAtRule.MatchString(trim) {
		return true
	}
	if strings.Contains(strings.ToLower(trim), "mso-") {
		return true
	}
	if reCSSProperty.MatchString(trim) {
		return true
	}
	if strings.Contains(trim, "url(") && strings.Contains(strings.ToLower(trim), "format(") {
		return true
	}
	return false
}

func collapseMarkupSpace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u00a0", " ")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		var b strings.Builder
		space := false
		for _, r := range line {
			if unicode.IsSpace(r) {
				if !space {
					b.WriteByte(' ')
					space = true
				}
				continue
			}
			space = false
			b.WriteRune(r)
		}
		trim := strings.TrimSpace(b.String())
		if trim == "" {
			blank++
			if blank > 2 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, trim)
	}
	return strings.Join(out, "\n")
}
