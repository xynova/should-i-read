package mailreport

import (
	"testing"
	"time"
)

func TestStropEmbedHTTPTimeout_underOperateAttachBudget(t *testing.T) {
	t.Parallel()
	if polypusEmbedHopTimeout != 60*time.Second {
		t.Fatalf("polypus hop %s", polypusEmbedHopTimeout)
	}
	if stropEmbedHTTPTimeout != 75*time.Second {
		t.Fatalf("timeout %s", stropEmbedHTTPTimeout)
	}
	if embedRequestTimeout(embedChunkSize) >= stropEmbedHTTPTimeout {
		t.Fatalf("chunk deadline %s must stay below HTTP cap %s", embedRequestTimeout(embedChunkSize), stropEmbedHTTPTimeout)
	}
	if embedTimeoutMax >= polypusEmbedHopTimeout {
		t.Fatalf("max chunk hop %s must stay under default embed hop %s", embedTimeoutMax, polypusEmbedHopTimeout)
	}
	if stropEmbedHTTPTimeout <= polypusEmbedHopTimeout {
		t.Fatalf("host timeout %s must stay above Polypus embed hop %s", stropEmbedHTTPTimeout, polypusEmbedHopTimeout)
	}
	if stropEmbedHTTPTimeout >= operateTimeoutAttach {
		t.Fatalf("embed timeout %s must stay below operate %s", stropEmbedHTTPTimeout, operateTimeoutAttach)
	}
	attempts := 1 + embedRetryMax
	if time.Duration(attempts)*stropEmbedHTTPTimeout >= operateTimeoutAttach {
		t.Fatalf("%d embed attempts at %s would exhaust operate %s", attempts, stropEmbedHTTPTimeout, operateTimeoutAttach)
	}
	if embedRetryMax != 0 {
		t.Fatalf("embedRetryMax %d; long hops must not retry", embedRetryMax)
	}
}
