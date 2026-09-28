package emailops_test

import (
	"testing"

	"github.com/xynova/should-i-read/internal/emailops"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestParseEnvelopeSuccess(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"ok":true,"data":{"ready":true},"error":null}`)
	env, err := emailops.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if !env.OK {
		t.Fatal("expected ok")
	}
	if string(env.Data) != `{"ready":true}` {
		t.Fatalf("data = %s", env.Data)
	}
}

func TestParseEnvelopeFailure(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"ok":false,"data":null,"error":{"code":"not_found","message":"missing","params":{}}}`)
	env, err := emailops.ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope: %v", err)
	}
	if env.OK {
		t.Fatal("expected !ok")
	}
	if env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("error = %+v", env.Error)
	}
}

func TestParseEnvelopeEmpty(t *testing.T) {
	t.Parallel()
	_, err := emailops.ParseEnvelope(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeFailed {
		t.Fatalf("code = %v ok=%v", code, ok)
	}
}

func TestCreateRejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := emailops.Create("", "/tmp"); err == nil {
		t.Fatal("expected error for empty bin")
	}
	if _, err := emailops.Create("/bin/true", ""); err == nil {
		t.Fatal("expected error for empty data dir")
	}
}
