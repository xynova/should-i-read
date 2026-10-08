package mailreport

import "testing"

func TestParseEssenceResponse_sameLine(t *testing.T) {
	raw := `WHY1: a
WHY2: b
WHY3: c
WHY4: d
WHY5: e
ABOUT: about text
SHAPE: shape text
KIND: notification > software-release > changelog`
	out, err := parseEssenceResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Kind != "notification > software-release > changelog" {
		t.Fatalf("%q", out.Kind)
	}
}

func TestParseEssenceResponse_nextLine(t *testing.T) {
	raw := `WHY1:
line one
WHY2: b
WHY3: c
WHY4: d
WHY5: e
ABOUT: about
SHAPE: shape
KIND: a > b > c`
	out, err := parseEssenceResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Why[0] != "line one" {
		t.Fatalf("%q", out.Why[0])
	}
}

func TestParseEssenceResponse_rejectShortKind(t *testing.T) {
	_, err := parseEssenceResponse(`WHY1: a
WHY2: b
WHY3: c
WHY4: d
WHY5: e
ABOUT: x
SHAPE: y
KIND: a > b`)
	if err == nil {
		t.Fatal("expected error")
	}
}
