package grampspatch

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testObject returns a minimal object dict with the given class and handle.
func testObject(class, handle string) map[string]any {
	return map[string]any{"_class": class, "handle": handle, "change": 0}
}

// mustOp returns a function that returns op, failing the test when err is
// not nil.
func mustOp(t *testing.T) func(op Op, err error) Op {
	return func(op Op, err error) Op {
		t.Helper()
		if err != nil {
			t.Fatalf("building op: unexpected error: %v", err)
		}
		return op
	}
}

func TestEncodeEnvelope(t *testing.T) {
	p := New(DBVersionDNA, "Add things")
	p.Created = time.Unix(1790000000, 0)
	if err := p.Add(ClassNote, "N1", testObject(ClassNote, "N1")); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if err := p.Encode(&buf); err != nil {
		t.Fatalf("Encode: unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "}\n") {
		t.Errorf("output does not end with a newline after the object")
	}
	if !strings.Contains(out, "\n  \"changes\": [") {
		t.Errorf("output is not indented by two spaces:\n%s", out)
	}

	var env struct {
		GrampsPatch *int              `json:"gramps_patch"`
		DBVersion   *int              `json:"db_version"`
		Schema      *string           `json:"schema"`
		Message     string            `json:"message"`
		Created     *int64            `json:"created"`
		Changes     []json.RawMessage `json:"changes"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("decoding output: unexpected error: %v", err)
	}
	if env.GrampsPatch == nil || *env.GrampsPatch != 1 {
		t.Errorf("gramps_patch = %v, want 1", env.GrampsPatch)
	}
	if env.DBVersion == nil || *env.DBVersion != 23 {
		t.Errorf("db_version = %v, want 23", env.DBVersion)
	}
	if env.Schema != nil {
		t.Errorf("schema = %q, want no schema member", *env.Schema)
	}
	if env.Message != "Add things" {
		t.Errorf("message = %q, want %q", env.Message, "Add things")
	}
	if env.Created == nil || *env.Created != 1790000000 {
		t.Errorf("created = %v, want 1790000000", env.Created)
	}
	if len(env.Changes) != 1 {
		t.Errorf("changes = %d, want 1", len(env.Changes))
	}
}

func TestEncodeDeterministic(t *testing.T) {
	build := func() []byte {
		p := New(DBVersionDNA, "m")
		p.Created = time.Unix(1790000000, 0)
		obj := testObject(ClassNote, "N1")
		obj["text"] = "a < b & c"
		if err := p.Add(ClassNote, "N1", obj); err != nil {
			t.Fatalf("Add: unexpected error: %v", err)
		}
		var buf bytes.Buffer
		if err := p.Encode(&buf); err != nil {
			t.Fatalf("Encode: unexpected error: %v", err)
		}
		return buf.Bytes()
	}
	a, b := build(), build()
	if !bytes.Equal(a, b) {
		t.Errorf("two encodings of the same patch differ")
	}
	if !bytes.Contains(a, []byte("a < b & c")) {
		t.Errorf("output escapes HTML characters:\n%s", a)
	}
}

func TestChangeEntries(t *testing.T) {
	op := mustOp(t)(Replace("/page", "x"))

	p := New(DBVersionDNA, "m")
	if err := p.Add(ClassDNATest, "T1", testObject(ClassDNATest, "T1")); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	if err := p.Update(ClassCitation, "C1", []Op{op}); err != nil {
		t.Fatalf("Update: unexpected error: %v", err)
	}
	if err := p.Delete(ClassNote, "N1", 1790000000); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	if p.Len() != 3 {
		t.Errorf("Len = %d, want 3", p.Len())
	}

	var buf bytes.Buffer
	if err := p.Encode(&buf); err != nil {
		t.Fatalf("Encode: unexpected error: %v", err)
	}
	var env struct {
		Changes []map[string]json.RawMessage `json:"changes"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("decoding output: unexpected error: %v", err)
	}
	want := []struct {
		op      string
		members []string
	}{
		{"add", []string{"op", "_class", "handle", "object"}},
		{"update", []string{"op", "_class", "handle", "patch"}},
		{"delete", []string{"op", "_class", "handle", "expect_change"}},
	}
	for i, w := range want {
		c := env.Changes[i]
		if string(c["op"]) != `"`+w.op+`"` {
			t.Errorf("change %d op = %s, want %q", i, c["op"], w.op)
		}
		if len(c) != len(w.members) {
			t.Errorf("change %d has %d members, want %v", i, len(c), w.members)
		}
		for _, m := range w.members {
			if _, ok := c[m]; !ok {
				t.Errorf("change %d lacks member %q", i, m)
			}
		}
	}
	if string(env.Changes[2]["expect_change"]) != "1790000000" {
		t.Errorf("expect_change = %s, want 1790000000", env.Changes[2]["expect_change"])
	}
}

func TestChangeErrors(t *testing.T) {
	op := mustOp(t)(Replace("/page", "x"))
	tests := []struct {
		name string
		call func(p *Patch) error
	}{
		{"add empty class", func(p *Patch) error { return p.Add("", "H1", testObject("", "H1")) }},
		{"add empty handle", func(p *Patch) error { return p.Add(ClassNote, "", testObject(ClassNote, "")) }},
		{"add class mismatch", func(p *Patch) error { return p.Add(ClassNote, "H1", testObject(ClassSource, "H1")) }},
		{"add handle mismatch", func(p *Patch) error { return p.Add(ClassNote, "H1", testObject(ClassNote, "H2")) }},
		{"add not an object", func(p *Patch) error { return p.Add(ClassNote, "H1", []int{1}) }},
		{"add duplicate handle", func(p *Patch) error { return p.Add(ClassNote, "DUP", testObject(ClassNote, "DUP")) }},
		{"update duplicate handle", func(p *Patch) error { return p.Update(ClassSource, "DUP", []Op{op}) }},
		{"delete duplicate handle", func(p *Patch) error { return p.Delete(ClassNote, "DUP", 1) }},
		{"update no ops", func(p *Patch) error { return p.Update(ClassNote, "H1", nil) }},
		{"update root path", func(p *Patch) error { return p.Update(ClassNote, "H1", []Op{Remove("")}) }},
		{"update handle path", func(p *Patch) error { return p.Update(ClassNote, "H1", []Op{Remove("/handle")}) }},
		{"update change path", func(p *Patch) error { return p.Update(ClassNote, "H1", []Op{Remove("/change")}) }},
		{"update nested class path", func(p *Patch) error { return p.Update(ClassNote, "H1", []Op{Remove("/type/_class")}) }},
		{"update bad pointer", func(p *Patch) error { return p.Update(ClassNote, "H1", []Op{Remove("type")}) }},
		{"update bad escape", func(p *Patch) error { return p.Update(ClassNote, "H1", []Op{Remove("/a~2b")}) }},
		{"delete empty handle", func(p *Patch) error { return p.Delete(ClassNote, "", 1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New(DBVersionDNA, "m")
			if err := p.Add(ClassNote, "DUP", testObject(ClassNote, "DUP")); err != nil {
				t.Fatalf("Add: unexpected error: %v", err)
			}
			if err := tc.call(p); err == nil {
				t.Errorf("expected error, got nil")
			}
			if p.Len() != 1 {
				t.Errorf("Len = %d after a failed call, want 1", p.Len())
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	p := New(DBVersionDNA, "m")
	if err := p.Add(ClassNote, "N1", testObject(ClassNote, "N1")); err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "test-20260926.gpatch.json")
	if err := p.WriteFile(path); err != nil {
		t.Fatalf("WriteFile: unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output: unexpected error: %v", err)
	}
	var want bytes.Buffer
	if err := p.Encode(&want); err != nil {
		t.Fatalf("Encode: unexpected error: %v", err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("file content differs from Encode output")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want 1", len(entries))
	}
}
