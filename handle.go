package grampspatch

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
)

// handlePrefix matches a valid prefix for Handle.
var handlePrefix = regexp.MustCompile(`^[A-Z0-9]{1,8}$`)

// handleHashLength is the number of base-32 characters Handle takes from the
// hash.
const handleHashLength = 16

// Handle returns prefix followed by 16 base-32 characters of a SHA-256 hash of
// the tokens.
func Handle(prefix string, tokens ...string) (string, error) {
	if !handlePrefix.MatchString(prefix) {
		return "", fmt.Errorf("grampspatch: handle prefix %q is not one to eight upper-case letters or digits", prefix)
	}
	h := sha256.New()
	for _, t := range tokens {
		h.Write([]byte(t))
		h.Write([]byte{0})
	}
	sum := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(h.Sum(nil))
	return prefix + sum[:handleHashLength], nil
}
