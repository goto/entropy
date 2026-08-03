package masking

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Restore implements the write-path merge. For each sensitive path whose
// incoming leaf is in masked form (`****-` prefix): if a value is currently
// stored at the same path, the leaf is replaced with it; if nothing is stored
// there (e.g. a masked value on Create, or a new field), the leaf is dropped.
// A non-masked incoming leaf is kept as-is (new or rotated secret). The
// incoming fingerprint is never validated for staleness, so a Get -> edit ->
// Update round-trip can never clobber a real secret.
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
		restorePath(incomingTree, storedTree, strings.Split(path, "."))
	}

	out, err := json.Marshal(incomingTree)
	if err != nil {
		return nil, fmt.Errorf("masking: marshal incoming: %w", err)
	}
	return out, nil
}

// restorePath walks segments in lockstep across the incoming and stored trees.
func restorePath(incoming, stored any, segments []string) {
	incObj, ok := incoming.(map[string]any)
	if !ok {
		return
	}
	stoObj, _ := stored.(map[string]any) // nil-safe: lookups just miss.

	seg := segments[0]
	last := len(segments) == 1

	if seg == wildcard {
		for k, v := range incObj {
			if isMasked(v) {
				restoreLeaf(incObj, stoObj, k)
			}
		}
		return
	}

	if last {
		if isMasked(incObj[seg]) {
			restoreLeaf(incObj, stoObj, seg)
		}
		return
	}

	child, exists := incObj[seg]
	if !exists {
		return
	}
	var storedChild any
	if stoObj != nil {
		storedChild = stoObj[seg]
	}
	restorePath(child, storedChild, segments[1:])
}

// restoreLeaf replaces incObj[key] with the stored value, or drops the key if
// there is no stored value to restore.
func restoreLeaf(incObj, stoObj map[string]any, key string) {
	storedVal, ok := stoObj[key]
	if !ok {
		delete(incObj, key)
		return
	}
	incObj[key] = storedVal
}
