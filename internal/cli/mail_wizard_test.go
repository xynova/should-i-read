package cli

import "testing"

func TestMailSetupUseWizard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		tty        bool
		wizard     bool
		prov, em   bool
		wantWizard bool
	}{
		{"non_tty", false, false, false, false, false},
		{"automation_flags", true, false, true, true, false},
		{"partial_flags", true, false, true, false, true},
		{"force_wizard", true, true, true, true, true},
		{"tty_no_flags", true, false, false, false, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mailSetupUseWizard(tc.tty, tc.wizard, tc.prov, tc.em)
			if got != tc.wantWizard {
				t.Fatalf("got %v want %v", got, tc.wantWizard)
			}
		})
	}
}
