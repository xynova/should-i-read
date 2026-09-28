package secret

import (
	"errors"
	"testing"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
)

func TestResolveEnvWins(t *testing.T) {
	t.Setenv("SIR_TEST_SECRET_ENV", "from-env")
	got, err := Resolve("SIR_TEST_SECRET_ENV")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveFromMemKeyring(t *testing.T) {
	t.Setenv("SIR_TEST_SECRET_KR", "")
	mem := operatorconfig.NewMemKeyring()
	prev := operatorconfig.DefaultKeyring()
	operatorconfig.SetTestKeyring(mem)
	t.Cleanup(func() { operatorconfig.SetTestKeyring(prev) })

	if err := mem.Set(ServiceName, "SIR_TEST_SECRET_KR", "from-keyring"); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve("SIR_TEST_SECRET_KR")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-keyring" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveMissing(t *testing.T) {
	t.Setenv("SIR_TEST_SECRET_MISSING", "")
	_, err := Resolve("SIR_TEST_SECRET_MISSING_XYZ_NEVER")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestResolveEmptyName(t *testing.T) {
	_, err := Resolve("  ")
	if err == nil {
		t.Fatal("expected error")
	}
}
