package flink

import (
	"context"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
)

func (*flinkDriver) Sync(_ context.Context, res module.ExpandedResource) (*resource.State, error) {
	return &resource.State{
		Status:     resource.StatusCompleted,
		Output:     res.Resource.State.Output,
		ModuleData: nil,
	}, nil
}
