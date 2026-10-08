package mailreport

import (
	"context"

	"github.com/behaviorengineering/taxonomy/pkg/harness"
)

// noopAuthor declines to draft; Operate returns CodeInvalidDraft which the runner treats as skip when AuthorOnSkip is false.
type noopAuthor struct{}

// NoopAuthor returns a harness Author that never calls chat.
func NoopAuthor() harness.Author { return noopAuthor{} }

func (noopAuthor) Draft(_ context.Context, _ harness.DraftIn) (harness.DraftOut, error) {
	return harness.DraftOut{}, nil
}
