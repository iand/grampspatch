package grampspatch

import (
	"encoding/json"
	"testing"
)

func TestPointer(t *testing.T) {
	tests := []struct {
		segments []string
		want     string
	}{
		{nil, ""},
		{[]string{"shared_cm"}, "/shared_cm"},
		{[]string{"event_ref_list", "0", "role"}, "/event_ref_list/0/role"},
		{[]string{"a/b"}, "/a~1b"},
		{[]string{"a~b"}, "/a~0b"},
		{[]string{"~/"}, "/~0~1"},
		{[]string{""}, "/"},
	}
	for _, tc := range tests {
		if got := Pointer(tc.segments...); got != tc.want {
			t.Errorf("Pointer(%q) = %q, want %q", tc.segments, got, tc.want)
		}
	}
}

func TestOpEncoding(t *testing.T) {
	tests := []struct {
		name string
		op   func() (Op, error)
		want string
	}{
		{"test", func() (Op, error) { return Test("/shared_cm", 2547.3) }, `{"op":"test","path":"/shared_cm","value":2547.3}`},
		{"replace", func() (Op, error) { return Replace("/page", "p") }, `{"op":"replace","path":"/page","value":"p"}`},
		{"add", func() (Op, error) { return AddValue("/note_list/-", "N1") }, `{"op":"add","path":"/note_list/-","value":"N1"}`},
		{"remove", func() (Op, error) { return Remove("/note_list/0"), nil }, `{"op":"remove","path":"/note_list/0"}`},
		{"null value kept", func() (Op, error) { return Replace("/source_handle", nil) }, `{"op":"replace","path":"/source_handle","value":null}`},
		{"raw value kept", func() (Op, error) { return Test("/shared_cm", json.RawMessage(`0.0`)) }, `{"op":"test","path":"/shared_cm","value":0.0}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := tc.op()
			if err != nil {
				t.Fatalf("building op: unexpected error: %v", err)
			}
			got, err := json.Marshal(op)
			if err != nil {
				t.Fatalf("Marshal: unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestOpValueError(t *testing.T) {
	if _, err := Replace("/x", func() {}); err == nil {
		t.Errorf("expected error for a value that does not encode, got nil")
	}
}
