# Firehose

[Firehose](https://goto.github.io/firehose/) is an extensible, no-code, and cloud-native service to load real-time streaming data from Kafka to data stores, data lakes, and analytical storage systems.

## What happens in Plan?

Plan handles two actions in the firehose module. It creates a new reosurce or updates(change) the exisiting ones. 
While creating a new firehose, simply a ***release_create*** step is added to the ***moduleData***. Updation in firehose adds ***release_create*** step to the ***moduleData***. It can either be `scale`, `start`, `stop` action or it can just be an `update` of other firehose configs. Firehose configs are adjusted here.

## What happens in Sync?

Sync in Firehose would receive pending step which will be either a "release_create" or "release_update", and it uses a helm client to implementation it.

## Firehose Module Configuration

The configuration struct for Firehose module looks like:

```
type moduleConfig struct {
	State        string `json:"state"`
	Firehose     struct {
		Replicas           int               `json:"replicas"`
		KafkaBrokerAddress string            `json:"kafka_broker_address"`
		KafkaTopic         string            `json:"kafka_topic"`
		KafkaConsumerID    string            `json:"kafka_consumer_id"`
		EnvVariables       map[string]string `json:"env_variables"`
	} `json:"firehose"`
}
```

| Fields | |
| :--- | :--- |
| `State` | `string` State of the firehose, "RUNNING" or "STOPPED". |
| `ChartVersion` | `string` Chart version you want to use. |
| `Firehose` | `struct` Holds firehose configuration. |

Detailed JSONSchema for config can be referenced [here](https://github.com/goto/entropy/blob/main/modules/firehose/schema/config.json).

## Kafka ACL (SASL/SSL) source streams

A firehose reading from a secured stream names its kafka resource through `stream_name`
(or the `SOURCE_KAFKA_NAME` env variable). The stream's security profile — the same
`security` block the kafka module exposes on its output — is resolved during Plan, in
this order:

1. an inline `stream_security` entry (prefetched by Dex),
2. a declared kafka dependency keyed by the stream name,
3. the kafka resource fetched internally by URN, when `stream_security_enabled` is set
   (or `SOURCE_KAFKA_SECURITY_ENABLED=true` is passed as an env variable — it is stripped
   before the config reaches the running firehose).

From the resolved profile the module injects the `SOURCE_KAFKA_CONSUMER_CONFIG_*` env
variables (security protocol, SASL mechanism, JAAS config, truststore location/password)
and records the secret volumes in `acl_mounts`, which is rendered as the `acl_mounts`
chart value alongside `service_account`. Credentials are never inlined: username, password
and truststore password are referenced through the `literalfile` config provider pointing
at the mounted secrets. Plaintext firehoses are untouched — no injected config, no mounts,
no chart value changes.

The provider/callback classes and the default service account are deployment level
settings under the module's driver config:

```json
{
  "kafka_security": {
    "config_provider_class": "com.example.kafka.configproviders.LiteralFileConfigProvider",
    "sasl_login_callback_handler_class": "com.example.kafka.security.PodLoginCallbackHandler",
    "service_account": "aegis-kafka"
  }
}
```