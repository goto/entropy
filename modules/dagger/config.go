package dagger

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

const helmReleaseNameMaxLength = 53
const keyStreams = "STREAMS"
const keyFlinkJobID = "FLINK_JOB_ID"
const keySinkInfluxURL = "SINK_INFLUX_URL"
const keySinkInfluxPassword = "SINK_INFLUX_PASSWORD"
const keySinkInfluxDBName = "SINK_INFLUX_DB_NAME"
const keySinkInfluxUsername = "SINK_INFLUX_USERNAME"
const keySinkInfluxMeasurementName = "SINK_INFLUX_MEASUREMENT_NAME"
const keyRedisServer = "REDIS_SERVER"
const SourceKafkaConsumerConfigAutoCommitEnable = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_COMMIT_ENABLE"
const SourceKafkaConsumerConfigAutoOffsetReset = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"
const SinkTypeInflux = "influx"
const SinkTypeKafka = "kafka"
const keySinkKafkaBrokers = "SINK_KAFKA_BROKERS"
const keySinkKafkaStream = "SINK_KAFKA_STREAM"

var (
	//go:embed schema/config.json
	configSchemaRaw []byte

	validateConfig = validator.FromJSONSchema(configSchemaRaw)
)

type UsageSpec struct {
	CPU    string `json:"cpu,omitempty" validate:"required"`
	Memory string `json:"memory,omitempty" validate:"required"`
}

type Resources struct {
	TaskManager UsageSpec `json:"taskmanager,omitempty"`
	JobManager  UsageSpec `json:"jobmanager,omitempty"`
}

type Config struct {
	Resources    Resources         `json:"resources,omitempty"`
	FlinkName    string            `json:"flink_name,omitempty"`
	DeploymentID string            `json:"deployment_id,omitempty"`
	Streams      []Stream          `json:"streams,omitempty"`
	JobId        string            `json:"job_id,omitempty"`
	Savepoint    any               `json:"savepoint,omitempty"`
	EnvVariables map[string]string `json:"env_variables,omitempty"`
	ChartValues  *ChartValues      `json:"chart_values,omitempty"`
	Deleted      bool              `json:"deleted,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Replicas     int               `json:"replicas"`
	SinkType     string            `json:"sink_type"`
}

type ChartValues struct {
	ChartVersion string `json:"chart_version" validate:"required"`
}

type SourceDetail struct {
	SourceName string `json:"SOURCE_NAME"`
	SourceType string `json:"SOURCE_TYPE"`
}

type Stream struct {
	InputSchemaTable                          string         `json:"INPUT_SCHEMA_TABLE"`
	SourceDetails                             []SourceDetail `json:"SOURCE_DETAILS"`
	SourceKafkaConsumerConfigAutoCommitEnable string         `json:"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_COMMIT_ENABLE"`
	SourceKafkaConsumerConfigAutoOffsetReset  string         `json:"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"`
	InputSchemaProtoClass                     string         `json:"INPUT_SCHEMA_PROTO_CLASS"`
	SourceKafkaTopicNames                     string         `json:"SOURCE_KAFKA_TOPIC_NAMES"`
	SourceKafkaName                           string         `json:"SOURCE_KAFKA_NAME"`
	InputSchemaEventTimestampFieldIndex       string         `json:"INPUT_SCHEMA_EVENT_TIMESTAMP_FIELD_INDEX"`
	SourceParquetFileDateRange                interface{}    `json:"SOURCE_PARQUET_FILE_DATE_RANGE"`
	SourceParquetFilePaths                    interface{}    `json:"SOURCE_PARQUET_FILE_PATHS"`
	SourceKafkaConsumerConfigGroupID          string         `json:"SOURCE_KAFKA_CONSUMER_CONFIG_GROUP_ID"`
}

func readConfig(r module.ExpandedResource, confJSON json.RawMessage, dc driverConf) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(confJSON, &cfg)
	if err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	//transformation #1
	streamsJSON := cfg.EnvVariables[keyStreams]
	var streams []Stream
	err = json.Unmarshal([]byte(streamsJSON), &streams)
	if err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	for i := range streams {
		if len(streams[i].SourceDetails) == 0 {
			streams[i].SourceDetails = []SourceDetail{
				{
					SourceName: "KAFKA_CONSUMER",
					SourceType: "UNBOUNDED",
				},
			}
		}
	}

	//transformation #2
	cfg.EnvVariables = modules.CloneAndMergeMaps(dc.EnvVariables, cfg.EnvVariables)

	//transformation #3
	var flinkOut flink.Output
	if err := json.Unmarshal(r.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	if cfg.Namespace == "" {
		ns := flinkOut.KubeNamespace
		cfg.Namespace = ns
	}

	//transformation #4
	//transform resource name to safe length

	//transformation #5
	cfg.EnvVariables[keyFlinkJobID] = r.Name

	//transformation #6
	// note: enforce the kubernetes deployment name length limit.
	if len(cfg.DeploymentID) == 0 {
		cfg.DeploymentID = modules.SafeName(fmt.Sprintf("%s-%s", r.Project, r.Name), "-dagger", helmReleaseNameMaxLength)
	} else if len(cfg.DeploymentID) > helmReleaseNameMaxLength {
		return nil, errors.ErrInvalid.WithMsgf("deployment_id must not have more than 53 chars")
	}

	//transformation #7
	cfg.EnvVariables[keySinkInfluxURL] = flinkOut.Influx.URL
	cfg.EnvVariables[keySinkInfluxPassword] = flinkOut.Influx.Password
	cfg.EnvVariables[keySinkInfluxUsername] = flinkOut.Influx.Username
	//TODO: add sink influx db name
	//TODO: check if SINK_INFLUX_MEASUREMENT_NAME and REDIS_SERVER needs modification

	//transformation #8
	//Longbow related  transformation skipped

	//transformation #9 and #11
	for i := range streams {
		streams[i].SourceKafkaConsumerConfigGroupID = incrementGroupId(r.Name+"-0001", i)
		streams[i].SourceKafkaConsumerConfigAutoCommitEnable = dc.EnvVariables[SourceKafkaConsumerConfigAutoCommitEnable]
		streams[i].SourceKafkaConsumerConfigAutoOffsetReset = dc.EnvVariables[SourceKafkaConsumerConfigAutoOffsetReset]
		//TODO: add stream URL for key SOURCE_KAFKA_CONSUMER_CONFIG_BOOTSTRAP_SERVERS

		cfg.Streams = append(cfg.Streams, Stream{
			SourceKafkaName:                  streams[i].SourceKafkaName,
			SourceKafkaConsumerConfigGroupID: streams[i].SourceKafkaConsumerConfigGroupID,
		})
	}

	//transformation #10
	//this shall check if the project of the conf.EnvVars.STREAMS is same as that of the corresponding flink
	//do we need to check this?

	//transformation #12
	cfg.EnvVariables[keyStreams] = string(mustMarshalJSON(streams))

	//transformation #13
	if cfg.SinkType == SinkTypeKafka {
		cfg.EnvVariables[keySinkKafkaStream] = flinkOut.SinkKafkaStream
		//TODO cfg.EnvVariables[keySinkKafkaBrokers] = stream URL
	}

	//transformation #14
	cfg.Resources = mergeResources(dc.Resources, cfg.Resources)

	if cfg.Replicas <= 0 {
		cfg.Replicas = 1
	}

	if err := validateConfig(confJSON); err != nil {
		return nil, err
	}

	return &cfg, nil
}
func incrementGroupId(groupId string, step int) string {
	incrementNumberInString := func(number string) int {
		num, _ := strconv.Atoi(number)
		return num + step
	}

	leftZeroPad := func(number int) string {
		return fmt.Sprintf("%04d", number)
	}

	getLastAndRestFromArray := func(arr []string) ([]string, string) {
		return arr[:len(arr)-1], arr[len(arr)-1]
	}

	parts := strings.Split(groupId, "-")
	name, number := getLastAndRestFromArray(parts)
	updatedNumber := leftZeroPad(incrementNumberInString(number))
	return strings.Join(append(name, updatedNumber), "-")
}

func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return data
}

func mergeResources(defaultResources, currResources Resources) Resources {
	if currResources.TaskManager.CPU == "" {
		currResources.TaskManager.CPU = defaultResources.TaskManager.CPU
	}
	if currResources.TaskManager.Memory == "" {
		currResources.TaskManager.Memory = defaultResources.TaskManager.Memory
	}
	if currResources.JobManager.CPU == "" {
		currResources.JobManager.CPU = defaultResources.JobManager.CPU
	}
	if currResources.JobManager.Memory == "" {
		currResources.JobManager.Memory = defaultResources.JobManager.Memory
	}
	return currResources
}
