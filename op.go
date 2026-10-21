package grampspatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Op is one JSON Patch (RFC 6902) operation.
type Op struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	From  string          `json:"from,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// Test returns an operation that fails the update unless the value at path
// equals value.
func Test(path string, value any) (Op, error) {
	return valueOp("test", path, value)
}

// Replace returns an operation that replaces the existing value at path.
func Replace(path string, value any) (Op, error) {
	return valueOp("replace", path, value)
}

// AddValue returns an operation that adds a member at path or inserts value
// into an array.
func AddValue(path string, value any) (Op, error) {
	return valueOp("add", path, value)
}

// Remove returns an operation that removes the member or array element at
// path.
func Remove(path string) Op {
	return Op{Op: "remove", Path: path}
}

// valueOp returns an operation of the given kind carrying the encoded value.
func valueOp(kind, path string, value any) (Op, error) {
	data, err := marshal(value)
	if err != nil {
		return Op{}, fmt.Errorf("grampspatch: encoding %s value for %s: %w", kind, path, err)
	}
	return Op{Op: kind, Path: path, Value: data}, nil
}

// pointerEscaper escapes a JSON Pointer reference token.
var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

// Pointer joins segments into a JSON Pointer (RFC 6901).
func Pointer(segments ...string) string {
	var b strings.Builder
	for _, s := range segments {
		b.WriteByte('/')
		b.WriteString(pointerEscaper.Replace(s))
	}
	return b.String()
}

// checkPath returns an error when path is not a JSON Pointer that a patch may
// address: the root, /handle, /change and any _class member are forbidden.
func checkPath(path string) error {
	if path == "" {
		return errors.New("path addresses the object root")
	}
	if path[0] != '/' {
		return fmt.Errorf("path %q is not a JSON Pointer", path)
	}
	for i, token := range strings.Split(path[1:], "/") {
		for j := 0; j < len(token); j++ {
			if token[j] == '~' && (j+1 == len(token) || (token[j+1] != '0' && token[j+1] != '1')) {
				return fmt.Errorf("path %q has an invalid escape", path)
			}
		}
		name := strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
		if name == "_class" {
			return fmt.Errorf("path %q addresses a _class member", path)
		}
		if i == 0 && (name == "handle" || name == "change") {
			return fmt.Errorf("path %q addresses /%s", path, name)
		}
	}
	return nil
}
