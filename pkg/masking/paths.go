package masking

import (
	"fmt"
	"strings"
)

// ValidatePaths checks that each sensitive_configs entry is a well-formed
// dot-notation path: non-empty, no empty segments, and at most one `*` which
// may only appear as the trailing segment. It does not require paths to resolve
// against any current config.
func ValidatePaths(paths []string) error {
	for _, path := range paths {
		if path == "" {
			return fmt.Errorf("masking: sensitive_configs path must not be empty")
		}

		segments := strings.Split(path, ".")
		for i, seg := range segments {
			if seg == "" {
				return fmt.Errorf("masking: sensitive_configs path %q has an empty segment", path)
			}
			if seg == wildcard && i != len(segments)-1 {
				return fmt.Errorf("masking: sensitive_configs path %q may only use %q as the last segment", path, wildcard)
			}
			if seg != wildcard && strings.Contains(seg, wildcard) {
				return fmt.Errorf("masking: sensitive_configs path %q uses %q outside a standalone segment", path, wildcard)
			}
		}
	}
	return nil
}
