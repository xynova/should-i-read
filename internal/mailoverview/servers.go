package mailoverview

import (
	"sync"

	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/mailack"
	"github.com/xynova/should-i-read/internal/maildelete"
)

// PageServer holds overview HTML, hashes, preview, sync, and events.
type PageServer struct {
	pageMu  sync.RWMutex
	HTML    []byte
	Hashes  []string
	Preview MailPreviewer
	Sync    *SyncController
	Events  *hostevent.Bus
}

// CartServer is the report-only deletion cart and trash jobs.
type CartServer struct {
	Cart     CartStore
	Jobs     *maildelete.Service
	Actioned ActionedLookup
}

// AckServer is the Acked cart and seen-at-source jobs.
type AckServer struct {
	AckCart  AckCartStore
	AckJobs  *mailack.Service
	Outcomes OutcomeStore
}

// IntentServer is POST /intent guess.
type IntentServer struct {
	Guess *IntentGuesser
}
