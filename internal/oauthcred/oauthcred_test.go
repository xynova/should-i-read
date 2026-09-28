package oauthcred

import (
	"errors"
	"testing"
)

func TestResolveEnvWinsOverProduct(t *testing.T) {
	t.Setenv(EnvGmailClientID, "from-env")
	prev := ProductGmailClientID
	ProductGmailClientID = "from-product"
	t.Cleanup(func() { ProductGmailClientID = prev })

	got, err := Resolve(EnvGmailClientID)
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveProductFallback(t *testing.T) {
	t.Setenv(EnvOutlookClientID, "")
	prev := ProductOutlookClientID
	ProductOutlookClientID = "outlook-product-id"
	t.Cleanup(func() { ProductOutlookClientID = prev })

	got, err := Resolve(EnvOutlookClientID)
	if err != nil {
		t.Fatal(err)
	}
	if got != "outlook-product-id" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveMissing(t *testing.T) {
	t.Setenv(EnvGmailClientID, "")
	prev := ProductGmailClientID
	ProductGmailClientID = ""
	t.Cleanup(func() { ProductGmailClientID = prev })

	_, err := Resolve(EnvGmailClientID)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestStatusRedacted(t *testing.T) {
	t.Setenv(EnvGmailClientID, "id")
	t.Setenv(EnvGmailClientSecret, "")
	prev := ProductOutlookClientID
	ProductOutlookClientID = ""
	t.Cleanup(func() { ProductOutlookClientID = prev })

	st := Status()
	if st[EnvGmailClientID] != "(set)" {
		t.Fatalf("gmail id: %q", st[EnvGmailClientID])
	}
	if st[EnvGmailClientSecret] != "(unset)" {
		t.Fatalf("gmail secret: %q", st[EnvGmailClientSecret])
	}
}

func TestHasAny(t *testing.T) {
	t.Setenv(EnvGmailClientID, "")
	t.Setenv(EnvOutlookClientID, "")
	prevG := ProductGmailClientID
	prevO := ProductOutlookClientID
	ProductGmailClientID = ""
	ProductOutlookClientID = ""
	t.Cleanup(func() {
		ProductGmailClientID = prevG
		ProductOutlookClientID = prevO
	})
	if HasAny() {
		t.Fatal("expected HasAny false")
	}
	ProductOutlookClientID = "outlook-product"
	if !HasAny() {
		t.Fatal("expected HasAny true from product outlook")
	}
}
