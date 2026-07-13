package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.4.0", "v1.3.1", true},
		{"v1.3.1", "v1.3.1", false},
		{"v1.3.0", "v1.3.1", false},
		{"1.10.0", "1.9.9", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.3.2", "v1.3.1-3-gabc-dirty", true},
		{"v1.3.1", "dev", false},
		{"", "v1.0.0", false},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if got := normalize("v1.3.1-3-gabcdef-dirty"); got != "1.3.1" {
		t.Fatalf("normalize = %q", got)
	}
}
