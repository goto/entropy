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

A firehose names its kafka resource through `stream_name`, and flags a secured one with
`stream_security_enabled`. The stream's security profile — the same `security` block the
kafka module exposes on its output — is then resolved during Plan, in this order:

1. an inline `stream_security` entry (prefetched by Dex),
2. a declared kafka dependency keyed by the stream name,
3. the kafka resource fetched internally by URN
   (`orn:entropy:kafka:<project>:<stream_name>`), with no dependency declared.

The flag exists so plaintext firehoses cost no lookup: the caller already knows whether a
profile exists, because Dex reads the same resource for the broker address. `stream_name`
is sent for plaintext streams too, so a stream that *loses* its ACLs gets the wiring a
previous plan left behind cleared.

The wiring mirrors odin's firehose adapter, so a migrated firehose renders the same pod
spec it does today. From the resolved profile the module injects the
`SOURCE_KAFKA_CONSUMER_CONFIG_*` env variables — security protocol, SASL mechanism, SSL
protocol, truststore type, and for a stream carrying certs a fixed
`SSL_TRUSTSTORE_LOCATION` of `/etc/secret/kafka-cert/truststore.p12` (`.jks` for JKS) plus
the `SSL_TRUSTSTORE_FILENAME` the chart uses to select that key out of the secret. The cert
gets its own subdirectory rather than odin's bare `/etc/secret`, because the chart already
mounts its sink-credential secret there. OAUTHBEARER
additionally gets the `OAuthBearerLoginModule` JAAS string and the pod login callback
handler class.

No secret value ever passes through entropy. The material is described as references in the
`acl` config, rendered as the `kafka_security` chart value alongside `service_account`:

| field | what the chart does with it |
| :--- | :--- |
| `ssl_config_credential` + `truststore_filename` | mounts that existing secret at `/etc/secret/kafka-cert`, selecting the filename as both key and path |
| `truststore_password` (`secretName` + `key`) | renders `SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD` as a `secretKeyRef` env var |
| `jaas_config_credential` | mounts the PLAIN/SCRAM `jaas.conf` secret at `/etc/secret/kafka` |
| `kafka_token_enabled` | adds the projected service-account token (`audience: kafka`) at `/var/run/secrets/kafka/serviceaccount` |

PLAIN/SCRAM streams never inline credentials or a JAAS string: they read a mounted
`jaas.conf`, and the module appends
`-Djava.security.auth.login.config=/etc/secret/kafka/jaas.conf` to `_JAVA_OPTIONS` (the
rest of that variable is left alone). The secret is the profile's `acls[team].secretName`
when set, otherwise odin's `<team>-<stream>-jaas` convention.

Plaintext firehoses are untouched — no injected config, no `kafka_security` value, no
chart value changes.

Naming a stream also relaxes the schema's `SOURCE_KAFKA_BROKERS` requirement: when the
stream is resolved, brokers are filled from its `url` unless the payload set them. Plan
fails if neither supplied them, rather than deploying a firehose with no brokers — note
that a stream named without the flag resolves nothing, so brokers must be explicit. Dex
sends the address itself in every case, so its value always wins.

The callback handler class and default service account are deployment level settings under
the module's driver config:

```json
{
  "kafka_security": {
    "sasl_login_callback_handler_class": "io.gtflabs.kafka.security.oauthbearer.kubernetes.PodLoginCallbackHandler",
    "service_account": "aegis-kafka"
  }
}
```

> Requires firehose chart `0.2.1` or later (`goto/charts`, `stable/firehose`). Chart `0.2.0`
> renders none of this — it has no `serviceAccountName`, projected volume or `secretKeyRef`
> support — so an ACL firehose on it starts without its secrets and fails to authenticate.

## Kafka DLQ defaults (self-serve ODS)

Env variables on the firehose **module** are merged under resource env vars. Render
templates against resource labels (`name`, `urn`, `namespace`, …) even when Telegraf is
disabled, so `DLQ_KAFKA_TOPIC={{ .name }}-firehose-dlq` works.

Set these on the ODS firehose modules (`gjk-p-acc`, `gjk-i-acc`, `al-oddp-id-p`,
`al-gtdp-id-p`, `al-oddp-id-s`, `al-gtdp-id-s`):

| key | value |
| :--- | :--- |
| `DLQ_KAFKA_TOPIC_CREATE` | `true` |
| `DLQ_KAFKA_TOPIC` | `{{ .name }}-firehose-dlq` |
| `DLQ_KAFKA_TOPIC_RETENTION` | `604800` |
| `DLQ_KAFKA_RETRIES` | `5` |
| `DLQ_RETRY_MAX_ATTEMPTS` | `5` on `gjk-p-acc` and `gjk-i-acc` |

`ERROR_TYPES_FOR_DLQ` default `DEFAULT_ERROR,SINK_RETRYABLE_ERROR` on Tencent modules
(`gjk-p-acc`, `gjk-i-acc`). Alicloud modules should leave it unset so blob DLQ keeps the
existing module defaults; Dex sets it when Kafka DLQ is first enabled.

When Kafka DLQ is enabled and `DLQ_KAFKA_BROKERS` is empty, plan fetches the project's
dagstream kafka resource `orn:entropy:kafka:<project>:dagstream` and uses its output
`url`. An explicit `DLQ_KAFKA_BROKERS` value is honoured.

When `DLQ_SINK_ENABLE=true` and `DLQ_WRITER_TYPE=KAFKA`, plan rejects a missing or
invalid `DLQ_KAFKA_TOPIC` (max 249 characters, `[a-zA-Z0-9._-]`). Values that still
contain `{{` are skipped until helm render. `DLQ_KAFKA_TOPIC_RETENTION` must be
between 86400 and 604800 seconds when set.

Kafka Governance cannot set topic retention; Firehose uses `DLQ_KAFKA_TOPIC_CREATE`
and `DLQ_KAFKA_TOPIC_RETENTION` on the cluster. Dex keeps an already-configured
custom DLQ topic name instead of overwriting it with `{{ .name }}-firehose-dlq`.