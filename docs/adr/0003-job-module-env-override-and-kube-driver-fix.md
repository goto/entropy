# ADR: Job-Creation Module Config — Per-Container Env Override + Nil-Driver Panic Fix

**Domain:** systems
**Complexity:** S
**Priority:** TBD
**Assignee:** TBD
**Created:** 2026-07-24
**Status:** In Progress
**MR:** TBD

*(Legacy field retired 2026-08 — kept here for the historical record: Estimated Effort: 2.5 man-days, Actual Effort: 1.5 man-days — not part of the current ADR template.)*

**Requirements:** [job-module-env-override](../requirements/0003-job-module-env-override.md)
**Related ADR:** [mask-sensitive-values-in-api-response](0001-mask-sensitive-values-in-api-response.md)

> **Merged record.** This ADR combines two previously separate records, both concerning how
> entropy handles **module driver config while creating a job resource**:
> `2026-07-24-adr-job-module-env-override.md` (Complexity S) and
> `2026-07-27-adr-kube-driver-nil-panic-fix.md` (Complexity XS). Both were `In Progress` at merge
> time; the estimate fields are the sums of the originals and are retained only as historical
> artifacts (effort is tracked outside this repo). Part B is an independent latent-bug fix, not a
> dependency of Part A — they ship together because they are touched by the same flow, and either
> part can be verified on its own.

## Context

Both problems surface on the same path: **Dex creates a `job` resource in entropy**, and the job
module's driver config is read to plan that job.

### Part A — placeholder secrets land in job containers

Masking ([mask-sensitive-values-in-api-response](0001-mask-sensitive-values-in-api-response.md)) now returns
placeholders (`****-<fp>`) for sensitive config values on read. When **Dex creates a job** from a
masked firehose resource, it dumps those placeholder values into the new job's container
`env_variables`. Masking's `Restore` only recovers a real secret on **Update** (where a stored
value exists); on **Create** there is nothing to restore, so the masked field is dropped and the
job launches with no real secret.

This change gives the job module its own escape hatch, **fully decoupled from masking**: the job
**module driver config** declares per-container `env_variables` overrides holding real secrets
(sourced from the secret manager via `${...}` placeholders). Entropy overlays these onto the
matching container on every Create/Update, with **module config always winning** per key —
regardless of whether the client value is masked or real, and regardless of whether masking is
enabled or what `sensitive_configs` contains.

### Part B — nil-driver panic resolving the `kube_cluster` dependency

Production panic when creating a `job` resource that depends on a `kubernetes` resource:

```
panic({0x269de40?, 0x4640cf0?})
github.com/goto/entropy/modules/kubernetes.(*kubeDriver).Output(...)
    modules/kubernetes/driver.go:78
```

`generateModuleSpec` (`core/core.go:53`) resolves the job's `kube_cluster` dependency by calling
`GetResource` on the kubernetes resource, which calls `GetOutput` → `driver.Output` on the
**kubernetes module's own driver** (built from the module's registration-level `Configs` for the
project, not the individual resource's `spec.configs`).

Root cause is in `modules/kubernetes/module.go:21-28`:

```go
DriverFactory: func(conf json.RawMessage) (module.Driver, error) {
    kd := &kubeDriver{}
    err := json.Unmarshal(conf, &kd)   // &kd is **kubeDriver
    ...
    return kd, nil
},
```

Passing `&kd` where `kd` is already `*kubeDriver` gives `json.Unmarshal` a `**kubeDriver`. Per
`encoding/json`'s documented null-handling, if `conf` is the JSON literal `null` (or effectively
empty), Unmarshal sets the pointee — `kd` itself — to `nil`. `err` stays `nil`, so `DriverFactory`
returns a **non-nil `module.Driver` interface wrapping a nil `*kubeDriver`**. The first field
dereference on the receiver, `m.TolerationMode` in `Output` (`driver.go:78`), then panics. Dormant,
preexisting bug — surfaces only once a job's `kube_cluster` dependency forces a live `Output` call
on a kubernetes module whose registration `Configs` is `null`/empty.

The identical pattern exists in `modules/flink/module.go:25-32` — same latent bug, fixed here too
even though no panic has been reported there yet.

## Approach

### Part A — per-container env overlay in `ReadConfig`

The job module already merges a **global** module `env_variables` map onto every container inside
`config.ReadConfig` (`modules/job/config/config.go:95`), with the **client winning**. We add a
**per-container** override map to the driver config and layer it in the same loop, immediately
after the global merge, with the **module winning**.

`ReadConfig` is the single funnel for `planCreate` (Create/Update, `act.Params`), `planPending*`,
`Sync`, and `Output`, so one change covers every path. `getJob` (`modules/job/driver/sync.go:101`)
consumes the already-merged map and needs no change.

Precedence layering, per container, in order:
1. `dc.EnvVariables` (global module defaults) — base
2. client `c.EnvVariables` — wins over (1) *(existing behavior, unchanged)*
3. `dc.Containers[c.Name].EnvVariables` (per-container module override) — wins over all *(new)*

Reuse `modules/utils.go:18-27` `CloneAndMergeMaps(m1, m2)` (m2 wins, no mutation, fresh map) for
both merges — override goes in the `m2` slot.

#### Config shape (module driver config JSON)

```json
{
  "namespace": "jobs",
  "containers": {
    "driver": {
      "env_variables": { "SOURCE_KAFKA_PASSWORD": "${vault:kafka#password}" }
    }
  }
}
```

New field on `DriverConf` (`modules/job/config/config.go:26-30`):

```go
Containers map[string]ContainerOverride `json:"containers,omitempty"`

type ContainerOverride struct {
    EnvVariables map[string]string `json:"env_variables,omitempty"`
}
```

Auto-populated by the existing `json.Unmarshal` in the `DriverFactory`
(`modules/job/module.go:58-64`) — no wiring change.

### Part B — unmarshal into a value, not a pointer-to-pointer

Unmarshal into a local **value**, not a pointer we then take the address of again. A zero-value
struct is always non-nil once we take its address, so the driver is well-formed even when `conf`
is empty/`null`.

```go
DriverFactory: func(conf json.RawMessage) (module.Driver, error) {
    var kd kubeDriver
    if len(conf) > 0 {
        if err := json.Unmarshal(conf, &kd); err != nil {
            return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal module config: %v", err)
        }
    } else {
        zap.L().Warn("kubernetes module has empty config; driver initialised with zero values")
    }
    return &kd, nil
},
```

Mirrored in `modules/flink/module.go` for `flinkDriver`. `zap.L()` is the existing global-logger
pattern used elsewhere in the repo (`core/write.go`, `core/sync.go`).

## Milestones

### M1 — Config struct + overlay *(Part A)*
- Add `ContainerOverride` type and `DriverConf.Containers` field (`modules/job/config/config.go`).
- In `ReadConfig`, inside the existing `for i := range cfg.Containers` loop
  (`modules/job/config/config.go:93-108`), after line 95:
  ```go
  if ov, ok := dc.Containers[c.Name]; ok && len(ov.EnvVariables) > 0 {
      c.EnvVariables = modules.CloneAndMergeMaps(c.EnvVariables, ov.EnvVariables)
  }
  ```
- **Acceptance:**
  - Overridden keys take the module value even when the client sends a real *or* masked value.
  - Non-overridden keys pass through unchanged; the client's own env vars survive.
  - Override entry for a container **absent** from the spec → ignored, no error.
  - Container with **no** override entry → untouched.
  - `containers` map empty/absent (today's default) → no-op, byte-identical to current behavior.
  - Container's `env_variables` nil → created and populated (guaranteed by `CloneAndMergeMaps`
    returning a fresh map).

### M2 — Tests *(Part A)*
- Extend `modules/job/driver/driver_test.go` (the module's only test). Current cases have no
  `containers` array, so env merging into containers is not yet asserted — add a case with a
  `containers` array in the spec **and** a populated `DriverConf` (replace the empty `driverConf()`
  helper at :211).
- Cases: module override wins over a real client value; over a masked-form client value
  (`****-abc12345`); non-overridden key passes through; override for a missing container is ignored;
  empty `containers` is a no-op.
- **Acceptance:** `go build ./...`, `go vet ./...`, and `go test ./modules/job/...` pass.

### M3 — Docs *(Part A)*
- Document the `containers.<name>.env_variables` module-config field in the job module docs under
  `docs/`, noting: module always wins, values must come from the secret manager via `${...}`
  (never hardcoded), and that it is independent of masking / `sensitive_configs`.
- **Acceptance:** docs describe the field, precedence, and the secret-manager requirement.

### M4 — Fix `kubernetes` module `DriverFactory` *(Part B)*
- Edit `modules/kubernetes/module.go` as above.
- **Acceptance:** driver constructed with `conf = nil`/`null`/`{}` returns non-nil, no error;
  `.Output(...)` no longer panics on nil-field access.

### M5 — Fix `flink` module `DriverFactory` *(Part B)*
- Same fix, mirrored in `modules/flink/module.go`.
- **Acceptance:** equivalent coverage for `flinkDriver`.

### M6 — Verify *(both parts)*
- `go build ./... && go vet ./...` pass.
- `go test ./modules/job/... ./modules/kubernetes/... ./modules/flink/...` all pass.

## Limitations / non-goals

### Part A
- **Job module only.** Firehose and other modules facing the same Dex-create problem are a noted
  follow-up (requirements §8), not in this change.
- No wholesale replacement of a container's `env_variables` map — this is a per-key overlay only.
- No validation beyond the existing `validator.TaggedStruct` on `DriverConf`; no schema check that
  `${...}` placeholders resolve.
- Not wired to masking's `Restore`/`sensitive_configs` in any way — deliberately decoupled.

### Part B
- Does not address `core/read.go`'s `GetResource` calling a dependency's `Output` unconditionally
  before checking that dependency's status — flagged as a follow-up, not fixed here.
- Does not add registration-time validation that exercises `Output`.
- Does not audit every module for the same pattern beyond `kubernetes`/`flink` (the two found via
  `grep -rn "Unmarshal(conf, &" modules/*/module.go`).

## Verification (end-to-end)

In `~/repo/goto/entropy`:

1. `go build ./... && go vet ./...`.
2. `go test ./modules/job/... ./modules/kubernetes/... ./modules/flink/...` — new + existing tests
   green.
3. *(Part A)* Create a job module with a `containers.<name>.env_variables` override; via the
   resource spec send that container's env with a masked/placeholder value; confirm the planned job
   spec (`getJob` output) carries the **module's** real value for that key and the client's other
   keys untouched.
4. *(Part A)* Confirm an existing job module with no `containers` block produces an unchanged job
   spec.
5. *(Part B)* Create a `job` resource whose `kube_cluster` dependency points at a kubernetes
   resource whose module registration `Configs` is `null`/empty — resolution completes instead of
   panicking.

## Production Validation

*Filled in only once the change is live. This is the evidence for `Status: Complete`; without it
the ADR stays `In Progress`.*

| Date | What was checked | Where observed | Result |
|------|------------------|----------------|--------|
| | | | |
