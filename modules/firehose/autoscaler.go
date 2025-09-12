package firehose

import (
	"encoding/json"

	"github.com/goto/entropy/pkg/errors"
)

type AutoscalerType string

const (
	KEDA AutoscalerType = "keda"
)

type AutoscalerSpec interface {
	Validate() error
	ReadConfig(cfg Config, driverConf driverConf) error
	Pause(replica ...int)
	Resume()
	GetHelmValues(cfg Config) (map[string]any, error)
}

type Autoscaler struct {
	Enabled bool           `json:"enabled"`
	Type    AutoscalerType `json:"type,omitempty"`
	Spec    AutoscalerSpec `json:"spec,omitempty"`
}

func (autoscaler *Autoscaler) GetHelmValues(cfg Config) (map[string]any, error) {
	values := map[string]any{
		"enabled": autoscaler.Enabled,
		"type":    autoscaler.Type,
	}

	typeValues, err := autoscaler.Spec.GetHelmValues(cfg)
	if err != nil {
		return nil, err
	}
	values[string(autoscaler.Type)] = typeValues

	return values, nil
}

func (autoscaler *Autoscaler) UnmarshalJSON(data []byte) error {
	type BaseAutoscaler Autoscaler
	autoscalerTemp := &struct {
		Spec json.RawMessage `json:"spec"`
		*BaseAutoscaler
	}{
		BaseAutoscaler: (*BaseAutoscaler)(autoscaler),
	}

	if err := json.Unmarshal(data, &autoscalerTemp); err != nil {
		return errors.ErrInvalid.WithMsgf("invalid autoscaler config").WithCausef(err.Error())
	}

	switch autoscalerTemp.Type {
	case KEDA:
		var kedaSpec *Keda
		if err := json.Unmarshal(autoscalerTemp.Spec, &kedaSpec); err != nil {
			return errors.ErrInvalid.WithMsgf("invalid keda config").WithCausef(err.Error())
		}
		autoscaler.Spec = kedaSpec
	default:
		return errors.ErrInvalid.WithMsgf("unsupported autoscaler type: %s", autoscaler.Type)
	}
	return nil
}

func (autoscaler *Autoscaler) Validate() error {
	if !autoscaler.Enabled {
		return nil
	}
	if autoscaler.Spec == nil {
		return errors.ErrInvalid.WithMsgf("autoscaler spec must be provided when autoscaler is enabled")
	}
	if err := autoscaler.Spec.Validate(); err != nil {
		return err
	}
	return nil
}
