package mailreport

import (
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	essenceKeyWhy1  = "WHY1"
	essenceKeyWhy2  = "WHY2"
	essenceKeyWhy3  = "WHY3"
	essenceKeyWhy4  = "WHY4"
	essenceKeyWhy5  = "WHY5"
	essenceKeyAbout = "ABOUT"
	essenceKeyTags  = "TAGS"
	essenceKeyForm  = "FORM"
	essenceKeyShape = "SHAPE"
	essenceKeyKind  = "KIND"

	essenceFieldMissing      = "missing"
	essenceFieldPresent      = "present"
	essenceFieldKindShort    = "kind_short"
	essenceFieldKindShortYes = "1"

	// EssenceErrWhy1Empty is the validate message when WHY1 is missing (stable for dumps and CLI gloss).
	EssenceErrWhy1Empty = "why1 empty"
	// EssenceErrWhyEmpty is the validate message when a WHY line after WHY1 is missing.
	EssenceErrWhyEmpty = "why empty"
	// EssenceErrAboutOrShape is the validate message when ABOUT or TAGS is missing.
	EssenceErrAboutOrShape = "about or shape empty"
	// EssenceErrKindShort is the validate message when KIND has fewer than 3 segments.
	EssenceErrKindShort = "kind needs 3+ segments"
)

var essenceWhyKeys = []string{
	essenceKeyWhy1,
	essenceKeyWhy2,
	essenceKeyWhy3,
	essenceKeyWhy4,
	essenceKeyWhy5,
}

var essenceAllKeys = []string{
	essenceKeyWhy1,
	essenceKeyWhy2,
	essenceKeyWhy3,
	essenceKeyWhy4,
	essenceKeyWhy5,
	essenceKeyAbout,
	essenceKeyTags,
	essenceKeyKind,
}

var essenceParseKeys = []string{
	essenceKeyWhy1,
	essenceKeyWhy2,
	essenceKeyWhy3,
	essenceKeyWhy4,
	essenceKeyWhy5,
	essenceKeyAbout,
	essenceKeyTags,
	essenceKeyForm,
	essenceKeyShape,
	essenceKeyKind,
}

var (
	defaultEssenceSystemPrompt string
	essenceKeyRE               *regexp.Regexp
	essenceTagCatalog          *tags.Catalog
)

func init() {
	defaultEssenceSystemPrompt = BuildEssenceSystemPrompt(nil)
	essenceKeyRE = regexp.MustCompile(`^(` + strings.Join(essenceParseKeys, "|") + `):\s*(.*)$`)
}

// DefaultEssenceSystemPrompt returns the static prompt without active tag ids (tests).
func DefaultEssenceSystemPrompt() string {
	return defaultEssenceSystemPrompt
}

// BuildEssenceSystemPrompt builds the essence system message, listing active tag ids when provided.
func BuildEssenceSystemPrompt(activeTags []string) string {
	var keys strings.Builder
	for _, k := range essenceAllKeys {
		keys.WriteString(k)
		keys.WriteString(":\n")
	}
	var tagHint strings.Builder
	tagHint.WriteString("TAGS: one or more kebab-case tag ids (comma or space separated). ")
	tagHint.WriteString("Prefer these tags when they fit. You may add a new kebab-case tag if none fit. ")
	tagHint.WriteString("Do not describe layout or geometry. Do not copy KIND.\n")
	if len(activeTags) > 0 {
		tagHint.WriteString("Active tags: ")
		tagHint.WriteString(strings.Join(activeTags, ", "))
		tagHint.WriteByte('\n')
	}
	return `Describe what the message is. Do not decide keep, discard, or unwanted.

Reply with these keys (one per line). Values may continue on the next line after "KEY:".

` + keys.String() + `
` + tagHint.String() + `
KIND must be 3 to 5 kebab-case segments joined by " > " (example: notification > software-release > changelog).`
}

// SetEssenceTagCatalog sets the catalog used for tag validation during classify runs.
func SetEssenceTagCatalog(cat *tags.Catalog) {
	essenceTagCatalog = cat
}

const essenceParseMaxAttempts = 3

func essenceIntegrityValidateFor(cat *tags.Catalog) func(string) error {
	return func(raw string) error {
		_, err := parseEssenceResponseWithCatalog(raw, cat)
		return err
	}
}

// essenceIntegrityCompleteNote builds the user turn after a parse validation failure.
func essenceIntegrityCompleteNote(err error) string {
	missing, present := essenceKeysFromErr(err)
	if len(missing) > 0 {
		var b strings.Builder
		b.WriteString("Continue your previous reply. Missing: ")
		b.WriteString(strings.Join(missing, ", "))
		b.WriteByte('.')
		if len(present) > 0 {
			b.WriteString(" Keep ")
			b.WriteString(strings.Join(present, ", "))
			b.WriteByte('.')
		}
		b.WriteString(" Add the missing keys with non-empty values.")
		return b.String()
	}
	if essenceKindShortFromErr(err) {
		return "Continue your previous reply. Keep WHY, ABOUT, and TAGS. Expand KIND to 3 to 5 kebab-case segments joined by \" > \" (example: notification > software-release > changelog)."
	}
	if essenceTagInvalidFromErr(err) {
		return "Continue your previous reply. Fix TAGS: use kebab-case tag ids only (no geometry words, no KIND paths). Prefer listed Active tags or mint a new kebab tag."
	}
	return "Continue your previous reply. Emit WHY1 through WHY5, ABOUT, TAGS, and KIND (3 to 5 kebab-case segments joined by \" > \"). Do not start over."
}

func essenceTagInvalidFromErr(err error) bool {
	var typed *sirerr.Error
	if !errors.As(err, &typed) || typed == nil {
		return false
	}
	return strings.Contains(strings.ToLower(typed.Message), tags.ErrTokenInvalid)
}

func essenceKeysFromErr(err error) (missing, present []string) {
	var typed *sirerr.Error
	if !errors.As(err, &typed) || typed == nil || typed.Fields == nil {
		return nil, nil
	}
	return splitEssenceKeyList(typed.Fields[essenceFieldMissing]), splitEssenceKeyList(typed.Fields[essenceFieldPresent])
}

func essenceKindShortFromErr(err error) bool {
	var typed *sirerr.Error
	if !errors.As(err, &typed) || typed == nil || typed.Fields == nil {
		return false
	}
	return typed.Fields[essenceFieldKindShort] == essenceFieldKindShortYes
}

func splitEssenceKeyList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// WorldContextAttach is the fixed brief for Attach Operate.
const WorldContextAttach = "Report only. Describe what the message is. Do not decide keep, discard, or unwanted."

func parseEssenceResponse(raw string) (harness.EssenceOut, error) {
	return parseEssenceResponseWithCatalog(raw, essenceTagCatalog)
}

func parseEssenceResponseWithCatalog(raw string, cat *tags.Catalog) (harness.EssenceOut, error) {
	var why [5]string
	about, tagsRaw, kind := "", "", ""
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var pendingKey string
	var pendingVal []string
	flush := func() {
		if pendingKey == "" {
			return
		}
		val := strings.TrimSpace(strings.Join(pendingVal, "\n"))
		essenceAssignParsedKey(&why, &about, &tagsRaw, &kind, pendingKey, val)
		pendingKey = ""
		pendingVal = nil
	}
	for _, line := range lines {
		if m := essenceKeyRE.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			flush()
			pendingKey = m[1]
			if rest := strings.TrimSpace(m[2]); rest != "" {
				pendingVal = []string{rest}
			}
			continue
		}
		if pendingKey != "" && strings.TrimSpace(line) != "" {
			pendingVal = append(pendingVal, strings.TrimSpace(line))
		}
	}
	flush()
	kind = normalizeKindString(kind)
	shape, err := normalizeEssenceTags(cat, tagsRaw)
	if err != nil {
		return harness.EssenceOut{}, err
	}
	out := harness.EssenceOut{Why: why, About: about, Shape: shape, Kind: kind}
	if err := validateParsedEssence(out); err != nil {
		return harness.EssenceOut{}, err
	}
	return out, nil
}

func normalizeEssenceTags(cat *tags.Catalog, raw string) (string, error) {
	const op = "mailreport.normalizeEssenceTags"
	raw = splitEssenceTagPaths(raw)
	if raw == "" {
		return "", nil
	}
	if cat != nil {
		return cat.NormalizeField(raw)
	}
	deny := mailTagDenySet()
	tokens := tags.SplitTags(raw)
	for _, tok := range tokens {
		if err := tags.ValidateToken(tok, deny); err != nil {
			return "", err
		}
	}
	joined := tags.JoinTags(tokens)
	if joined == "" {
		return "", sirerr.New(sirerr.CodeAI, op, tags.ErrTokenInvalid)
	}
	return joined, nil
}

// splitEssenceTagPaths turns KIND-shaped TAGS ("a > b > c") into leftover tokens.
func splitEssenceTagPaths(raw string) string {
	raw = strings.ReplaceAll(raw, ">", " ")
	return strings.TrimSpace(raw)
}

func essenceAssignParsedKey(why *[5]string, about, tagsRaw, kind *string, key, val string) {
	switch key {
	case essenceKeyWhy1:
		why[0] = val
	case essenceKeyWhy2:
		why[1] = val
	case essenceKeyWhy3:
		why[2] = val
	case essenceKeyWhy4:
		why[3] = val
	case essenceKeyWhy5:
		why[4] = val
	case essenceKeyAbout:
		*about = val
	case essenceKeyTags, essenceKeyForm, essenceKeyShape:
		*tagsRaw = val
	case essenceKeyKind:
		*kind = val
	}
}

func validateParsedEssence(e harness.EssenceOut) error {
	const op = "mailreport.validateParsedEssence"
	missing, present, kindShort := essenceKeyInventory(e)
	if len(missing) > 0 {
		msg := strings.Join(missing, ", ") + " empty"
		if essenceHasWhyMissing(missing) {
			if slices.Contains(missing, essenceKeyWhy1) {
				msg = EssenceErrWhy1Empty
			} else {
				msg = EssenceErrWhyEmpty
			}
		} else if slices.Contains(missing, essenceKeyAbout) || slices.Contains(missing, essenceKeyTags) {
			msg = EssenceErrAboutOrShape
		}
		typed := sirerr.New(sirerr.CodeAI, op, msg).With(essenceFieldMissing, strings.Join(missing, ", "))
		if len(present) > 0 {
			typed = typed.With(essenceFieldPresent, strings.Join(present, ", "))
		}
		return typed
	}
	if kindShort {
		return sirerr.New(sirerr.CodeAI, op, EssenceErrKindShort).With(essenceFieldKindShort, essenceFieldKindShortYes)
	}
	return nil
}

func essenceKeyInventory(e harness.EssenceOut) (missing, present []string, kindShort bool) {
	for i, w := range e.Why {
		key := essenceWhyKeys[i]
		if strings.TrimSpace(w) == "" {
			missing = append(missing, key)
			continue
		}
		present = append(present, key)
	}
	if strings.TrimSpace(e.About) == "" {
		missing = append(missing, essenceKeyAbout)
	} else {
		present = append(present, essenceKeyAbout)
	}
	if strings.TrimSpace(e.Shape) == "" {
		missing = append(missing, essenceKeyTags)
	} else {
		present = append(present, essenceKeyTags)
	}
	kind := strings.TrimSpace(e.Kind)
	if kind == "" {
		missing = append(missing, essenceKeyKind)
		return missing, present, false
	}
	present = append(present, essenceKeyKind)
	return missing, present, len(catalog.ParseKindSegments(kind)) < 3
}

func essenceHasWhyMissing(missing []string) bool {
	for _, key := range missing {
		if slices.Contains(essenceWhyKeys, key) {
			return true
		}
	}
	return false
}

func normalizeKindString(kind string) string {
	segs := catalog.ParseKindSegments(kind)
	if len(segs) == 0 {
		return kind
	}
	var out []string
	for _, seg := range segs {
		out = append(out, slugKindSegment(seg))
	}
	return strings.Join(out, " > ")
}

func slugKindSegment(seg string) string {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return seg
	}
	if isKebabID(seg) {
		return seg
	}
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(seg) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && b.Len() > 0 {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}
