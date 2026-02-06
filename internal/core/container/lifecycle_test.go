package container

import "testing"

func TestGenerateContainerID(t *testing.T) {
	a := GenerateContainerID()
	b := GenerateContainerID()

	if a == b {
		t.Fatalf("expected unique IDs, got same: %s", a)
	}
	if len(a) <= len("Qube-") || a[:5] != "Qube-" {
		t.Fatalf("unexpected prefix/length: %s", a)
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{5, "5s"},
		{120, "2m"},
		{3600 + 120, "1h 2m"},
		{86400 + 7200, "1d 2h"},
	}

	for _, tc := range cases {
		if got := formatUptime(tc.in); got != tc.want {
			t.Fatalf("formatUptime(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
