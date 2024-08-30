package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

type kafkaDriver struct{}

type Output struct {
	URL string `json:"url"`
}

func (m *kafkaDriver) Plan(ctx context.Context, res module.ExpandedResource,
	act module.ActionRequest,
) (*resource.Resource, error) {
	res.Resource.Spec = resource.Spec{
		Configs:      act.Params,
		Dependencies: nil,
	}

	output, err := m.Output(ctx, res)
	if err != nil {
		return nil, err
	}

	res.Resource.State = resource.State{
		Status: resource.StatusCompleted,
		Output: output,
	}

	return &res.Resource, nil
}

func (*kafkaDriver) Sync(_ context.Context, res module.ExpandedResource) (*resource.State, error) {
	return &resource.State{
		Status:     resource.StatusCompleted,
		Output:     res.Resource.State.Output,
		ModuleData: nil,
	}, nil
}

func (m *kafkaDriver) Output(ctx context.Context, res module.ExpandedResource) (json.RawMessage, error) {
	cfg, err := readConfig(res.Resource.Spec.Configs)
	if err != nil {
		return nil, err
	}

	var mode, port string
	if cfg.AdvertiseMode.Address != "" {
		mode = "address"
		port = cfg.AdvertiseMode.Address
	} else {
		mode = "host"
		port = cfg.AdvertiseMode.Host
	}

	var urls []string
	for _, broker := range cfg.Brokers {
		var addr string
		if mode == "address" {
			addr = broker.Address
		} else {
			addr = broker.Host
		}
		urls = append(urls, fmt.Sprintf("%s:%s", addr, port))
	}

	output, err := json.Marshal(Output{URL: strings.Join(urls, ",")})
	if err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	return output, nil
}
