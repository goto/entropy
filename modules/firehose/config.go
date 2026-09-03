package firehose

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	kafkamod "github.com/goto/entropy/modules/kafka"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

const (
	confSinkType        = "SINK_TYPE"
	confKeyConsumerID   = "SOURCE_KAFKA_CONSUMER_GROUP_ID"
	confKeyKafkaBrokers = "SOURCE_KAFKA_BROKERS"
	confKeyKafkaTopic   = "SOURCE_KAFKA_TOPIC"

	confDLQSinkEnable          = "DLQ_SINK_ENABLE"
	confDLQWriterType          = "DLQ_WRITER_TYPE"
	confDLQKafkaTopic          = "DLQ_KAFKA_TOPIC"
	confDLQKafkaBrokers        = "DLQ_KAFKA_BROKERS"
	confDLQKafkaTopicRetention = "DLQ_KAFKA_TOPIC_RETENTION"
	dlqWriterTypeKafka         = "KAFKA"
	dlqKafkaStreamName         = "dagstream"
	kafkaTopicNameMaxLength    = 249
	minDLQTopicRetentionSec    = 86400
	maxDLQTopicRetentionSec    = 604800
)

// Kafka sink env variable keys
const (
	sinkTypeKafka             = "KAFKA"
	confSinkKafkaBrokers      = "SINK_KAFKA_BROKERS"
	confSinkKafkaStream       = "SINK_KAFKA_STREAM"
	confSinkKafkaTopic        = "SINK_KAFKA_TOPIC"
	confSinkKafkaProtoMessage = "SINK_KAFKA_PROTO_MESSAGE"
	confSinkKafkaProtoMapping = "SINK_KAFKA_PROTO_MAPPING"
)

const helmReleaseNameMaxLength = 53

var (
	//go:embed schema/config.json
	configSchemaRaw []byte

	validateConfig = validator.FromJSONSchema(configSchemaRaw)
)

type ScaleParams struct {
	Replicas int `json:"replicas"`
}

type StartParams struct {
	StopTime *time.Time `json:"stop_time"`
}

type Config struct {
	// Stopped flag when set forces the firehose to be stopped on next sync.
	Stopped bool `json:"stopped"`

	// StopTime can be set to schedule the firehose to be stopped at given time.
	StopTime *time.Time `json:"stop_time,omitempty"`

	// Replicas is the number of firehose instances to run.
	Replicas int `json:"replicas"`

	// Namespace is the target namespace where firehose should be deployed.
	// Inherits from driver config.
	Namespace string `json:"namespace,omitempty"`

	// DeploymentID will be used as the release-name for the deployment.
	// Must be shorter than 53 chars if set. If not set, one will be generated
	// automatically.
	DeploymentID string `json:"deployment_id,omitempty"`

	// EnvVariables contains all the firehose environment config values.
	EnvVariables map[string]string `json:"env_variables,omitempty"`

	// ResetOffset represents the value to which kafka consumer offset was set to
	ResetOffset string `json:"reset_offset,omitempty"`

	Limits        UsageSpec     `json:"limits,omitempty"`
	Requests      UsageSpec     `json:"requests,omitempty"`
	Telegraf      *Telegraf     `json:"telegraf,omitempty"`
	ChartValues   *ChartValues  `json:"chart_values,omitempty"`
	InitContainer InitContainer `json:"init_container,omitempty"`
	Autoscaler    *Autoscaler   `json:"autoscaler,omitempty"`

	// Team owns the firehose. It selects the credential reference from a
	// stream's PLAIN/SCRAM ACL list.
	Team string `json:"team,omitempty"`

	// StreamName is the name of the kafka resource backing SOURCE_KAFKA_BROKERS.
	// It is the key used to resolve the stream's security profile. Setting it
	// also makes this module the owner of the SASL/SSL env variables, which are
	// rebuilt from the stream on every plan.
	StreamName string `json:"stream_name,omitempty"`

	// StreamSecurityEnabled says the named stream carries a security profile, so
	// the driver resolves its kafka resource internally (by URN, without a
	// declared dependency). The caller already knows this — Dex reads the same
	// resource for the broker address — so a plaintext firehose costs no lookup.
	StreamSecurityEnabled bool `json:"stream_security_enabled,omitempty"`

	// StreamSecurity holds the kafka security profile fetched by Dex, keyed by
	// stream name. References only — never inline secret values.
	StreamSecurity map[string]*kafkamod.SecurityProfile `json:"stream_security,omitempty"`

	// ACL describes the secret material an ACL (SASL_SSL/OAUTHBEARER, SSL,
	// PLAIN/SCRAM) source stream needs mounted. It is computed by
	// applyStreamSecurity from the resolved stream security profile and holds
	// references only — never secret values.
	ACL *ACLConfig `json:"acl,omitempty"`

	// ServiceAccount, when set, becomes the pod's service account. It is the
	// OAuth identity authorized for ACL streams. Empty preserves the chart's
	// default service account.
	ServiceAccount string `json:"service_account,omitempty"`
}

type Telegraf struct {
	Enabled  bool           `json:"enabled,omitempty"`
	Image    map[string]any `json:"image,omitempty"`
	Config   TelegrafConf   `json:"config,omitempty"`
	Limits   UsageSpec      `json:"limits,omitempty"`
	Requests UsageSpec      `json:"requests,omitempty"`
}

type TelegrafConf struct {
	Output               map[string]any    `json:"output"`
	AdditionalGlobalTags map[string]string `json:"additional_global_tags"`
}

type ChartValues struct {
	ImageRepository string `json:"image_repository" validate:"required"`
	ImageTag        string `json:"image_tag" validate:"required"`
	ChartVersion    string `json:"chart_version" validate:"required"`
	ImagePullPolicy string `json:"image_pull_policy" validate:"required"`
}

func readConfig(r resource.Resource, confJSON json.RawMessage, dc driverConf) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef("%s", err.Error())
	}

	cfg.EnvVariables = modules.CloneAndMergeMaps(dc.EnvVariables, cfg.EnvVariables)
	cfg.InitContainer = dc.InitContainer

	if cfg.Replicas <= 0 {
		cfg.Replicas = 1
	}

	if err := validateConfig(confJSON); err != nil {
		return nil, err
	}

	// note: enforce the kubernetes deployment name length limit.
	if len(cfg.DeploymentID) == 0 {
		cfg.DeploymentID = modules.SafeName(fmt.Sprintf("%s-%s", r.Project, r.Name), "-firehose", helmReleaseNameMaxLength)
	} else if len(cfg.DeploymentID) > helmReleaseNameMaxLength {
		return nil, errors.ErrInvalid.WithMsgf("deployment_id must not have more than 53 chars")
	}

	// we name a consumer group by adding a sequence suffix to the deployment name
	// this sequence will later be incremented to name new consumer group while resetting offset
	if consumerID := cfg.EnvVariables[confKeyConsumerID]; consumerID == "" {
		cfg.EnvVariables[confKeyConsumerID] = fmt.Sprintf("%s-1", cfg.DeploymentID)
	}

	rl := dc.RequestsAndLimits[defaultKey]
	if overrides, ok := dc.RequestsAndLimits[cfg.EnvVariables[confSinkType]]; ok {
		rl.Limits = rl.Limits.merge(overrides.Limits)
		rl.Requests = rl.Requests.merge(overrides.Requests)
	}
	cfg.Limits = rl.Limits.merge(cfg.Limits)
	cfg.Requests = rl.Requests.merge(cfg.Requests)

	if cfg.Namespace == "" {
		ns := dc.Namespace[defaultKey]
		if override, ok := dc.Namespace[cfg.EnvVariables[confSinkType]]; ok {
			ns = override
		}
		cfg.Namespace = ns
	}

	if cfg.EnvVariables[confSinkType] == sinkTypeKafka {
		if err := validateKafkaSinkEnvVars(cfg.EnvVariables); err != nil {
			return nil, err
		}
	}

	if err := validateKafkaDLQEnvVars(cfg.EnvVariables); err != nil {
		return nil, err
	}

	if cfg.Autoscaler != nil && cfg.Autoscaler.Enabled {
		if err := cfg.Autoscaler.Spec.ReadConfig(cfg, dc); err != nil {
			return nil, err
		}

		if err := cfg.Autoscaler.Validate(); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

func validateKafkaSinkEnvVars(envVars map[string]string) error {
	required := []string{
		confSinkKafkaBrokers,
		confSinkKafkaTopic,
		confSinkKafkaProtoMessage,
		confSinkKafkaProtoMapping,
	}
	for _, key := range required {
		if envVars[key] == "" {
			return errors.ErrInvalid.WithMsgf("env variable '%s' is required when SINK_TYPE=KAFKA", key)
		}
	}
	return nil
}

var kafkaTopicNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateKafkaDLQEnvVars(envVars map[string]string) error {
	enabled, _ := strconv.ParseBool(envVars[confDLQSinkEnable])
	if !enabled || !strings.EqualFold(strings.TrimSpace(envVars[confDLQWriterType]), dlqWriterTypeKafka) {
		return nil
	}

	topic := strings.TrimSpace(envVars[confDLQKafkaTopic])
	if topic == "" {
		return errors.ErrInvalid.WithMsgf("env variable '%s' is required when %s=true and %s=%s", confDLQKafkaTopic, confDLQSinkEnable, confDLQWriterType, dlqWriterTypeKafka)
	}
	// Module defaults may still be a Go template until helm render.
	if strings.Contains(topic, "{{") {
		return nil
	}
	if len(topic) > kafkaTopicNameMaxLength {
		return errors.ErrInvalid.WithMsgf("env variable '%s' exceeds kafka topic name limit of %d characters", confDLQKafkaTopic, kafkaTopicNameMaxLength)
	}
	if !kafkaTopicNamePattern.MatchString(topic) {
		return errors.ErrInvalid.WithMsgf("env variable '%s' contains characters that are not allowed in a kafka topic name", confDLQKafkaTopic)
	}
	return validateDLQTopicRetention(envVars)
}

func validateDLQTopicRetention(envVars map[string]string) error {
	raw := strings.TrimSpace(envVars[confDLQKafkaTopicRetention])
	if raw == "" || strings.Contains(raw, "{{") {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return errors.ErrInvalid.WithMsgf("env variable '%s' must be an integer number of seconds", confDLQKafkaTopicRetention)
	}
	if n < minDLQTopicRetentionSec || n > maxDLQTopicRetentionSec {
		return errors.ErrInvalid.WithMsgf("env variable '%s' must be between %d and %d seconds", confDLQKafkaTopicRetention, minDLQTopicRetentionSec, maxDLQTopicRetentionSec)
	}
	return nil
}
