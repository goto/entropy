package masking

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Restore implements the write-path merge. For each sensitive path whose
// incoming leaf is in masked form (`****-` prefix), the leaf is replaced with
// the value currently stored at the same path; a non-masked incoming leaf is
// kept as-is (new or rotated secret). The incoming fingerprint is never
// validated for staleness, so a Get -> edit -> Update round-trip can never
// clobber a real secret.
//
// If an incoming leaf is masked but stored has no value at that path,
// ErrMaskedWithoutStored is returned (e.g. a masked value on Create).
func (m *Masker) Restore(incoming, stored json.RawMessage, paths []string) (json.RawMessage, error) {
	if len(incoming) == 0 || len(paths) == 0 {
		return incoming, nil
	}

	var incomingTree any
	if err := json.Unmarshal(incoming, &incomingTree); err != nil {
		return nil, fmt.Errorf("masking: parse incoming: %w", err)
	}

	var storedTree any
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &storedTree); err != nil {
			return nil, fmt.Errorf("masking: parse stored: %w", err)
		}
	}

	for _, path := range paths {
		if err := restorePath(incomingTree, storedTree, strings.Split(path, ".")); err != nil {
			return nil, err
		}
	}

	out, err := json.Marshal(incomingTree)
	if err != nil {
		return nil, fmt.Errorf("masking: marshal incoming: %w", err)
	}
	return out, nil
}

// restorePath walks segments in lockstep across the incoming and stored trees.
func restorePath(incoming, stored any, segments []string) error {
	incObj, ok := incoming.(map[string]any)
	if !ok {
		return nil
	}
	stoObj, _ := stored.(map[string]any) // nil-safe: lookups just miss.

	seg := segments[0]
	last := len(segments) == 1

	if seg == wildcard {
		for k, v := range incObj {
			if !isMasked(v) {
				continue
			}
			if err := restoreLeaf(incObj, stoObj, k); err != nil {
				return err
			}
		}
		return nil
	}

	if last {
		if isMasked(incObj[seg]) {
			return restoreLeaf(incObj, stoObj, seg)
		}
		return nil
	}

	child, exists := incObj[seg]
	if !exists {
		return nil
	}
	var storedChild any
	if stoObj != nil {
		storedChild = stoObj[seg]
	}
	return restorePath(child, storedChild, segments[1:])
}

// restoreLeaf replaces incObj[key] with the stored value, or errors if there is
// no stored value to restore.
func restoreLeaf(incObj, stoObj map[string]any, key string) error {
	storedVal, ok := stoObj[key]
	if !ok {
		return ErrMaskedWithoutStored
	}
	incObj[key] = storedVal
	return nil
}
