package grampspatch

import (
	"regexp"
	"testing"
)

func TestHandle(t *testing.T) {
	a, err := Handle("MH", "DNATest", "49A9AAED", "D96F9AAD")
	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	b, err := Handle("MH", "DNATest", "49A9AAED", "D96F9AAD")
	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	if a != b {
		t.Errorf("same tokens gave %q and %q", a, b)
	}
	if !regexp.MustCompile(`^MH[A-Z2-7]{16}$`).MatchString(a) {
		t.Errorf("handle %q does not have the form MH plus 16 base-32 characters", a)
	}

	c, err := Handle("MH", "DNAMatch", "49A9AAED", "D96F9AAD")
	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	if c == a {
		t.Errorf("different tokens gave the same handle %q", a)
	}

	x, err := Handle("MH", "ab", "c")
	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	y, err := Handle("MH", "a", "bc")
	if err != nil {
		t.Fatalf("Handle: unexpected error: %v", err)
	}
	if x == y {
		t.Errorf("token boundaries ignored: both gave %q", x)
	}
}

func TestHandlePrefix(t *testing.T) {
	for _, prefix := range []string{"", "mh", "M_H", "ABCDEFGHI", "M H"} {
		if _, err := Handle(prefix, "x"); err == nil {
			t.Errorf("Handle(%q): expected error, got nil", prefix)
		}
	}
	for _, prefix := range []string{"M", "MH", "FTDNA", "ABCDEFGH", "A1"} {
		if _, err := Handle(prefix, "x"); err != nil {
			t.Errorf("Handle(%q): unexpected error: %v", prefix, err)
		}
	}
}
