# Job

The `job` module runs a Kubernetes `Job` for one-off or scheduled workloads (e.g. Dagger jobs).

## Job Module Configuration

The driver config for the `job` module (set on the module, not per-resource) looks like:

```
type DriverConf struct {
	Namespace         string                       `json:"namespace"`
	RequestsAndLimits map[string]RequestsAndLimits `json:"requestsAndLimits"`
	EnvVariables      map[string]string            `json:"env_variables"`
	Containers        map[string]ContainerOverride `json:"containers,omitempty"`
}

type ContainerOverride struct {
	EnvVariables map[string]string `json:"env_variables,omitempty"`
}
```

| Fields | |
| :--- | :--- |
| `Namespace` | `string` Default namespace used when the resource spec doesn't set one. |
| `RequestsAndLimits` | `struct` Default CPU/memory requests and limits, used when a container doesn't set its own. |
| `EnvVariables` | `map[string]string` Env vars merged onto **every** container's `env_variables`; the client-provided per-container value wins on key conflict. |
| `Containers` | `map[string]ContainerOverride` Per-container env var overrides, keyed by container name (`configs.containers.{container_name}.env_variables`). |

### Per-container env variable override

Entries under `Containers` overlay onto the matching container's `env_variables` **after** the
global `EnvVariables` merge above, and the **module value always wins** — regardless of whether
the client sent a real or masked (`****-<fp>`) value for that key. A container name with no entry
is left untouched; an entry naming a container absent from the resource spec is ignored.

This exists so a job created by an automated caller (e.g. Dex re-dumping masked placeholder values
from a firehose resource into a new job's env vars on `Create`) still launches with real secrets:
masking's `Restore` only recovers a stored value on `Update`, so on `Create` there is nothing to
restore. This override is independent of masking and `sensitive_configs` — it is a per-key overlay
that always takes effect, regardless of the masking feature's state.

Values must be sourced from the secret manager via `${...}` placeholders, never hardcoded:

```json
{
  "namespace": "jobs",
  "containers": {
    "driver": {
      "env_variables": {
        "SOURCE_KAFKA_PASSWORD": "${vault:kafka#password}"
      }
    }
  }
}
```
