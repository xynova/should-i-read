package mailtext

import (
	"bytes"
	"io"
	"mime"
	"net/mail"
	"regexp"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const previewCSPMeta = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: data:; style-src 'unsafe-inline' https:; font-src https: data:; form-action 'none'; base-uri 'none'; media-src https:">` +
	`<meta name="referrer" content="no-referrer">`

const previewChromeStyle = `<style>
html{margin:0;padding:0.5rem 0 2rem !important;background:#fff;overflow:visible;}
body{margin:0;padding:0;background:#fff;overflow:visible;}
img,video{max-width:100%;}
img:not([width="1"]):not([height="1"]),video{height:auto;}
img[width="1"],img[height="1"]{max-width:none;}
</style>`

var (
	reAttrURL     = regexp.MustCompile(`(?i)\b(src|srcset|href|action|poster)\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
	reCSSURL      = regexp.MustCompile(`(?i)url\s*\(\s*['"]?([^'")]+)['"]?\s*\)`)
	reImgTag      = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reTrackingURL = regexp.MustCompile(`(?i)(?:c\.gle(?:/|$)|google-analytics|doubleclick\.net|googlesyndication|adservice\.google|facebook\.com/tr|list-manage\.com|/track(?:ing)?(?:/|\?|$)|pixel\.gif|open\.gif)`)
)

// HTMLFromRFC822 returns the first decoded text/html body without preferring text/plain.
func HTMLFromRFC822(raw []byte) (string, error) {
	const op = "mailtext.HTMLFromRFC822"
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "parse message")
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		mediaType = "text/plain"
	}
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read body")
	}
	body = decodeMIMEBody(msg.Header.Get("Content-Transfer-Encoding"), body)
	html, err := extractHTMLBody(mediaType, params, body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(html), nil
}

func extractHTMLBody(mediaType string, params map[string]string, body []byte) (string, error) {
	const op = "mailtext.extractHTMLBody"
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return "", sirerr.New(sirerr.CodeFailed, op, "multipart missing boundary")
		}
		_, html, err := walkMultipart(boundary, body)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(html), nil
	}
	if strings.HasPrefix(mediaType, "text/html") {
		return string(body), nil
	}
	return "", nil
}

// SafePreviewHTML wraps HTML for iframe srcdoc: CSP, tracking URL neutering, content images kept.
func SafePreviewHTML(html string) string {
	html = strings.TrimSpace(html)
	if html == "" {
		return ""
	}
	html = stripTrackingPixels(html)
	html = stripRemoteURLs(html)
	if html == "" {
		return ""
	}
	chrome := previewCSPMeta + "\n" + previewChromeStyle
	if strings.Contains(strings.ToLower(html), "<head") {
		return injectAfterHeadOpen(html, chrome)
	}
	return chrome + "\n" + html
}

func stripTrackingPixels(s string) string {
	return reImgTag.ReplaceAllStringFunc(s, func(tag string) string {
		if isTrackingURL(tag) {
			return ""
		}
		return tag
	})
}

func stripRemoteURLs(s string) string {
	s = reAttrURL.ReplaceAllStringFunc(s, scrubAttrURL)
	s = reCSSURL.ReplaceAllStringFunc(s, scrubCSSURL)
	return s
}

func scrubAttrURL(attr string) string {
	eq := strings.Index(attr, "=")
	if eq < 0 {
		return attr
	}
	name := strings.ToLower(strings.TrimSpace(attr[:eq]))
	raw := strings.TrimSpace(attr[eq+1:])
	quote := ""
	val := raw
	if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') {
		quote = raw[:1]
		val = raw[1 : len(raw)-1]
	}
	if name == "srcset" {
		return name + "=" + quote + scrubSrcset(val) + quote
	}
	canon := canonicalPreviewURL(val)
	if name == "action" || isTrackingURL(canon) {
		return name + "=" + quote + "#" + quote
	}
	if canon == val {
		return attr
	}
	return name + "=" + quote + canon + quote
}

func scrubSrcset(srcset string) string {
	parts := strings.Split(srcset, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.Fields(p)
		fields[0] = canonicalPreviewURL(fields[0])
		if isTrackingURL(fields[0]) {
			continue
		}
		out = append(out, strings.Join(fields, " "))
	}
	if len(out) == 0 {
		return "#"
	}
	return strings.Join(out, ", ")
}

func scrubCSSURL(match string) string {
	sub := reCSSURL.FindStringSubmatch(match)
	if len(sub) < 2 {
		return match
	}
	orig := strings.TrimSpace(sub[1])
	url := canonicalPreviewURL(orig)
	if isTrackingURL(url) {
		return "url(#)"
	}
	if url == orig {
		return match
	}
	return "url(" + url + ")"
}

func canonicalPreviewURL(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return u
}

func isTrackingURL(u string) bool {
	return reTrackingURL.MatchString(u)
}

func injectAfterHeadOpen(html, chrome string) string {
	lower := strings.ToLower(html)
	idx := strings.Index(lower, "<head")
	if idx < 0 {
		return chrome + "\n" + html
	}
	rest := html[idx:]
	close := strings.Index(strings.ToLower(rest), ">")
	if close < 0 {
		return chrome + "\n" + html
	}
	insertAt := idx + close + 1
	return html[:insertAt] + "\n" + chrome + html[insertAt:]
}

// TruncateHTMLBytes caps html at maxBytes for preview responses.
func TruncateHTMLBytes(html string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(html) <= maxBytes {
		return html, false
	}
	return html[:maxBytes], true
}
