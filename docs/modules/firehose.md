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

The wiring mirrors odin's firehose adapter, so a migrated firehose renders the same pod
spec it does today. From the resolved profile the module injects the
`SOURCE_KAFKA_CONSUMER_CONFIG_*` env variables — security protocol, SASL mechanism, SSL
protocol, truststore type, and for a stream carrying certs a fixed
`SSL_TRUSTSTORE_LOCATION` of `/etc/secret/truststore.p12` (`.jks` for JKS) plus the
`SSL_TRUSTSTORE_FILENAME` the chart uses to select that key out of the secret. OAUTHBEARER
additionally gets the `OAuthBearerLoginModule` JAAS string and the pod login callback
handler class.

No secret value ever passes through entropy. The material is described as references in the
`acl` config, rendered as the `kafka_security` chart value alongside `service_account`:

| field | what the chart does with it |
| :--- | :--- |
| `ssl_config_credential` + `truststore_filename` | mounts that existing secret at `/etc/secret`, selecting the filename as both key and path |
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

> The published `firehose` chart (0.2.0) does not yet render any of this: its `mountSecrets`
> builds a *new* secret from inline values, and it has no `serviceAccountName`, projected
> volume or `secretKeyRef` support. Those four template blocks — ports of odin's
> `manifests/firehose.yaml` — are needed before an ACL firehose can run.