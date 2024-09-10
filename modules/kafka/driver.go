package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
)

var defaultDriverConf = driverConf{
	Type: "source",
}

type kafkaDriver struct {
	conf driverConf
}

type Output struct {
	URL string `json:"url"`
}

type driverConf struct {
	Type         string `json:"type"`
	Entity       string `json:"entity"`
	Organization string `json:"organization"`
	Landscape    string `json:"landscape"`
	Environment  string `json:"environment"`
}

func (m *kafkaDriver) Plan(ctx context.Context, res module.ExpandedResource,
	act module.ActionRequest,
) (*resource.Resource, error) {
	cfg, err := readConfig(res.Resource, act.Params, m.conf)
	if err != nil {
		return nil, err
	}

	res.Resource.Spec = resource.Spec{
		Configs:      modules.MustJSON(cfg),
		Dependencies: nil,
	}

	res.Resource.State = resource.State{
		Status: resource.StatusCompleted,
		Output: modules.MustJSON(Output{
			URL: mapUrl(cfg),
		}),
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
	cfg, err := readConfig(res.Resource, res.Resource.Spec.Configs, m.conf)
	if err != nil {
		return nil, err
	}

	return modules.MustJSON(Output{
		URL: mapUrl(cfg),
	}), nil
}

func mapUrl(cfg *Config) string {
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

	return strings.Join(urls, ",")
}
