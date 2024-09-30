package flink

import (
	"context"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
)

func (fd *flinkDriver) Plan(ctx context.Context, res module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	res.Resource.Spec = resource.Spec{
		Configs:      act.Params,
		Dependencies: res.Spec.Dependencies,
	}

	output, err := fd.Output(ctx, res)
	if err != nil {
		return nil, err
	}

	res.Resource.State = resource.State{
		Status: resource.StatusCompleted,
		Output: output,
	}

	return &res.Resource, nil
}
