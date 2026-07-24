package resources

import (
	"context"
	"encoding/json"
	"strings"

	"go.uber.org/zap"

	"github.com/goto/entropy/core/resource"
)

// maskResource returns a copy of res with sensitive spec.configs and
// state.output values replaced by their masked fingerprints. It never mutates
// the source resource (Mask returns fresh bytes). Masking is fail-open: on any
// resolution or masking error the original value is kept and a warning logged,
// so an orphaned resource (no module) is returned unmasked.
func (server APIServer) maskResource(ctx context.Context, res resource.Resource) resource.Resource {
	if server.masker == nil {
		return res
	}

	paths, err := server.configCache.PathsFor(ctx, res.Kind, res.Project)
	if err != nil {
		zap.L().Warn("masking: could not resolve sensitive_config; returning unmasked",
			zap.String("resource_urn", res.URN), zap.Error(err))
		return res
	}
	if len(paths) == 0 {
		return res
	}

	res.Spec.Configs = server.maskBytes(res.Spec.Configs, paths, res.URN)
	res.State.Output = server.maskBytes(res.State.Output, paths, res.URN)
	return res
}

// maskRevision masks a revision's spec.configs. A revision carries only its URN
// and spec, so the kind/project are recovered from the resource URN.
func (server APIServer) maskRevision(ctx context.Context, rev resource.Revision) resource.Revision {
	if server.masker == nil {
		return rev
	}

	kind, project, ok := kindProjectFromURN(rev.URN)
	if !ok {
		zap.L().Warn("masking: could not parse kind/project from revision urn; returning unmasked",
			zap.String("revision_urn", rev.URN))
		return rev
	}

	paths, err := server.configCache.PathsFor(ctx, kind, project)
	if err != nil {
		zap.L().Warn("masking: could not resolve sensitive_config; returning unmasked",
			zap.String("revision_urn", rev.URN), zap.Error(err))
		return rev
	}
	if len(paths) == 0 {
		return rev
	}

	rev.Spec.Configs = server.maskBytes(rev.Spec.Configs, paths, rev.URN)
	return rev
}

// maskBytes masks a single raw JSON payload, keeping the original on error.
func (server APIServer) maskBytes(raw json.RawMessage, paths []string, urn string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	out, err := server.masker.Mask(raw, paths)
	if err != nil {
		zap.L().Warn("masking: failed to mask payload; returning unmasked",
			zap.String("resource_urn", urn), zap.Error(err))
		return raw
	}
	return out
}

// kindProjectFromURN extracts kind and project from a resource URN of the form
// orn:entropy:{kind}:{project}:{name}.
func kindProjectFromURN(urn string) (kind, project string, ok bool) {
	parts := strings.Split(urn, ":")
	if len(parts) < 5 || parts[0] != "orn" || parts[1] != "entropy" {
		return "", "", false
	}
	return parts[2], parts[3], true
}
