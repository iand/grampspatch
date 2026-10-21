package grampspatch

import (
	"encoding/json"
	"strings"
	"testing"
)

// testMatch is a cut-down object type used to exercise Diff.
type testMatch struct {
	Class        string        `json:"_class"`
	Handle       string        `json:"handle"`
	Change       int64         `json:"change"`
	SharedCM     float64       `json:"shared_cm"`
	SegmentCount int           `json:"segment_count"`
	Private      bool          `json:"private"`
	NoteList     []string      `json:"note_list"`
	Provider     testType      `json:"provider"`
	Segments     []testSegment `json:"segment_list"`
}

type testType struct {
	Class  string `json:"_class"`
	Value  int    `json:"value"`
	String string `json:"string"`
}

type testSegment struct {
	Class    string  `json:"_class"`
	SharedCM float64 `json:"shared_cm"`
}

// storedMatch is the stored form of baseMatch, with members in a different
// order from the Go type and floats written as Python writes them.
const storedMatch = `{"handle": "M1", "change": 1790000000, "_class": "DNAMatch",
 "private": false, "shared_cm": 117.8, "segment_count": 0, "note_list": ["N1"],
 "provider": {"string": "", "value": 3, "_class": "DNAProviderType"},
 "segment_list": [{"_class": "DNASegment", "shared_cm": 20.0}]}`

func baseMatch() testMatch {
	return testMatch{
		Class:        "DNAMatch",
		Handle:       "M1",
		Change:       1790000000,
		SharedCM:     117.8,
		SegmentCount: 0,
		NoteList:     []string{"N1"},
		Provider:     testType{Class: "DNAProviderType", Value: 3},
		Segments:     []testSegment{{Class: "DNASegment", SharedCM: 20}},
	}
}

func opsJSON(t *testing.T, ops []Op) string {
	t.Helper()
	data, err := json.Marshal(ops)
	if err != nil {
		t.Fatalf("Marshal: unexpected error: %v", err)
	}
	return string(data)
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name   string
		modify func(m *testMatch)
		want   string
	}{
		{
			name:   "no change",
			modify: func(m *testMatch) {},
			want:   `[]`,
		},
		{
			name:   "scalar change",
			modify: func(m *testMatch) { m.SharedCM = 177.8 },
			want:   `[{"op":"test","path":"/shared_cm","value":117.8},{"op":"replace","path":"/shared_cm","value":177.8}]`,
		},
		{
			name:   "list change",
			modify: func(m *testMatch) { m.NoteList = append(m.NoteList, "N2") },
			want:   `[{"op":"test","path":"/note_list","value":["N1"]},{"op":"replace","path":"/note_list","value":["N1","N2"]}]`,
		},
		{
			name:   "nested change emitted at top-level member",
			modify: func(m *testMatch) { m.Segments[0].SharedCM = 21.5 },
			want:   `[{"op":"test","path":"/segment_list","value":[{"_class":"DNASegment","shared_cm":20.0}]},{"op":"replace","path":"/segment_list","value":[{"_class":"DNASegment","shared_cm":21.5}]}]`,
		},
		{
			name: "members in stored order",
			modify: func(m *testMatch) {
				m.SegmentCount = 3
				m.Private = true
			},
			want: `[{"op":"test","path":"/private","value":false},{"op":"replace","path":"/private","value":true},{"op":"test","path":"/segment_count","value":0},{"op":"replace","path":"/segment_count","value":3}]`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := baseMatch()
			after := baseMatch()
			tc.modify(&after)
			ops, err := Diff(json.RawMessage(storedMatch), before, after)
			if err != nil {
				t.Fatalf("Diff: unexpected error: %v", err)
			}
			if got := opsJSON(t, ops); got != tc.want {
				t.Errorf("ops =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestDiffErrors(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		before any
		after  any
		want   string // want is a substring of the error
	}{
		{
			name:   "before differs from stored",
			stored: storedMatch,
			before: func() testMatch { m := baseMatch(); m.SharedCM = 1; return m }(),
			after:  baseMatch(),
			want:   "/shared_cm",
		},
		{
			name:   "boolean against number",
			stored: strings.Replace(storedMatch, `"private": false`, `"private": 0`, 1),
			before: baseMatch(),
			after:  baseMatch(),
			want:   "/private",
		},
		{
			name:   "member missing from after",
			stored: `{"_class": "X", "handle": "H", "a": 1}`,
			before: map[string]any{"_class": "X", "handle": "H", "a": 1},
			after:  map[string]any{"_class": "X", "handle": "H"},
			want:   `"a"`,
		},
		{
			name:   "member added in after",
			stored: `{"_class": "X", "handle": "H"}`,
			before: map[string]any{"_class": "X", "handle": "H"},
			after:  map[string]any{"_class": "X", "handle": "H", "b": 1},
			want:   `"b"`,
		},
		{
			name:   "handle changed",
			stored: storedMatch,
			before: baseMatch(),
			after:  func() testMatch { m := baseMatch(); m.Handle = "M2"; return m }(),
			want:   "handle",
		},
		{
			name:   "change changed",
			stored: storedMatch,
			before: baseMatch(),
			after:  func() testMatch { m := baseMatch(); m.Change = 1; return m }(),
			want:   "change",
		},
		{
			name:   "stored not an object",
			stored: `[1]`,
			before: []int{1},
			after:  []int{1},
			want:   "object",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Diff(json.RawMessage(tc.stored), tc.before, tc.after)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestEqualJSON(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{`1`, `1.0`, true},
		{`117.8`, `117.80`, true},
		{`1e2`, `100`, true},
		{`9007199254740993`, `9007199254740992`, false},
		{`false`, `0`, false},
		{`true`, `1`, false},
		{`null`, `""`, false},
		{`null`, `null`, true},
		{`"a"`, `"a"`, true},
		{`{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{`{"a":1}`, `{"a":1,"b":2}`, false},
		{`[1,2]`, `[2,1]`, false},
		{`[1,2]`, `[1,2,3]`, false},
		{`[]`, `{}`, false},
	}
	for _, tc := range tests {
		av, err := decodeValue([]byte(tc.a))
		if err != nil {
			t.Fatalf("decoding %s: unexpected error: %v", tc.a, err)
		}
		bv, err := decodeValue([]byte(tc.b))
		if err != nil {
			t.Fatalf("decoding %s: unexpected error: %v", tc.b, err)
		}
		_, _, differs := firstDifference("", av, bv)
		got := !differs
		if got != tc.want {
			t.Errorf("equal(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
