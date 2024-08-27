package dagger

import (
	"context"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
)

func (dd *daggerDriver) Sync(ctx context.Context, exr module.ExpandedResource) (*resource.State, error) {
	return nil, nil
}
