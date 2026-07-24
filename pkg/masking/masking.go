// Package masking replaces configured sensitive values in JSON payloads with a
// deterministic, keyed fingerprint (`****-<8-hex-hmac>`) at the API response
// boundary, and restores stored secrets when a client resends a masked value.
//
// Masking is boundary-only: it operates on json.RawMessage payloads while a
// response is being built and never mutates the domain objects used for
// reconciliation, so drivers keep receiving real secrets during Plan/Sync.
package masking

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// maskPrefix is the literal prefix of a masked value. A value that starts with
// this prefix is treated as masked on the write path.
const maskPrefix = "****-"

// fingerprintLen is the number of hex characters kept from the HMAC digest.
const fingerprintLen = 8

// wildcard matches every direct child key of the node named by the preceding
// path segment.
const wildcard = "*"

// ErrMaskedWithoutStored is returned by Restore when an incoming value is in
// masked form but there is no stored value to restore for that path. Callers
// (e.g. resource Create) surface this as invalid input.
var ErrMaskedWithoutStored = errors.New("masking: incoming value is masked but no stored value exists")

// Masker holds the HMAC key used to fingerprint sensitive values.
type Masker struct {
	key []byte
}

// New builds a Masker with the given HMAC key.
func New(key []byte) *Masker {
	return &Masker{key: key}
}

// Mask parses payload into a generic tree, replaces the leaf at each sensitive
// path with `****-<fingerprint>`, and re-marshals. Paths that do not resolve in
// this payload are skipped silently. A trailing `*` segment masks every direct
// child of the named node.
func (m *Masker) Mask(payload json.RawMessage, paths []string) (json.RawMessage, error) {
	if len(payload) == 0 || len(paths) == 0 {
		return payload, nil
	}

	var tree any
	if err := json.Unmarshal(payload, &tree); err != nil {
		return nil, fmt.Errorf("masking: parse payload: %w", err)
	}

	for _, path := range paths {
		m.applyMask(tree, strings.Split(path, "."))
	}

	out, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("masking: marshal payload: %w", err)
	}
	return out, nil
}

// applyMask walks segments from node, masking the resolved leaf/leaves.
func (m *Masker) applyMask(node any, segments []string) {
	obj, ok := node.(map[string]any)
	if !ok {
		return
	}

	seg := segments[0]
	last := len(segments) == 1

	if seg == wildcard {
		// Only valid as a trailing segment (guaranteed by ValidatePaths).
		for k, v := range obj {
			obj[k] = m.masked(v)
		}
		return
	}

	child, exists := obj[seg]
	if !exists {
		return
	}

	if last {
		obj[seg] = m.masked(child)
		return
	}
	m.applyMask(child, segments[1:])
}

// masked returns the masked-form string for a value.
func (m *Masker) masked(value any) string {
	return maskPrefix + m.fingerprint(value)
}

// fingerprint canonicalises value via JSON, computes a keyed HMAC-SHA256, and
// returns the first fingerprintLen hex characters. Deterministic per value and
// key; brute-force resistant because the key is server-only.
func (m *Masker) fingerprint(value any) string {
	canonical, err := json.Marshal(value)
	if err != nil {
		// json.Marshal only fails on unsupported types, which cannot appear in
		// a tree parsed from JSON. Fall back to the Go representation.
		canonical = []byte(fmt.Sprintf("%v", value))
	}
	mac := hmac.New(sha256.New, m.key)
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil))[:fingerprintLen]
}

// isMasked reports whether v is a string in masked form.
func isMasked(v any) bool {
	s, ok := v.(string)
	return ok && strings.HasPrefix(s, maskPrefix)
}
