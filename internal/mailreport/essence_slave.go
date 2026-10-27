package mailreport

import (
	"slices"
	"strings"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"
	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	essenceSlaveWhy  = "why"
	essenceSlaveTags = "tags"
	essenceSlaveKind = "kind"

	essencePromptJoinSep = "\n---\n"
)

// EssencePrompts holds the three Essence integrity shell system prompts.
type EssencePrompts struct {
	Why  string
	Tags string
	Kind string
}

// JoinEssencePrompts fingerprints all three system prompts for essence_cache.
func JoinEssencePrompts(p EssencePrompts) string {
	return strings.TrimSpace(p.Why) + essencePromptJoinSep + strings.TrimSpace(p.Tags) + essencePromptJoinSep + strings.TrimSpace(p.Kind)
}

// EssencePromptsHash returns SHA-256 hex of the joined triple prompt.
func EssencePromptsHash(p EssencePrompts) string {
	return EssencePromptHash(JoinEssencePrompts(p))
}

// BuildEssencePrompts builds WHY, TAGS, and KIND slave system prompts.
func BuildEssencePrompts(activeTags []string) EssencePrompts {
	return EssencePrompts{
		Why:  buildEssenceWhySystemPrompt(),
		Tags: buildEssenceTagsSystemPrompt(activeTags),
		Kind: buildEssenceKindSystemPrompt(),
	}
}

func buildEssenceWhySystemPrompt() string {
	return `You extract a Five Whys chain for one email. Report only. Do not decide keep, discard, move, or unwanted.
Fill every required key. Do not add keys. Do not invent facts that are not in the message.
Do not write headings or labels in parentheses. Each output line starts with a key name, a colon, then the value.

Reply with these keys only, in this order:
WHY1:
WHY2:
WHY3:
WHY4:
WHY5:
ABOUT:

WHY1 is the surface fact this mail presents. Each later WHY answers why the previous WHY is true, using only this mail. If the mail does not support another cause, emit a short grounded restatement, not motive and not what the sender wants. Never start at WHY4. WHY4 and WHY5 continue the same chain; they are not a second summary.
ABOUT is one sentence of what this message is. Include the event, the subject or source, and one concrete fact from the message when available, such as a version, PR number, amount, named service, requested action, or stated cause.
Prefer concrete facts over generic wording such as "this is a notification" or "someone left a comment." Preserve names, numbers, versions, amounts, and causes exactly as stated. If the message does not state a cause, do not infer one. Do not omit the event just to add a detail.
Not a path. Not a tag list. Not a KIND.`
}

func buildEssenceTagsSystemPrompt(activeTags []string) string {
	var tagList strings.Builder
	tagList.WriteString("Active tags: ")
	if len(activeTags) > 0 {
		tagList.WriteString(strings.Join(activeTags, ", "))
	}
	return `You assign mail-form tags for one email. Report only. Do not decide keep, discard, move, or unwanted.
Fill the TAGS key only. Do not add other keys. Do not invent facts that are not in the message or the WHY chain.

Reply with this key only:
TAGS:

TAGS are reusable mail-form ids such as receipt, newsletter, calendar-invite, billing, or github-notification, not a topic tree and not a KIND path.
Pick 1 to 5 kebab-case ids that actually fit this mail. Prefer Active tags when they fit. You may mint one new kebab-case tag if none fit.
Do not dump the Active tags list. Do not describe layout or geometry. Do not emit paths that contain " > ".
` + tagList.String()
}

func buildEssenceKindSystemPrompt() string {
	return `You assign a catalog KIND path for one email. Report only. Do not decide keep, discard, move, or unwanted.
Fill the KIND key only. Do not add other keys. Do not invent facts that are not in the message or the WHY chain.

Reply with this key only:
KIND:

` + essenceKindContractText() + `
If the honest class is ci, the second level is ci, not ci-failure. Do not paraphrase WHY1 as KIND. Do not copy TAGS into KIND. Do not copy a path from these instructions.`
}

// formatEssenceWhyBlock formats WHY1-5 and ABOUT for TAGS and KIND user turns.
func formatEssenceWhyBlock(out harness.EssenceOut) string {
	var b strings.Builder
	for i, key := range essenceWhyKeys {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(out.Why[i]))
		b.WriteByte('\n')
	}
	b.WriteString(essenceKeyAbout)
	b.WriteString(": ")
	b.WriteString(strings.TrimSpace(out.About))
	return strings.TrimRight(b.String(), "\n")
}

func parseEssenceWhy(raw string) (harness.EssenceOut, error) {
	var why [5]string
	about := ""
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var pendingKey string
	var pendingVal []string
	flush := func() {
		if pendingKey == "" {
			return
		}
		val := strings.TrimSpace(strings.Join(pendingVal, "\n"))
		switch pendingKey {
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
			about = val
		}
		pendingKey = ""
		pendingVal = nil
	}
	for _, line := range lines {
		if key, rest, ok := matchEssenceKeyLine(line); ok {
			if !isEssenceWhyParseKey(key) {
				flush()
				continue
			}
			flush()
			pendingKey = key
			if rest != "" {
				pendingVal = []string{rest}
			}
			continue
		}
		if pendingKey != "" && strings.TrimSpace(line) != "" {
			pendingVal = append(pendingVal, strings.TrimSpace(line))
		}
	}
	flush()
	out := harness.EssenceOut{Why: why, About: about}
	return out, validateEssenceWhy(out)
}

func isEssenceWhyParseKey(key string) bool {
	switch key {
	case essenceKeyWhy1, essenceKeyWhy2, essenceKeyWhy3, essenceKeyWhy4, essenceKeyWhy5, essenceKeyAbout:
		return true
	default:
		return false
	}
}

func parseEssenceTags(raw string, cat *tags.Catalog) (string, error) {
	const op = "mailreport.parseEssenceTags"
	tagsRaw := ""
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var pendingKey string
	var pendingVal []string
	flush := func() {
		if pendingKey == "" {
			return
		}
		if pendingKey == essenceKeyTags || pendingKey == essenceKeyForm || pendingKey == essenceKeyShape {
			tagsRaw = strings.TrimSpace(strings.Join(pendingVal, "\n"))
		}
		pendingKey = ""
		pendingVal = nil
	}
	for _, line := range lines {
		if key, rest, ok := matchEssenceKeyLine(line); ok {
			flush()
			pendingKey = key
			if rest != "" {
				pendingVal = []string{rest}
			}
			continue
		}
		if pendingKey != "" && strings.TrimSpace(line) != "" {
			pendingVal = append(pendingVal, strings.TrimSpace(line))
		}
	}
	flush()
	shape, err := normalizeEssenceTags(cat, tagsRaw)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(shape) == "" {
		return "", sirerr.New(sirerr.CodeAI, op, EssenceErrTagsEmpty).With(essenceFieldMissing, essenceKeyTags)
	}
	return shape, nil
}

func parseEssenceKind(raw string) (string, error) {
	const op = "mailreport.parseEssenceKind"
	kind := ""
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var pendingKey string
	var pendingVal []string
	flush := func() {
		if pendingKey == "" {
			return
		}
		if pendingKey == essenceKeyKind {
			kind = strings.TrimSpace(strings.Join(pendingVal, "\n"))
		}
		pendingKey = ""
		pendingVal = nil
	}
	for _, line := range lines {
		if key, rest, ok := matchEssenceKeyLine(line); ok {
			flush()
			pendingKey = key
			if rest != "" {
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
	segs := catalog.ParseKindSegments(kind)
	if len(segs) < essenceKindLevels {
		return "", sirerr.New(sirerr.CodeAI, op, EssenceErrKindShort).With(essenceFieldKindShort, essenceFieldKindShortYes)
	}
	if len(segs) > essenceKindLevels {
		return "", sirerr.New(sirerr.CodeAI, op, EssenceErrKindLong).With(essenceFieldKindLong, essenceFieldKindShortYes)
	}
	if kindOrPathRepeatsParent(segs) {
		return "", sirerr.New(sirerr.CodeAI, op, EssenceErrKindAncestor)
	}
	return kind, nil
}

func validateEssenceWhy(e harness.EssenceOut) error {
	const op = "mailreport.validateEssenceWhy"
	missing, present := essenceWhyKeyInventory(e)
	if len(missing) == 0 {
		return nil
	}
	msg := strings.Join(missing, ", ") + " empty"
	if essenceHasWhyMissing(missing) {
		if slices.Contains(missing, essenceKeyWhy1) {
			msg = EssenceErrWhy1Empty
		} else {
			msg = EssenceErrWhyEmpty
		}
	} else if slices.Contains(missing, essenceKeyAbout) {
		msg = EssenceErrAboutOrShape
	}
	typed := sirerr.New(sirerr.CodeAI, op, msg).With(essenceFieldMissing, strings.Join(missing, ", "))
	if len(present) > 0 {
		typed = typed.With(essenceFieldPresent, strings.Join(present, ", "))
	}
	return typed
}

func essenceWhyKeyInventory(e harness.EssenceOut) (missing, present []string) {
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
	return missing, present
}

func essenceWhyIntegrityValidate(cat *tags.Catalog) func(string) error {
	_ = cat
	return func(raw string) error {
		_, err := parseEssenceWhy(raw)
		return err
	}
}

func essenceTagsIntegrityValidate(cat *tags.Catalog) func(string) error {
	return func(raw string) error {
		_, err := parseEssenceTags(raw, cat)
		return err
	}
}

func essenceKindIntegrityValidate(cat *tags.Catalog) func(string) error {
	_ = cat
	return func(raw string) error {
		_, err := parseEssenceKind(raw)
		return err
	}
}

func essenceWhyIntegrityCompleteNote(err error) string {
	missing, present := essenceKeysFromErr(err)
	if len(missing) > 0 {
		if essenceStartedAtLaterWhy(missing, present) {
			return "Continue your previous reply. Add WHY1, WHY2, and WHY3 as the first three reasons. Keep WHY4 and WHY5 as those same keys; do not rename WHY4 to WHY1. Keep ABOUT. Fill every missing key: " + strings.Join(missing, ", ") + "."
		}
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
	return "Continue your previous reply. Emit WHY1 through WHY5 and ABOUT with non-empty values."
}

func essenceTagsIntegrityCompleteNote(err error) string {
	if essenceTagInvalidFromErr(err) {
		return "Continue your previous reply. Fix TAGS: use kebab-case tag ids only (no geometry words, no KIND paths). Prefer listed Active tags or mint a new kebab tag."
	}
	return "Continue your previous reply. Add TAGS with 1 to 5 kebab-case mail-form ids."
}

func essenceKindIntegrityCompleteNote(err error) string {
	if essenceKindShortFromErr(err) {
		return `Continue your previous reply. Expand KIND to ` + essenceKindRetryContract() + `. Do not leave a trailing " > ".`
	}
	if essenceKindLongFromErr(err) {
		return `Continue your previous reply. Drop extra KIND levels. Use ` + essenceKindRetryContract() + `. Do not add a 4th level.`
	}
	if essenceKindAncestorFromErr(err) {
		return `Continue your previous reply. Rewrite KIND so no segment repeats its parent as a prefix (use ci not notification-ci; use failure not ci-failure when the parent is already ci).`
	}
	return `Continue your previous reply. Emit KIND with ` + essenceKindRetryContract() + `.`
}

func essenceWhyIntegrityAfterMiss(err error) integrity.MissAction {
	missing, _ := essenceKeysFromErr(err)
	for _, key := range missing {
		if key == essenceKeyWhy1 {
			return integrity.MissRestart
		}
	}
	return integrity.MissContinue
}

func essenceTagsIntegrityAfterMiss(_ error) integrity.MissAction {
	return integrity.MissContinue
}

func essenceKindIntegrityAfterMiss(_ error) integrity.MissAction {
	return integrity.MissContinue
}

func essenceFullFormShellHooks(cat *tags.Catalog) essenceShellHooks {
	return essenceShellHooks{
		Validate:     essenceIntegrityValidateFor(cat),
		CompleteNote: essenceIntegrityCompleteNote,
		AfterMiss:    essenceIntegrityAfterMiss,
	}
}

func essenceWhyShellHooks(cat *tags.Catalog) essenceShellHooks {
	return essenceShellHooks{
		Validate:     essenceWhyIntegrityValidate(cat),
		CompleteNote: essenceWhyIntegrityCompleteNote,
		AfterMiss:    essenceWhyIntegrityAfterMiss,
	}
}

func essenceTagsShellHooks(cat *tags.Catalog) essenceShellHooks {
	return essenceShellHooks{
		Validate:     essenceTagsIntegrityValidate(cat),
		CompleteNote: essenceTagsIntegrityCompleteNote,
		AfterMiss:    essenceTagsIntegrityAfterMiss,
	}
}

func essenceKindShellHooks(cat *tags.Catalog) essenceShellHooks {
	return essenceShellHooks{
		Validate:     essenceKindIntegrityValidate(cat),
		CompleteNote: essenceKindIntegrityCompleteNote,
		AfterMiss:    essenceKindIntegrityAfterMiss,
	}
}
