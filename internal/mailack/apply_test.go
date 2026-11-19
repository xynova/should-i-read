package mailack

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
)

type recordingFlagger struct {
	folder string
	env    string
	flag   string
}

func (r *recordingFlagger) FlagAdd(_ context.Context, folder, envelopeID, flag string) (*pimalaya.Result, error) {
	r.folder = folder
	r.env = envelopeID
	r.flag = flag
	return &pimalaya.Result{Raw: json.RawMessage(`{"data":{"flags":[{"raw":"\\Seen","iana":"seen"}],"count":1,"queued":true}}`)}, nil
}

func TestApplySeenSuccessUpsertsOutcome(t *testing.T) {
	st, ctx := openTestStore(t)
	if err := st.AddAckCart(ctx, []hoststore.AckCartItem{{ObjectHash: "h1", Sender: "a@x"}}); err != nil {
		t.Fatal(err)
	}
	jobID, err := st.CreateAckJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BindAckJobItems(ctx, jobID, []hoststore.AckJobBinding{
		{ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "1", Status: hoststore.AckJobItemPlanned},
	}); err != nil {
		t.Fatal(err)
	}
	flagger := &recordingFlagger{}
	svc, err := CreateAckJobService(JobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Flagger: flagger,
		Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, jobID, hoststore.AckJobConfirmToken); err != nil {
		t.Fatal(err)
	}
	if flagger.folder != "INBOX" || flagger.env != "1" || flagger.flag != pimalaya.FlagSeen {
		t.Fatalf("flagger %+v", flagger)
	}
	got, ok, err := st.GetOutcome(ctx, "h1")
	if err != nil || !ok || got.Outcome != hoststore.OutcomeAcked {
		t.Fatalf("outcome %+v ok %v err %v", got, ok, err)
	}
	cart, err := st.ListAckCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart) != 0 {
		t.Fatalf("cart leftover %+v", cart)
	}
}

func TestApplyRecordsFlagAddSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	st, ctx := openTestStore(t)
	_ = st.AddAckCart(ctx, []hoststore.AckCartItem{{ObjectHash: "h1"}})
	jobID, err := st.CreateAckJobFromCart(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.BindAckJobItems(ctx, jobID, []hoststore.AckJobBinding{
		{ObjectHash: "h1", Folder: "INBOX", EnvelopeID: "9", Status: hoststore.AckJobItemPlanned},
	})
	svc, err := CreateAckJobService(JobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Flagger: &recordingFlagger{},
		Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, jobID, hoststore.AckJobConfirmToken); err != nil {
		t.Fatal(err)
	}
	var item sdktrace.ReadOnlySpan
	for _, sp := range sr.Ended() {
		if sp.Name() == "mailack.FlagAdd" {
			item = sp
			break
		}
	}
	if item == nil {
		t.Fatal("missing mailack.FlagAdd span")
	}
	attrs := attribute.NewSet(item.Attributes()...)
	gotFolder, _ := attrs.Value(attribute.Key("ack.folder"))
	gotEnv, _ := attrs.Value(attribute.Key("ack.envelope_id"))
	if gotFolder.AsString() != "INBOX" || gotEnv.AsString() != "9" {
		t.Fatalf("attrs folder=%q env=%q", gotFolder.AsString(), gotEnv.AsString())
	}
}

func TestApplyRejectsWrongConfirm(t *testing.T) {
	st, ctx := openTestStore(t)
	svc, err := CreateAckJobService(JobConfig{
		Store: st, Pimdir: &pimdir.Reader{}, Flagger: &recordingFlagger{},
		Now: time.Now, ItemTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Apply(ctx, "nope", "trash-at-source"); err == nil {
		t.Fatal("expected confirm mismatch")
	}
}

func openTestStore(t *testing.T) (*hoststore.Store, context.Context) {
	t.Helper()
	path := t.TempDir() + "/ledger.db"
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: path, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = st.Close() })
	return st, ctx
}
