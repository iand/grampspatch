package grampspatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
)

// protectedMembers lists the top-level members whose change Diff reports as an error.
var protectedMembers = map[string]bool{"handle": true, "change": true, "_class": true}

// Diff returns a test and a replace operation for each top-level member that
// differs between stored and after, and an error when before, the object as
// decoded from stored, does not encode to JSON equal to stored.
func Diff(stored json.RawMessage, before, after any) ([]Op, error) {
	keys, storedMembers, err := objectMembers(stored)
	if err != nil {
		return nil, fmt.Errorf("grampspatch: stored value: %w", err)
	}

	beforeData, err := marshal(before)
	if err != nil {
		return nil, fmt.Errorf("grampspatch: encoding object as read: %w", err)
	}
	if err := checkEqual(stored, beforeData); err != nil {
		return nil, fmt.Errorf("grampspatch: object as read does not match stored JSON %w", err)
	}

	afterData, err := marshal(after)
	if err != nil {
		return nil, fmt.Errorf("grampspatch: encoding new object: %w", err)
	}
	afterKeys, afterMembers, err := objectMembers(afterData)
	if err != nil {
		return nil, fmt.Errorf("grampspatch: new object: %w", err)
	}
	for _, k := range afterKeys {
		if _, ok := storedMembers[k]; !ok {
			return nil, fmt.Errorf("grampspatch: member %q is absent from the stored object", k)
		}
	}

	ops := []Op{}
	for _, k := range keys {
		newValue, ok := afterMembers[k]
		if !ok {
			return nil, fmt.Errorf("grampspatch: member %q is absent from the new object", k)
		}
		sv, err := decodeValue(storedMembers[k])
		if err != nil {
			return nil, err
		}
		av, err := decodeValue(newValue)
		if err != nil {
			return nil, err
		}
		if _, _, differs := firstDifference("", sv, av); !differs {
			continue
		}
		if protectedMembers[k] {
			return nil, fmt.Errorf("grampspatch: member %q cannot be changed", k)
		}
		test, err := Test(Pointer(k), storedMembers[k])
		if err != nil {
			return nil, err
		}
		replace, err := Replace(Pointer(k), newValue)
		if err != nil {
			return nil, err
		}
		ops = append(ops, test, replace)
	}
	return ops, nil
}

// checkEqual returns an error naming the first difference between two JSON
// documents.
func checkEqual(a, b []byte) error {
	av, err := decodeValue(a)
	if err != nil {
		return err
	}
	bv, err := decodeValue(b)
	if err != nil {
		return err
	}
	if path, desc, differs := firstDifference("", av, bv); differs {
		return fmt.Errorf("at %q: %s", path, desc)
	}
	return nil
}

// objectMembers returns the member names of a JSON object in document order,
// and the raw value of each member.
func objectMembers(data []byte) ([]string, map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, errors.New("not a JSON object")
	}
	var keys []string
	members := map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}
		if _, dup := members[key]; dup {
			return nil, nil, fmt.Errorf("member %q appears twice", key)
		}
		keys = append(keys, key)
		members[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, nil, errors.New("data follows the JSON object")
	}
	return keys, members, nil
}

// decodeValue decodes one JSON value with numbers kept as json.Number.
func decodeValue(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("data follows the JSON value")
	}
	return v, nil
}

// firstDifference compares two decoded JSON values as RFC 6902 section 4.6
// defines equality, and returns the JSON Pointer and a description of the
// first difference, with differs set when one exists.
func firstDifference(path string, a, b any) (string, string, bool) {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return path, fmt.Sprintf("%s against %s", describe(a), describe(b)), true
		}
		keys := make([]string, 0, len(av)+len(bv))
		for k := range av {
			keys = append(keys, k)
		}
		for k := range bv {
			if _, ok := av[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := path + Pointer(k)
			ae, aok := av[k]
			be, bok := bv[k]
			if !aok {
				return p, "member present only in the second value", true
			}
			if !bok {
				return p, "member present only in the first value", true
			}
			if dp, desc, differs := firstDifference(p, ae, be); differs {
				return dp, desc, true
			}
		}
		return "", "", false
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return path, fmt.Sprintf("%s against %s", describe(a), describe(b)), true
		}
		if len(av) != len(bv) {
			return path, fmt.Sprintf("array length %d against %d", len(av), len(bv)), true
		}
		for i := range av {
			if dp, desc, differs := firstDifference(path+"/"+strconv.Itoa(i), av[i], bv[i]); differs {
				return dp, desc, true
			}
		}
		return "", "", false
	case json.Number:
		bv, ok := b.(json.Number)
		if !ok || !equalNumbers(av, bv) {
			return path, fmt.Sprintf("%s against %s", describe(a), describe(b)), true
		}
		return "", "", false
	default:
		if a != b {
			return path, fmt.Sprintf("%s against %s", describe(a), describe(b)), true
		}
		return "", "", false
	}
}

// equalNumbers reports whether two JSON numbers have the same value.
func equalNumbers(a, b json.Number) bool {
	ar, aok := new(big.Rat).SetString(string(a))
	br, bok := new(big.Rat).SetString(string(b))
	return aok && bok && ar.Cmp(br) == 0
}

// describe returns a short form of a decoded JSON value for messages.
func describe(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return strconv.Quote(v)
	default:
		return fmt.Sprint(v)
	}
}
