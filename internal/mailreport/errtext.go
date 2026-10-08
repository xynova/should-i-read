package mailreport

import (
	"fmt"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// formatRowError stores a stable code prefix for report rows while keeping the full message.
func formatRowError(err error) string {
	if err == nil {
		return ""
	}
	if code := harness.CodeOf(err); code != "" {
		return fmt.Sprintf("[%s] %s", code, err.Error())
	}
	if c, ok := sirerr.AsCode(err); ok {
		return fmt.Sprintf("[%s] %s", c, err.Error())
	}
	return err.Error()
}
