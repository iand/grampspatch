// Package grampspatch writes Gramps patch files for the GrampsPatch addon.
//
// The file format is defined in FORMAT.md in the GrampsPatch addon source.
package grampspatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Schema versions for the Patch envelope.
const (
	Schema172 = "1.7.2" // Schema172 is the Gramps 6.1 XML schema version.
	Schema180 = "1.8.0" // Schema180 is the dna-core development branch XML schema version.
)

// Primary object class names used in change entries.
const (
	ClassPerson     = "Person"
	ClassFamily     = "Family"
	ClassEvent      = "Event"
	ClassPlace      = "Place"
	ClassSource     = "Source"
	ClassCitation   = "Citation"
	ClassRepository = "Repository"
	ClassMedia      = "Media"
	ClassNote       = "Note"
	ClassTag        = "Tag"
	ClassDNATest    = "DNATest"
	ClassDNAMatch   = "DNAMatch"
)

// formatVersion is the patch file format version this package writes.
const formatVersion = 1

// Patch holds the envelope and change entries of one patch file.
type Patch struct {
	Schema  string    // Schema is the Gramps XML schema version of the tree the patch is written against.
	Message string    // Message is the description of the change shown in the Gramps undo history.
	Created time.Time // Created is the time the patch was written.

	changes []change
	handles map[string]bool
}

// change is one entry of the changes array.
type change struct {
	Op           string          `json:"op"`
	Class        string          `json:"_class"`
	Handle       string          `json:"handle"`
	Object       json.RawMessage `json:"object,omitempty"`
	Patch        []Op            `json:"patch,omitempty"`
	ExpectChange *int64          `json:"expect_change,omitempty"`
}

// envelope is the top-level object of a patch file.
type envelope struct {
	GrampsPatch int      `json:"gramps_patch"`
	Schema      string   `json:"schema"`
	Message     string   `json:"message,omitempty"`
	Created     int64    `json:"created"`
	Changes     []change `json:"changes"`
}

// New returns an empty patch for the given schema version and message.
func New(schema, message string) *Patch {
	return &Patch{
		Schema:  schema,
		Message: message,
		Created: time.Now(),
		handles: map[string]bool{},
	}
}

// Len returns the number of change entries in the patch.
func (p *Patch) Len() int { return len(p.changes) }

// Add appends an entry that stores obj, whose _class and handle members must
// equal class and handle, as a new object.
func (p *Patch) Add(class, handle string, obj any) error {
	if err := p.checkEntry(class, handle); err != nil {
		return err
	}
	data, err := marshal(obj)
	if err != nil {
		return fmt.Errorf("grampspatch: encoding %s %s: %w", class, handle, err)
	}
	var head struct {
		Class  *string `json:"_class"`
		Handle *string `json:"handle"`
	}
	if data[0] != '{' {
		return fmt.Errorf("grampspatch: %s %s: object does not encode as a JSON object", class, handle)
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("grampspatch: %s %s: reading object members: %w", class, handle, err)
	}
	if head.Class == nil || *head.Class != class {
		return fmt.Errorf("grampspatch: %s %s: object _class is %s", class, handle, describePtr(head.Class))
	}
	if head.Handle == nil || *head.Handle != handle {
		return fmt.Errorf("grampspatch: %s %s: object handle is %s", class, handle, describePtr(head.Handle))
	}
	p.append(change{Op: "add", Class: class, Handle: handle, Object: data})
	return nil
}

// Update appends an entry that applies ops to the stored object of the given
// class and handle.
func (p *Patch) Update(class, handle string, ops []Op) error {
	if err := p.checkEntry(class, handle); err != nil {
		return err
	}
	if len(ops) == 0 {
		return fmt.Errorf("grampspatch: %s %s: update has no operations", class, handle)
	}
	for i, op := range ops {
		if err := checkPath(op.Path); err != nil {
			return fmt.Errorf("grampspatch: %s %s: operation %d: %w", class, handle, i, err)
		}
		if op.Op == "move" || op.Op == "copy" {
			if err := checkPath(op.From); err != nil {
				return fmt.Errorf("grampspatch: %s %s: operation %d from: %w", class, handle, i, err)
			}
		}
	}
	p.append(change{Op: "update", Class: class, Handle: handle, Patch: ops})
	return nil
}

// Delete appends an entry that removes the object of the given class and
// handle, guarded by the object's change timestamp.
func (p *Patch) Delete(class, handle string, expectChange int64) error {
	if err := p.checkEntry(class, handle); err != nil {
		return err
	}
	p.append(change{Op: "delete", Class: class, Handle: handle, ExpectChange: &expectChange})
	return nil
}

// checkEntry returns an error for an empty class or handle, or for a handle
// already used by an entry of the patch.
func (p *Patch) checkEntry(class, handle string) error {
	if class == "" {
		return errors.New("grampspatch: empty class")
	}
	if handle == "" {
		return fmt.Errorf("grampspatch: %s: empty handle", class)
	}
	if p.handles[handle] {
		return fmt.Errorf("grampspatch: handle %s appears in more than one change", handle)
	}
	return nil
}

// append adds c to the patch and records its handle.
func (p *Patch) append(c change) {
	if p.handles == nil {
		p.handles = map[string]bool{}
	}
	p.handles[c.Handle] = true
	p.changes = append(p.changes, c)
}

// Encode writes the patch as JSON indented by two spaces, with a final
// newline.
func (p *Patch) Encode(w io.Writer) error {
	env := envelope{
		GrampsPatch: formatVersion,
		Schema:      p.Schema,
		Message:     p.Message,
		Created:     p.Created.Unix(),
		Changes:     p.changes,
	}
	if env.Changes == nil {
		env.Changes = []change{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// WriteFile writes the patch to path through a temporary file in the same
// directory.
func (p *Patch) WriteFile(path string) error {
	var buf bytes.Buffer
	if err := p.Encode(&buf); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(buf.Bytes()); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// marshal encodes v as compact JSON without escaping HTML characters.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// describePtr returns a quoted string, or "missing" for a nil pointer.
func describePtr(s *string) string {
	if s == nil {
		return "missing"
	}
	return fmt.Sprintf("%q", *s)
}
