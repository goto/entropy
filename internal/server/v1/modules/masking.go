package modules

import (
	"go.uber.org/zap"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/masking"
)

// maskModule returns a copy of mod with sensitive values in its configs masked,
// resolving the path list from the module's own sensitive_configs key. The
// sensitive_configs list itself is field-path metadata, not a secret, so it is
// never masked. Fail-open: on any error the original configs are kept and a
// warning logged.
func (srv *APIServer) maskModule(mod module.Module) module.Module {
	if srv.masker == nil || len(mod.Configs) == 0 {
		return mod
	}

	paths, err := masking.PathsFromConfigs(mod.Configs)
	if err != nil {
		zap.L().Warn("masking: could not parse module sensitive_configs; returning unmasked",
			zap.String("module_urn", mod.URN), zap.Error(err))
		return mod
	}
	if len(paths) == 0 {
		return mod
	}

	masked, err := srv.masker.Mask(mod.Configs, paths)
	if err != nil {
		zap.L().Warn("masking: failed to mask module configs; returning unmasked",
			zap.String("module_urn", mod.URN), zap.Error(err))
		return mod
	}
	mod.Configs = masked
	return mod
}
