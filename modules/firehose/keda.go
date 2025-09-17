package firehose

import (
	"fmt"
	"maps"
	"strings"

	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/pkg/errors"
)

type Scaler string

const (
	KAFKA      Scaler = "kafka"
	PROMETHEUS Scaler = "prometheus"
)

const (
	KedaPausedAnnotationKey        = "autoscaling.keda.sh/paused"
	KedaPausedReplicaAnnotationKey = "autoscaling.keda.sh/paused-replicas"

	KedaKafkaMetadataBootstrapServersKey = "bootstrapServers"
	KedaKafkaMetadataTopicKey            = "topic"
	KedaKafkaMetadataConsumerGroupKey    = "consumerGroup"

	KafkaTopicDelimiter = "|"
)

type Keda struct {
	Paused                   bool                     `json:"paused,omitempty"`
	PausedWithReplica        bool                     `json:"paused_with_replica,omitempty"`
	PausedReplica            int                      `json:"paused_replica,omitempty"`
	MinReplicas              int                      `json:"min_replicas"`
	MaxReplicas              int                      `json:"max_replicas"`
	PollingInterval          int                      `json:"polling_interval,omitempty"`
	CooldownPeriod           int                      `json:"cooldown_period,omitempty"`
	Triggers                 map[string]Trigger       `json:"triggers,omitempty"`
	RestoreToOriginalReplica bool                     `json:"restore_to_original_replica_count,omitempty"`
	Fallback                 *Fallback                `json:"fallback,omitempty"`
	HPA                      *HorizontalPodAutoscaler `json:"hpa,omitempty"`
}

type Trigger struct {
	Type              Scaler            `json:"type,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	AuthenticationRef AuthenticationRef `json:"authentication_ref,omitempty"`
}

type AuthenticationRef struct {
	Name string `json:"name,omitempty" validate:"required"`
	Kind string `json:"kind,omitempty"`
}

type Fallback struct {
	Behavior         string `json:"behavior,omitempty"`
	Replicas         int    `json:"replicas,omitempty"`
	FailureThreshold int    `json:"failure_threshold,omitempty"`
}

type HorizontalPodAutoscaler struct {
	ScaleDown ScaleBehaviour `json:"scale_down,omitempty"`
	ScaleUp   ScaleBehaviour `json:"scale_up,omitempty"`
}

type ScaleBehaviour struct {
	Policies                   []Policy `json:"policies,omitempty"`
	StabilizationWindowSeconds *int     `json:"stabilization_window_seconds,omitempty"`
	Tolerance                  *float32 `json:"tolerance,omitempty"`
}

type Policy struct {
	Type          string  `json:"type,omitempty"`
	Value         float32 `json:"value,omitempty"`
	PeriodSeconds int     `json:"period_seconds,omitempty"`
}

func (keda *Keda) ReadConfig(cfg Config, driverCfg driverConf) error {
	kedaConfig := Keda{}

	defaultConfig := driverCfg.Autoscaler.Keda[defaultKey]
	kedaConfig = defaultConfig

	sinkType := cfg.EnvVariables[confSinkType]
	SinkConfig, ok := driverCfg.Autoscaler.Keda[sinkType]
	if ok {
		kedaConfig = SinkConfig
	}

	kedaConfig.updateTriggersMetadata(cfg.EnvVariables)

	kedaConfig.MinReplicas = keda.MinReplicas
	kedaConfig.MaxReplicas = keda.MaxReplicas

	if keda.Fallback != nil && keda.Fallback.Behavior != "" {
		kedaConfig.Fallback = keda.Fallback
	}

	if keda.HPA != nil {
		kedaConfig.HPA = keda.HPA
	}

	kedaConfig.Paused = keda.Paused
	kedaConfig.PausedWithReplica = keda.PausedWithReplica
	kedaConfig.PausedReplica = keda.PausedReplica

	*keda = kedaConfig
	return nil
}

func (keda *Keda) Pause(replica ...int) {
	if len(replica) == 0 {
		keda.Paused = true
	}
	if len(replica) > 0 {
		keda.PausedWithReplica = true
		keda.PausedReplica = replica[0]
	}
}

func (keda *Keda) Resume() {
	keda.Paused = false
	keda.PausedWithReplica = false
}

func (keda *Keda) GetHelmValues(cfg Config) (map[string]any, error) {
	annotations := make(map[string]string)
	if keda.Paused {
		annotations[KedaPausedAnnotationKey] = "true"
	}
	if keda.PausedWithReplica {
		annotations[KedaPausedReplicaAnnotationKey] = fmt.Sprint(keda.PausedReplica)
	}

	var firehoseConfigs = map[string]string{
		"namespace": cfg.Namespace,
		"replicas":  fmt.Sprint(cfg.Replicas),
	}
	var triggers []map[string]any
	for _, trigger := range keda.Triggers {
		renderedMetadata, err := renderTpl(trigger.Metadata, modules.CloneAndMergeMaps(firehoseConfigs, cfg.EnvVariables))
		if err != nil {
			return nil, err
		}
		trigger.Metadata = renderedMetadata

		topicMetadata, topicMetadataExists := trigger.Metadata[KedaKafkaMetadataTopicKey]
		if trigger.Type == KAFKA &&
			topicMetadataExists &&
			strings.Contains(topicMetadata, KafkaTopicDelimiter) {
			topics := strings.Split(topicMetadata, KafkaTopicDelimiter)
			for _, topic := range topics {
				metadata := maps.Clone(trigger.Metadata)
				metadata[KedaKafkaMetadataTopicKey] = topic
				triggers = append(triggers, map[string]any{
					"type":     trigger.Type,
					"metadata": metadata,
					"authenticationRef": map[string]any{
						"name": trigger.AuthenticationRef.Name,
						"kind": trigger.AuthenticationRef.Kind,
					},
				})
			}
			continue
		}

		triggers = append(triggers, map[string]any{
			"type":     trigger.Type,
			"metadata": trigger.Metadata,
			"authenticationRef": map[string]any{
				"name": trigger.AuthenticationRef.Name,
				"kind": trigger.AuthenticationRef.Kind,
			},
		})
	}

	var hpa map[string]any
	if keda.HPA != nil {
		var scaleUpPolicy []map[string]any
		for _, policy := range keda.HPA.ScaleUp.Policies {
			scaleUpPolicy = append(scaleUpPolicy, map[string]any{
				"type":          policy.Type,
				"value":         policy.Value,
				"periodSeconds": policy.PeriodSeconds,
			})
		}

		var scaleDownPolicy []map[string]any
		for _, policy := range keda.HPA.ScaleDown.Policies {
			scaleDownPolicy = append(scaleDownPolicy, map[string]any{
				"type":          policy.Type,
				"value":         policy.Value,
				"periodSeconds": policy.PeriodSeconds,
			})
		}

		hpa = map[string]any{
			"scaleUp": map[string]any{
				"policies":                   scaleUpPolicy,
				"stabilizationWindowSeconds": keda.HPA.ScaleUp.StabilizationWindowSeconds,
				"tolerance":                  keda.HPA.ScaleUp.Tolerance,
			},
			"scaleDown": map[string]any{
				"policies":                   scaleDownPolicy,
				"stabilizationWindowSeconds": keda.HPA.ScaleDown.StabilizationWindowSeconds,
				"tolerance":                  keda.HPA.ScaleDown.Tolerance,
			},
		}
	}

	var fallback map[string]any
	if keda.Fallback != nil {
		fallback = map[string]any{
			"behavior":         keda.Fallback.Behavior,
			"failureThreshold": keda.Fallback.FailureThreshold,
			"replicas":         keda.Fallback.Replicas,
		}
	}

	return map[string]any{
		"annotations":                   annotations,
		"maxReplicaCount":               keda.MaxReplicas,
		"minReplicaCount":               keda.MinReplicas,
		"pollingInterval":               keda.PollingInterval,
		"cooldownPeriod":                keda.CooldownPeriod,
		"restoreToOriginalReplicaCount": keda.RestoreToOriginalReplica,
		"fallback":                      fallback,
		"triggers":                      triggers,
		"hpa":                           hpa,
	}, nil
}

func (keda *Keda) updateTriggersMetadata(cfg map[string]string) error {
	for key, trigger := range keda.Triggers {
		switch trigger.Type {
		case KAFKA:
			if _, ok := cfg[confKeyConsumerID]; ok {
				trigger.Metadata[KedaKafkaMetadataConsumerGroupKey] = cfg[confKeyConsumerID]
			}
			if _, ok := cfg[confKeyKafkaTopic]; ok {
				trigger.Metadata[KedaKafkaMetadataTopicKey] = cfg[confKeyKafkaTopic]
			}
			if _, ok := cfg[confKeyKafkaBrokers]; ok {
				trigger.Metadata[KedaKafkaMetadataBootstrapServersKey] = cfg[confKeyKafkaBrokers]
			}
		}
		keda.Triggers[key] = trigger
	}
	return nil
}

func (keda *Keda) Validate() error {
	if keda.MinReplicas == 0 && keda.MaxReplicas == 0 {
		return errors.ErrInvalid.WithMsgf("min_replicas and max_replicas must be set when autoscaler is enabled")
	}

	if keda.MinReplicas < 0 {
		return errors.ErrInvalid.WithMsgf("min_replicas must be greater than or equal to 0")
	}

	if keda.MaxReplicas < 1 {
		return errors.ErrInvalid.WithMsgf("max_replicas must be greater than or equal to 1")
	}

	if keda.MinReplicas > keda.MaxReplicas {
		return errors.ErrInvalid.WithMsgf("min_replicas must be less than or equal to max_replicas")
	}

	if len(keda.Triggers) == 0 {
		return errors.ErrInvalid.WithMsgf("at least one trigger must be defined when autoscaler is enabled")
	}
	return nil
}
