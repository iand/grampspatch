package grampspatch

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestFixture rebuilds the GrampsPatch DNA fixture, copied from
// GrampsPatch/tests/fixtures/data-dna.gpatch.json, and compares the result
// with it.
func TestFixture(t *testing.T) {
	want, err := os.ReadFile("testdata/data-dna.gpatch.json")
	if err != nil {
		t.Fatalf("reading fixture: unexpected error: %v", err)
	}
	var fixture struct {
		Changes []struct {
			Object json.RawMessage `json:"object"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(want, &fixture); err != nil {
		t.Fatalf("decoding fixture: unexpected error: %v", err)
	}

	p := New(Schema180, "GrampsPatch DNA fixture")
	p.Created = time.Unix(1790000000, 0)
	if err := p.Add(ClassDNATest, "GPFIXDNATEST0000001", fixture.Changes[0].Object); err != nil {
		t.Fatalf("Add DNATest: unexpected error: %v", err)
	}
	if err := p.Add(ClassDNAMatch, "GPFIXDNAMATCH000001", fixture.Changes[1].Object); err != nil {
		t.Fatalf("Add DNAMatch: unexpected error: %v", err)
	}
	ops := []Op{
		mustOp(t)(Test("/shared_cm", 2547.3)),
		mustOp(t)(Replace("/shared_cm", json.RawMessage("2551.0"))),
		mustOp(t)(Test("/segment_count", 22)),
		mustOp(t)(Replace("/segment_count", 23)),
	}
	if err := p.Update(ClassDNAMatch, "dna1m0001ksiberic011", ops); err != nil {
		t.Fatalf("Update: unexpected error: %v", err)
	}

	var got bytes.Buffer
	if err := p.Encode(&got); err != nil {
		t.Fatalf("Encode: unexpected error: %v", err)
	}
	gv, err := decodeValue(got.Bytes())
	if err != nil {
		t.Fatalf("decoding output: unexpected error: %v", err)
	}
	wv, err := decodeValue(want)
	if err != nil {
		t.Fatalf("decoding fixture: unexpected error: %v", err)
	}
	if path, diff, differs := firstDifference("", wv, gv); differs {
		t.Errorf("output differs from fixture at %q: %s", path, diff)
	}
}
