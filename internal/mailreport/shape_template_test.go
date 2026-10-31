package mailreport

import "testing"

func TestShapeTemplateGitLabPipeline(t *testing.T) {
	tmpl, err := CreateSubjectTemplater(SubjectTemplaterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	a := tmpl.Template("Acme | Failed pipeline for main | deadbeef")
	b := tmpl.Template("Acme | Failed pipeline for main | cafebabe")
	if a != b {
		t.Fatalf("a=%q b=%q", a, b)
	}
	if !HasSlot(a) {
		t.Fatalf("want slot in %q", a)
	}
}

func TestShapeTemplateNoSlot(t *testing.T) {
	tmpl, err := CreateSubjectTemplater(SubjectTemplaterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	s := tmpl.Template("Hello from Jane")
	if HasSlot(s) {
		t.Fatalf("unexpected slot in %q", s)
	}
}

func TestShapeFingerprintStable(t *testing.T) {
	tmpl, err := CreateSubjectTemplater(SubjectTemplaterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	domain := senderDomain("GitLab <incoming@mg.gitlab.com>")
	sub := tmpl.Template("Failed pipeline for branch | abcdef0123456789")
	fp1 := ShapeFingerprint(domain, sub)
	fp2 := ShapeFingerprint(domain, sub)
	if fp1 == "" || fp1 != fp2 {
		t.Fatalf("fp1=%q fp2=%q domain=%q sub=%q", fp1, fp2, domain, sub)
	}
}
