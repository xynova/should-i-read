package hoststore

import (
	"testing"
	"time"
)

func TestAckCartRoundTrip(t *testing.T) {
	st, ctx := testStore(t)
	if err := st.AddAckCart(ctx, []AckCartItem{
		{ObjectHash: "h1", Sender: "a@x", Subject: "Hi", MailedAt: "2026-10-01T00:00:00Z"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListAckCart(ctx)
	if err != nil || len(got) != 1 || got[0].ObjectHash != "h1" {
		t.Fatalf("list %+v err %v", got, err)
	}
	jobID, err := st.CreateAckJobFromCart(ctx, time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	job, err := st.GetAckJob(ctx, jobID)
	if err != nil || len(job.Items) != 1 || job.ConfirmToken != AckJobConfirmToken {
		t.Fatalf("job %+v err %v", job, err)
	}
	if err := st.ClearAckCart(ctx); err != nil {
		t.Fatal(err)
	}
	left, err := st.ListAckCart(ctx)
	if err != nil || len(left) != 0 {
		t.Fatalf("cleared %+v err %v", left, err)
	}
}
