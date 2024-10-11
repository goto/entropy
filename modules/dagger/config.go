package dagger

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

const (
	helmReleaseNameMaxLength = 45
)

// Stream-related constants
const (
	keyStreams  = "STREAMS"
	keySinkType = "SINK_TYPE"
)

// Flink-related constants
const (
	keyFlinkJobID = "FLINK_JOB_ID"
)

// Influx-related constants
const (
	keySinkInfluxURL             = "SINK_INFLUX_URL"
	keySinkInfluxPassword        = "SINK_INFLUX_PASSWORD"
	keySinkInfluxDBName          = "SINK_INFLUX_DB_NAME"
	keySinkInfluxUsername        = "SINK_INFLUX_USERNAME"
	keySinkInfluxMeasurementName = "SINK_INFLUX_MEASUREMENT_NAME"
)

// Kafka-related constants
const (
	SourceKafkaConsumerConfigAutoCommitEnable = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_COMMIT_ENABLE"
	SourceKafkaConsumerConfigAutoOffsetReset  = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"
	SourceKafkaConsumerConfigBootstrapServers = "SOURCE_KAFKA_CONSUMER_CONFIG_BOOTSTRAP_SERVERS"
	keySinkKafkaBrokers                       = "SINK_KAFKA_BROKERS"
	keySinkKafkaStream                        = "SINK_KAFKA_STREAM"
	keySinkKafkaProtoMsg                      = "SINK_KAFKA_PROTO_MESSAGE"
	keySinkKafkaTopic                         = "SINK_KAFKA_TOPIC"
	keySinkKafkaKey                           = "SINK_KAFKA_PROTO_KEY"
	keySinkKafkaLingerMs                      = "SINK_KAFKA_LINGER_MS"
)

// Sink types
const (
	SinkTypeInflux   = "INFLUX"
	SinkTypeKafka    = "KAFKA"
	SinkTypeBigquery = "BIGQUERY"
)

// BigQuery-related constants
const (
	keySinkBigqueryGoogleCloudProjectID    = "SINK_BIGQUERY_GOOGLE_CLOUD_PROJECT_ID"
	keySinkBigqueryDatasetName             = "SINK_BIGQUERY_DATASET_NAME"
	keySinkBigqueryTableName               = "SINK_BIGQUERY_TABLE_NAME"
	keySinkBigqueryDatasetLabels           = "SINK_BIGQUERY_DATASET_LABELS"
	keySinkBigqueryTableLabels             = "SINK_BIGQUERY_TABLE_LABELS"
	keySinkBigqueryTablePartitioningEnable = "SINK_BIGQUERY_TABLE_PARTITIONING_ENABLE"
	keySinkBigqueryTableClusteringEnable   = "SINK_BIGQUERY_TABLE_CLUSTERING_ENABLE"
	keySinkBigqueryBatchSize               = "SINK_BIGQUERY_BATCH_SIZE"
	keySinkBigqueryTablePartitionKey       = "SINK_BIGQUERY_TABLE_PARTITION_KEY"
	keySinkBigqueryRowInsertIDEnable       = "SINK_BIGQUERY_ROW_INSERT_ID_ENABLE"
	keySinkBigqueryClientReadTimeoutMs     = "SINK_BIGQUERY_CLIENT_READ_TIMEOUT_MS"
	keySinkBigqueryClientConnectTimeoutMs  = "SINK_BIGQUERY_CLIENT_CONNECT_TIMEOUT_MS"
	keySinkBigqueryTablePartitionExpiryMs  = "SINK_BIGQUERY_TABLE_PARTITION_EXPIRY_MS"
	keySinkBigqueryDatasetLocation         = "SINK_BIGQUERY_DATASET_LOCATION"
	keySinkErrorTypesForFailure            = "SINK_ERROR_TYPES_FOR_FAILURE"
	keySinkBigqueryTableClusteringKeys     = "SINK_BIGQUERY_TABLE_CLUSTERING_KEYS"
)

var (
	//go:embed schema/config.json
	configSchemaRaw []byte

	validateConfig = validator.FromJSONSchema(configSchemaRaw)
)

type StartParams struct {
	StopTime *time.Time `json:"stop_time"`
}

type UsageSpec struct {
	CPU    string `json:"cpu,omitempty" validate:"required"`
	Memory string `json:"memory,omitempty" validate:"required"`
}

type Resources struct {
	TaskManager UsageSpec `json:"taskmanager,omitempty"`
	JobManager  UsageSpec `json:"jobmanager,omitempty"`
}

type Config struct {
	Resources     Resources         `json:"resources,omitempty"`
	Source        []Source          `json:"source,omitempty"`
	Sink          Sink              `json:"sink,omitempty"`
	EnvVariables  map[string]string `json:"env_variables,omitempty"`
	Replicas      int               `json:"replicas"`
	SinkType      string            `json:"sink_type"`
	Team          string            `json:"team"`
	FlinkName     string            `json:"flink_name,omitempty"`
	DeploymentID  string            `json:"deployment_id,omitempty"`
	Savepoint     any               `json:"savepoint,omitempty"`
	ChartValues   *ChartValues      `json:"chart_values,omitempty"`
	Deleted       bool              `json:"deleted,omitempty"`
	Namespace     string            `json:"namespace,omitempty"`
	PrometheusURL string            `json:"prometheus_url,omitempty"`
	JarURI        string            `json:"jar_uri,omitempty"`
	State         string            `json:"state"`
	JobState      string            `json:"job_state"`
	ResetOffset   string            `json:"reset_offset"`
	StopTime      *time.Time        `json:"stop_time,omitempty"`
}

type ChartValues struct {
	ImageRepository string `json:"image_repository" validate:"required"`
	ImageTag        string `json:"image_tag" validate:"required"`
	ChartVersion    string `json:"chart_version" validate:"required"`
	ImagePullPolicy string `json:"image_pull_policy"`
}

type SourceDetail struct {
	SourceName string `json:"SOURCE_NAME"`
	SourceType string `json:"SOURCE_TYPE"`
}

type SourceKafka struct {
	SourceKafkaConsumerConfigAutoCommitEnable string `json:"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_COMMIT_ENABLE"`
	SourceKafkaConsumerConfigAutoOffsetReset  string `json:"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"`
	SourceKafkaTopicNames                     string `json:"SOURCE_KAFKA_TOPIC_NAMES"`
	SourceKafkaName                           string `json:"SOURCE_KAFKA_NAME"`
	SourceKafkaConsumerConfigGroupID          string `json:"SOURCE_KAFKA_CONSUMER_CONFIG_GROUP_ID"`
	SourceKafkaConsumerConfigBootstrapServers string `json:"SOURCE_KAFKA_CONSUMER_CONFIG_BOOTSTRAP_SERVERS"`
}

type SourceParquet struct {
	SourceParquetFileDateRange interface{} `json:"SOURCE_PARQUET_FILE_DATE_RANGE"`
	SourceParquetFilePaths     []string    `json:"SOURCE_PARQUET_FILE_PATHS"`
}

type Source struct {
	InputSchemaProtoClass               string         `json:"INPUT_SCHEMA_PROTO_CLASS"`
	InputSchemaEventTimestampFieldIndex string         `json:"INPUT_SCHEMA_EVENT_TIMESTAMP_FIELD_INDEX"`
	SourceDetails                       []SourceDetail `json:"SOURCE_DETAILS"`
	InputSchemaTable                    string         `json:"INPUT_SCHEMA_TABLE"`
	SourceKafka
	SourceParquet
}

type SinkKafka struct {
	SinkKafkaBrokers  string `json:"SINK_KAFKA_BROKERS"`
	SinkKafkaStream   string `json:"SINK_KAFKA_STREAM"`
	SinkKafkaTopic    string `json:"SINK_KAFKA_TOPIC"`
	SinkKafkaProtoMsg string `json:"SINK_KAFKA_PROTO_MESSAGE"`
	SinkKafkaLingerMs string `json:"SINK_KAFKA_LINGER_MS"`
	SinkKafkaProtoKey string `json:"SINK_KAFKA_PROTO_KEY"`
}

type SinkInflux struct {
	SinkInfluxDBName          string `json:"SINK_INFLUX_DB_NAME"`
	SinkInfluxMeasurementName string `json:"SINK_INFLUX_MEASUREMENT_NAME"`
}

type SinkBigquery struct {
	SinkBigqueryGoogleCloudProjectID    string `json:"SINK_BIGQUERY_GOOGLE_CLOUD_PROJECT_ID"`
	SinkBigqueryTableName               string `json:"SINK_BIGQUERY_TABLE_NAME"`
	SinkBigqueryDatasetLabels           string `json:"SINK_BIGQUERY_DATASET_LABELS"`
	SinkBigqueryTableLabels             string `json:"SINK_BIGQUERY_TABLE_LABELS"`
	SinkBigqueryDatasetName             string `json:"SINK_BIGQUERY_DATASET_NAME"`
	SinkBigqueryTablePartitioningEnable string `json:"SINK_BIGQUERY_TABLE_PARTITIONING_ENABLE"`
	SinkBigqueryTablePartitionKey       string `json:"SINK_BIGQUERY_TABLE_PARTITION_KEY"`
	SinkBigqueryRowInsertIDEnable       string `json:"SINK_BIGQUERY_ROW_INSERT_ID_ENABLE"`
	SinkBigqueryClientReadTimeoutMs     string `json:"SINK_BIGQUERY_CLIENT_READ_TIMEOUT_MS"`
	SinkBigqueryClientConnectTimeoutMs  string `json:"SINK_BIGQUERY_CLIENT_CONNECT_TIMEOUT_MS"`
	SinkBigqueryTablePartitionExpiryMs  string `json:"SINK_BIGQUERY_TABLE_PARTITION_EXPIRY_MS"`
	SinkBigqueryDatasetLocation         string `json:"SINK_BIGQUERY_DATASET_LOCATION"`
	SinkBigqueryBatchSize               string `json:"SINK_BIGQUERY_BATCH_SIZE"`
	SinkBigqueryTableClusteringEnable   string `json:"SINK_BIGQUERY_TABLE_CLUSTERING_ENABLE"`
	SinkBigqueryTableClusteringKeys     string `json:"SINK_BIGQUERY_TABLE_CLUSTERING_KEYS"`
	SinkErrorTypesForFailure            string `json:"SINK_ERROR_TYPES_FOR_FAILURE"`
}

type Sink struct {
	SinkKafka
	SinkInflux
	SinkBigquery
}

func readConfig(r module.ExpandedResource, confJSON json.RawMessage, dc driverConf) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(confJSON, &cfg)
	if err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	//transformation #9 and #11
	//transformation #1
	source := cfg.Source

	if !(source[0].SourceParquet.SourceParquetFilePaths != nil && len(source[0].SourceParquet.SourceParquetFilePaths) > 0) {
		for i := range source {
			//TODO: check how to handle increment group id on update
			if source[i].SourceKafkaConsumerConfigGroupID == "" {
				source[i].SourceKafkaConsumerConfigGroupID = incrementGroupId(r.Name+"-0001", i)
			}
			if source[i].SourceKafkaConsumerConfigAutoCommitEnable == "" {
				source[i].SourceKafkaConsumerConfigAutoCommitEnable = dc.EnvVariables[SourceKafkaConsumerConfigAutoCommitEnable]
			}
			if source[i].SourceKafkaConsumerConfigAutoOffsetReset == "" {
				source[i].SourceKafkaConsumerConfigAutoOffsetReset = dc.EnvVariables[SourceKafkaConsumerConfigAutoOffsetReset]
			}
			if source[i].SourceKafkaConsumerConfigBootstrapServers == "" {
				source[i].SourceKafkaConsumerConfigBootstrapServers = dc.EnvVariables[SourceKafkaConsumerConfigBootstrapServers]
			}
		}
	}

	cfg.Source = source

	//transformation #2
	cfg.EnvVariables = modules.CloneAndMergeMaps(dc.EnvVariables, cfg.EnvVariables)

	//transformation #3
	var flinkOut flink.Output
	if err := json.Unmarshal(r.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid flink state").WithCausef(err.Error())
	}

	cfg.Namespace = flinkOut.KubeNamespace

	//transformation #4
	//transform resource name to safe length

	//transformation #5
	//TODO: build name from title as project-<title>-dagger
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
	cfg.EnvVariables[keySinkInfluxUsername] = flinkOut.Influx.Username

	//SINK_INFLUX_DB_NAME is added by client
	//SINK_INFLUX_MEASUREMENT_NAME is added by client
	//REDIS_SERVER is skipped

	//transformation #8
	//Longbow configs would be in base configs

	//transformation #10
	//this shall check if the project of the conf.EnvVars.STREAMS is same as that of the corresponding flink
	//do we need to check this?

	//transformation #13
	cfg.EnvVariables[keySinkType] = cfg.SinkType
	if cfg.SinkType == SinkTypeKafka {
		cfg.EnvVariables[keySinkKafkaStream] = cfg.Sink.SinkKafka.SinkKafkaStream
		cfg.EnvVariables[keySinkKafkaBrokers] = cfg.Sink.SinkKafka.SinkKafkaBrokers
		cfg.EnvVariables[keySinkKafkaProtoMsg] = cfg.Sink.SinkKafka.SinkKafkaProtoMsg
		cfg.EnvVariables[keySinkKafkaTopic] = cfg.Sink.SinkKafka.SinkKafkaTopic
		cfg.EnvVariables[keySinkKafkaKey] = cfg.Sink.SinkKafka.SinkKafkaProtoKey
		cfg.EnvVariables[keySinkKafkaLingerMs] = cfg.Sink.SinkKafka.SinkKafkaLingerMs
	} else if cfg.SinkType == SinkTypeInflux {
		cfg.EnvVariables[keySinkInfluxPassword] = flinkOut.Influx.Password
		cfg.EnvVariables[keySinkInfluxDBName] = cfg.Sink.SinkInflux.SinkInfluxDBName
		cfg.EnvVariables[keySinkInfluxMeasurementName] = cfg.Sink.SinkInflux.SinkInfluxMeasurementName
	} else if cfg.SinkType == SinkTypeBigquery {
		cfg.EnvVariables[keySinkBigqueryGoogleCloudProjectID] = cfg.Sink.SinkBigquery.SinkBigqueryGoogleCloudProjectID
		cfg.EnvVariables[keySinkBigqueryDatasetName] = cfg.Sink.SinkBigquery.SinkBigqueryDatasetName
		cfg.EnvVariables[keySinkBigqueryTableName] = cfg.Sink.SinkBigquery.SinkBigqueryTableName
		cfg.EnvVariables[keySinkBigqueryDatasetLabels] = cfg.Sink.SinkBigquery.SinkBigqueryDatasetLabels
		cfg.EnvVariables[keySinkBigqueryTableLabels] = cfg.Sink.SinkBigquery.SinkBigqueryTableLabels
		cfg.EnvVariables[keySinkBigqueryTablePartitioningEnable] = cfg.Sink.SinkBigquery.SinkBigqueryTablePartitioningEnable
		cfg.EnvVariables[keySinkBigqueryTablePartitionKey] = cfg.Sink.SinkBigquery.SinkBigqueryTablePartitionKey
		cfg.EnvVariables[keySinkBigqueryRowInsertIDEnable] = cfg.Sink.SinkBigquery.SinkBigqueryRowInsertIDEnable
		cfg.EnvVariables[keySinkBigqueryClientReadTimeoutMs] = cfg.Sink.SinkBigquery.SinkBigqueryClientReadTimeoutMs
		cfg.EnvVariables[keySinkBigqueryClientConnectTimeoutMs] = cfg.Sink.SinkBigquery.SinkBigqueryClientConnectTimeoutMs
		cfg.EnvVariables[keySinkBigqueryTablePartitionExpiryMs] = cfg.Sink.SinkBigquery.SinkBigqueryTablePartitionExpiryMs
		cfg.EnvVariables[keySinkBigqueryDatasetLocation] = cfg.Sink.SinkBigquery.SinkBigqueryDatasetLocation
		cfg.EnvVariables[keySinkBigqueryBatchSize] = cfg.Sink.SinkBigquery.SinkBigqueryBatchSize
		cfg.EnvVariables[keySinkBigqueryTableClusteringEnable] = cfg.Sink.SinkBigquery.SinkBigqueryTableClusteringEnable
		cfg.EnvVariables[keySinkBigqueryTableClusteringKeys] = cfg.Sink.SinkBigquery.SinkBigqueryTableClusteringKeys
		cfg.EnvVariables[keySinkErrorTypesForFailure] = cfg.Sink.SinkBigquery.SinkErrorTypesForFailure
	}

	//transformation #14
	cfg.Resources = mergeResources(dc.Resources, cfg.Resources)

	cfg.PrometheusURL = flinkOut.PrometheusURL
	cfg.FlinkName = flinkOut.FlinkName

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

func mergeResources(oldResources, newResources Resources) Resources {
	if newResources.TaskManager.CPU == "" {
		newResources.TaskManager.CPU = oldResources.TaskManager.CPU
	}
	if newResources.TaskManager.Memory == "" {
		newResources.TaskManager.Memory = oldResources.TaskManager.Memory
	}
	if newResources.JobManager.CPU == "" {
		newResources.JobManager.CPU = oldResources.JobManager.CPU
	}
	if newResources.JobManager.Memory == "" {
		newResources.JobManager.Memory = oldResources.JobManager.Memory
	}
	return newResources
}
