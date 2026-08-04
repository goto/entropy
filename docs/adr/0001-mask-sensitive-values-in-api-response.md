# ADR: Mask Sensitive Values in Entropy API Responses

**Complexity:** L
**Priority:** High
**Assignee:** TBD
**Created:** 2026-07-22
**Status:** In Progress
**MR:** [link once opened]

*(Legacy field retired 2026-08 — kept here for the historical record: Estimated Effort: 4–6 days — not part of the current ADR template.)*

## Implementation Progress (2026-07-24)

Landed in the `entropy` repo (`main`), unit tests green (`pkg/masking`, `core/module`; Docker-gated postgres/e2e suites not run locally):

- `pkg/masking/` — `Masker` (`Mask`, `Restore`, keyed HMAC-SHA256 `fingerprint`), dot/`*`-wildcard path walker, `ValidatePaths`, and request-scoped `Provider` with per-URN cache over a narrow `ModuleConfigLookup`.
- Resource read path — masking injected into `resources.APIServer` (`NewAPIServer(svc, masker, moduleConfig)`); `GetResource`/`ListResources`/`CreateResource`/`UpdateResource`/`ApplyAction`/`GetResourceRevisions` mask `spec.configs` + `state.output` via a source-preserving copy. Fail-open with warning; revisions resolve kind/project from the URN.
- Module read path — `modules.APIServer` masks `configs` in `GetModule`/`ListModules`, keeping `sensitive_configs` visible.
- Write path — `core.Service` gains `WithMasking(...)` option; `execAction` runs `Restore` before `Plan` so drivers see real secrets; masked-on-create rejected as `ErrInvalid`.
- Module write validation — `sensitive_configs` syntax validated in `CreateModule`/`UpdateModule`.
- Config/wiring — `MaskingConfig.HMACKey` (`masking.hmac_key`); empty ⇒ masking disabled (rollback toggle). Threaded through `cli/serve.go` → `server.Serve(..., maskKey)`.

Remaining: integration tests for the six endpoints + module endpoints (Docker), docs updates (`sensitive_configs`, masked format, round-trip/rotation), staging deploy with the HMAC key from the secret manager, manual verification.

## Simplification Pass (done, 2026-07-24)

The landed implementation above was more complex than necessary in two spots. Both were
simplified in the `entropy` repo (branch `mask-sensitive-values-api-response`), with
**no change to external behavior** (masked format, HMAC change-detection, path syntax,
and endpoint coverage are all unchanged). `go build ./...`, `go vet ./...`, and all
non-Docker-gated tests pass (`pkg/masking`, `core`, `core/module`,
`internal/server/v1/resources`, `internal/server/v1/modules`); Docker-gated
postgres/e2e suites still require a local Docker daemon and were not run.

- **Config lookup:** `pkg/masking.Provider` (request-scoped, per-URN cache) replaced by
  `pkg/masking.ConfigCache` (`NewConfigCache`), a process-level cache
  (`map[urn][]string`, `sync.RWMutex`-guarded, no TTL) owned independently of the
  `Masker`. No cache object is constructed or threaded through any mapper call —
  `resources.APIServer` and `core.Service` each hold a `*masking.ConfigCache` field and
  call `PathsFor` directly. `core/module.Service` gained `SetMaskingCache` and calls
  `configCache.Evict(mod.Name, mod.Project)` after a successful `CreateModule`/
  `UpdateModule`, so reads reflect a changed `sensitive_configs` immediately. The
  duplicate `moduleConfigLookup` adapter (previously defined separately in
  `internal/server/server.go` and `cli/serve.go`) collapsed to a single instance
  constructed once in `cli/serve.go`, shared by the read path (`entropyserver.Serve`),
  the write path (`core.WithMasking`), and eviction (`module.Service`).
- **Write merge:** `Masker.Restore` (`pkg/masking/restore.go`) now applies one uniform
  rule with no error branch: masked-form incoming value + a stored value exists →
  restore stored; masked-form + nothing stored (Create, or a new field) → **drop the
  field**; real value → keep as-is. `ErrMaskedWithoutStored` and the `Create`-specific
  `ErrInvalid` rejection in `core/write.go` were removed — `restoreSensitive` now has a
  single code path for Create and Update.

### Files changed
- `pkg/masking/masking.go` — removed `ErrMaskedWithoutStored`.
- `pkg/masking/restore.go` — `restorePath`/`restoreLeaf` drop instead of erroring; `Restore` no longer returns a merge-specific error.
- `pkg/masking/provider.go` → renamed `pkg/masking/cache.go` — `Provider`/`NewProvider` replaced by `ConfigCache`/`NewConfigCache`/`Evict`.
- `pkg/masking/provider_test.go` → renamed `pkg/masking/cache_test.go`; added `TestConfigCache_EvictForcesReResolve`.
- `pkg/masking/masking_test.go` — `TestRestore_CreateRejectsMaskedInput` → `TestRestore_CreateDropsMaskedInput`.
- `core/core.go` — `Service.moduleConfig` → `Service.configCache *masking.ConfigCache`; `WithMasking` signature updated.
- `core/write.go` — `restoreSensitive` uses `svc.configCache.PathsFor` directly (no per-call `Provider`); removed the `ErrMaskedWithoutStored`/`ErrInvalid` branch.
- `core/module/service.go` — added `configCache` field, `SetMaskingCache`, and `Evict` calls in `CreateModule`/`UpdateModule`.
- `internal/server/v1/resources/server.go`, `masking.go` — `APIServer.moduleConfig` → `APIServer.configCache`; removed `newMaskProvider`; `maskResource`/`maskRevision` no longer take a provider parameter; all six call sites in `server.go` simplified accordingly.
- `internal/server/server.go` — `Serve` now takes `masker *masking.Masker, configCache *masking.ConfigCache` instead of `maskKey []byte`; removed the now-unused `moduleConfigLookup` adapter (and its `encoding/json` import) from this file.
- `cli/serve.go` — constructs `masker`/`configCache` once, calls `moduleService.SetMaskingCache(configCache)`, passes both into `core.WithMasking` and `entropyserver.Serve`.

See decisions #8 and #10, and Core Tasks 2 and 5 below, which now reflect the simplified
design. Design decisions #1–7, #9, #11–13 and read-path behavior are unchanged.

## Scope Refinement (done, 2026-07-27)

Two scope narrowings in the `entropy` repo, no change to the write path, `ConfigCache`, or wiring:

- **`state.output` masking removed everywhere.** `maskResource` (`internal/server/v1/resources/masking.go`) no longer masks `res.State.Output` — only `res.Spec.Configs`. Sensitive paths (`sensitive_configs`) target `spec.configs` (e.g. `env_variables.*`), so masking `state.output` was unnecessary. Applies to all resource read endpoints (`GetResource`/`CreateResource`/`UpdateResource`/`ApplyAction`/`ListResources`). `maskRevision` already masked `spec.configs` only.
- **`ListResources` masking gated on `with_spec_configs`.** In `ListResources` (`internal/server/v1/resources/server.go`), `maskResource` is called only when `withSpecConfigs=true`. When false, the store does not hydrate `spec.configs`, so there is nothing to mask; skipping avoids a per-resource `ConfigCache.PathsFor` lookup. Other endpoints always return `spec.configs`, so they mask unconditionally.

Tests: `TestAPIServer_ListResources_MaskingGate` asserts spec.configs is masked with the flag and returned as-is without it. `go build`/`go vet` and `internal/server/v1/...`, `pkg/masking`, `core/...` tests pass (Docker-gated suites not run).

### Files changed
- `internal/server/v1/resources/masking.go` — `maskResource` drops the `state.output` mask line; doc comment updated.
- `internal/server/v1/resources/server.go` — `ListResources` gates `maskResource` on `withSpecConfigs`.
- `internal/server/v1/resources/server_test.go` — added `TestAPIServer_ListResources_MaskingGate`.

## Overview

**What:** Mask configured sensitive values in every Entropy resource and module API response so that any sensitive information stored in Entropy cannot leave the system in cleartext, while still letting consumers detect when a specific credential has changed.
**Why:** Resource `spec.configs` and module `configs` currently expose secrets (Kafka passwords, GCS/BigQuery credentials, tokens, etc.) verbatim in API response bodies, which is a security exposure.
**Success Metric:** No configured sensitive value appears in cleartext in `spec.configs`/module `configs` across the response paths (`ListResources` masked only when `with_spec_configs=true`); the UI round-trip (Get → edit unrelated field → Update) never clobbers a real secret; consumers can tell when a specific credential value changed.

## Requirements

### User Story
**As a** platform operator / security owner
**I want** sensitive values in resource and module configs/outputs to be masked in all API responses, configurable per resource kind
**So that** secrets stored in Entropy are never exposed through the API, without breaking existing read/update workflows.

### Acceptance Criteria
- [ ] Sensitive values are masked in the response bodies of: `GetResource`, `ListResources`, `CreateResource`, `UpdateResource`, `ApplyAction`, and `GetResourceRevisions`.
- [ ] Sensitive values are masked in the response bodies of module endpoints: `GetModule` and `ListModules`.
- [ ] Masking applies to `resource.spec.configs` and `module.configs`. `resource.state.output` and `state.module_data` are **out of scope**.
- [ ] On `ListResources`, masking runs only when `with_spec_configs=true` (spec.configs is not hydrated otherwise).
- [ ] Sensitive variables are configured per **resource kind** (module-level policy), via a `sensitive_configs` list in the module's `configs`.
- [ ] All read APIs (including `Get` and `List`) return masked data based on the **latest** module `sensitive_configs`.
- [ ] A masked value is rendered as `****-<8-hex-hmac>` where the fingerprint is a truncated **keyed HMAC** of the real value, so the same secret always yields the same fingerprint and a changed secret yields a different one.
- [ ] The HMAC key is loaded from an environment variable / secret manager and is never hardcoded.
- [ ] On write (`Create`/`Update`/`ApplyAction`), a sensitive field whose incoming value is in masked form (`****-<fp>`) is **not** persisted; the currently-stored value is restored instead. A real (non-masked) incoming value is persisted as the new secret.
- [ ] Masking never mutates the domain objects used internally for reconciliation (drivers still receive real secrets during `Plan`/`Sync`).
- [ ] The `sensitive_configs` list itself is returned unmasked (it is field-path metadata, not a secret).

### Constraints
- **Technical:**
  - Masking must run at the response boundary only; internal reconciliation (`Plan`/`Sync`/`Output`) must keep operating on real values.
  - `ListResources` and `GetResourceRevisions` do **not** currently load a module, so the masker must resolve `sensitive_configs` on demand.
  - Existing module `driverConf` structs must not need per-module edits.
- **Business:** Sensitive-config policy is defined once per kind and expected to be consistent across a kind's per-project module instances.
- **Timeline:** None specified.

## Design Decisions (resolved during grill-me)

1. **Masking scope:** `resource.spec.configs` and `module.configs`. `resource.state.output` and `state.module_data` excluded (see Scope Refinement 2026-07-27 for why `state.output` was dropped).
2. **Sensitive-variable identification:** explicit **JSON paths** (not key-name matching).
3. **Path list model:** a **single** list of paths, applied against both payload roots (`spec.configs`, module `configs`). Over-matching is the safe direction.
4. **Config storage:** the list lives in the **module `configs`** (`mod.Configs`, i.e. `driverConf` JSON), per `(kind, project)`, treated as a kind-level policy. Chosen because the requirement says "defined as a module config" and it stays operator-editable without a deploy.
5. **Read mechanism:** a **generic well-known key** `sensitive_configs` parsed directly from the raw `mod.Configs` JSON by the masking layer. No per-module `driverConf` struct changes — modules `json.Unmarshal` into their own structs and silently ignore the extra key.
6. **Masked value format:** `****-<8-hex-hmac>`. HMAC key loaded from env/secret manager.
7. **Change detection:** truncated **keyed HMAC-SHA256** fingerprint (`-` delimiter). Deterministic per value; brute-force resistant because the key is server-only.
8. **Write round-trip (simplified):** `Masker.Restore(incoming, stored, paths)` called once at **one choke point** in `core/write.go` (`restoreSensitive`, before `Plan`), with one uniform rule per sensitive path: masked-form incoming **and** stored exists → restore stored; masked-form **and** no stored (Create / new field) → **drop the field**; real value → persist as-is. **Ignore-and-restore**: the incoming fingerprint is NOT validated for staleness (round-trip can never clobber). No per-endpoint merge logic and no `Create`-specific `ErrInvalid`/`ErrMaskedWithoutStored` branch — the drop-on-create case subsumes it.
9. **Placement:** the **proto-mapping boundary** — `resourceToProto`, `revisionToProto`, `moduleToProto` — operating while building the protobuf, never mutating source domain structs.
10. **Config lookup (simplified):** `pkg/masking.ConfigCache` is a standalone **process-level cache** (`map[urn][]string`, `sync.RWMutex`-guarded, no TTL), held alongside (not inside) the `Masker` by both `resources.APIServer` and `core.Service`, replacing the request-scoped `Provider` threaded through every mapper call. On a miss, `GetModule` (via `ModuleConfigLookup`) populates the entry. The entry is **evicted on module `Create`/`Update`** (`configCache.Evict(kind, project)` called from `core/module.Service`, wired via `SetMaskingCache`), so reads always reflect the latest `sensitive_configs` without staleness or per-request plumbing. On missing module (orphaned resource), **fail-open**: return unmasked and log a warning.
11. **Path syntax:** dot-notation, including into nested objects/maps (`env_variables.SOURCE_KAFKA_PASSWORD`, `telegraf.config.output.password`), plus a trailing `*` wildcard for "all keys under this node" (`env_variables.*`). **No array indexing** in v1. Paths that don't resolve in a given payload are skipped silently.
12. **Scope boundaries:** `GetLog`/log streaming **out of scope** (flagged risk); **no at-rest encryption** (flagged follow-up); dry-run responses **are** masked (same mapper path); CLI **inherits** masking (no separate work, verify only).
13. **Validation:** `sensitive_configs` path **syntax** is validated at module `Create`/`Update` (well-formed dot/wildcard paths, no empty segments); paths are **not** required to resolve against any current config.

## How It Works (end-to-end)

### Configuration
A module is created/updated with a `sensitive_configs` list inside its `configs`:

```json
{
  "name": "firehose",
  "project": "my-project",
  "configs": {
    "namespace": { "default": "firehose" },
    "chart_values": { "image_tag": "latest" },
    "sensitive_configs": [
      "env_variables.SOURCE_KAFKA_PASSWORD",
      "env_variables.*",
      "gcs_sink_credential"
    ]
  }
}
```

### Read path (mask)
1. A resource/module response is built in the proto mapper.
2. The masker resolves the `sensitive_configs` list for the resource's `(kind, project)` (module URN `orn:entropy:module:{project}:{kind}`), using the process-level `ConfigCache`. For module responses, it uses the module's own `sensitive_configs`. On `ListResources`, this step runs only when `with_spec_configs=true`.
3. For each path, resolved against each in-scope payload (`spec.configs`, module `configs`), the value is replaced with `****-<8-hex-hmac>` where `hmac = HMAC-SHA256(maskKey, canonicalBytesOfValue)` truncated to 8 hex chars.
4. If the module can't be found, the payload is returned unmasked and a warning is logged.

### Write path (merge)
1. On `Create`/`Update`/`ApplyAction`, the incoming `spec.configs` is scanned at the sensitive paths.
2. For each sensitive path whose incoming value is in masked form (`****-` prefix): restore the value currently stored for that path (from the loaded existing resource). The incoming fingerprint is ignored.
3. For each sensitive path whose incoming value is NOT masked: persist it as-is (new/rotated secret).
4. On `Create` there is no stored value to restore; a masked-form value is rejected as invalid.

### Masked value shape
```
"SOURCE_KAFKA_PASSWORD": "hunter2"          // stored (DB, cleartext)
"SOURCE_KAFKA_PASSWORD": "****-a1b2c3d4"     // in API response
```
Rotating the secret to `s3cr3t!` changes the response to `****-9f8e7d6c`, so a consumer can detect that this specific credential changed while all other fingerprints stay stable.

## Implementation Plan

### Prerequisites
- [ ] Provision an HMAC key and a secure delivery mechanism (env var, e.g. `ENTROPY_MASK_HMAC_KEY`, sourced from the secret manager). Wire it into server config.
- [ ] Confirm the `sensitive_configs` extra key is tolerated by every module's `DriverFactory` unmarshal (all inspected modules use non-strict `json.Unmarshal`; verify no module sets `DisallowUnknownFields`).

### Core Tasks

1. **Masking package (`pkg/masking` or `internal/masking`)** — Core masker.
   - **Estimate:** 1 day
   - **Details:**
     - `Masker` holds the HMAC key.
     - `Mask(payload json.RawMessage, paths []string) (json.RawMessage, error)`: parse to a generic tree, walk each path (dot-notation + trailing `*` wildcard, no array indexing), replace matched leaves with `****-<fp>`, re-marshal. Unresolved paths skipped silently.
     - `fingerprint(value any) string`: canonicalize the value to bytes, `HMAC-SHA256(key, bytes)`, hex-encode, truncate to 8 chars.
     - `Restore(incoming, stored json.RawMessage, paths []string)`: for each path, if incoming leaf is masked-form (`****-` prefix) replace it with the stored leaf; else keep incoming. Missing stored leaf on a masked input → treat as invalid (surface error for `Create`).
     - Path parser + syntax validator `ValidatePaths([]string) error` (well-formed segments, no empties, at most one trailing `*`).
     - `// TODO: Securely load this value from an environment variable or secrets vault. Do not hardcode.` at the key-loading site.

2. **Sensitive-config cache (`pkg/masking.ConfigCache`)** — Resolve `sensitive_configs` per `(kind, project)`.
   - **Estimate:** 0.5 day
   - **Details:**
     - No provider interface. `ConfigCache` is a standalone type (constructed via `NewConfigCache(lookup)`) holding a concurrency-safe process-level cache: `map[string][]string` keyed by module URN, guarded by `sync.RWMutex`. `resources.APIServer` and `core.Service` each hold a `*ConfigCache` field.
     - `PathsFor(ctx, kind, project) ([]string, error)`: read-lock hit → return; miss → `ModuleConfigLookup.ModuleConfigs` (backed by `module.Service.GetModule`), parse only the `sensitive_configs` key from `mod.Configs`, store, return.
     - **Eviction, not per-request caching:** `configCache.Evict(kind, project)` is called from `core/module.Service.CreateModule`/`UpdateModule` after a successful write, via a `SetMaskingCache` setter that wires the shared cache in post-construction. No cache object is constructed or threaded through mappers/`ListResources`.
     - Missing module → lookup error propagates (not cached); caller fails open + logs warning.

3. **Wire masking into resource proto mappers** — `internal/server/v1/resources`.
   - **Estimate:** 1 day
   - **Details:**
     - Inject masker + config cache into `resources.APIServer` (extend `NewAPIServer` in `internal/server/v1/resources/server.go:33` and its call site `internal/server/server.go:77`).
     - Apply masking inside/after `resourceToProto` and `revisionToProto` (`internal/server/v1/resources/mappers.go`) for `spec.configs` and `state.output`. Operate on the proto/`json.RawMessage`, not the source `resource.Resource`.
     - `ListResources`/`GetResourceRevisions` call `server.configCache.PathsFor` directly per resource; no per-request cache object needed since the cache is process-level.
     - `GetResourceRevisions`: revisions carry `spec.configs` only; resolve the kind/project from the revision URN.

4. **Wire masking into module proto mapper** — `internal/server/v1/modules`.
   - **Estimate:** 0.5 day
   - **Details:**
     - Inject masker into `modules.APIServer` (`internal/server/v1/modules/server.go:28`, call site `internal/server/server.go:85`).
     - In `moduleToProto` (`internal/server/v1/modules/mappers.go`), read the module's own `sensitive_configs` and mask its own `configs`. Keep the `sensitive_configs` list itself unmasked.

5. **Write-path merge (restore stored secrets)** — resource write flow.
   - **Estimate:** 0.5 day
   - **Details:**
     - `Masker.Restore(incoming, stored json.RawMessage, paths []string) (json.RawMessage, error)` (stored may be nil), one uniform rule: masked-form + stored exists → restore stored; masked-form + no stored → drop the field; real value → keep. Error return is now only for JSON parse failures.
     - Called **once**, at a single choke point in the resource write path (`core/write.go` — `restoreSensitive`, shared by `CreateResource`/`UpdateResource`/`ApplyAction` via `execAction`), before `Plan`.
     - `Update`/`ApplyAction` already load the existing resource; pass its stored `spec.configs`. `Create` passes `stored = nil`, so any masked-form field is simply dropped — no separate `ErrInvalid`/`ErrMaskedWithoutStored` branch or per-endpoint logic.
     - Ensure the merge happens before `Plan`, so drivers see the real (restored) secret.

6. **Module write validation** — validate `sensitive_configs` syntax.
   - **Estimate:** 0.5 day
   - **Details:**
     - In `module.Service.CreateModule`/`UpdateModule` (`core/module/service.go`), parse `sensitive_configs` and call `ValidatePaths`; reject malformed entries with `ErrInvalid` and a clear message. Do not require resolution.

### Testing Tasks
- [ ] **Unit Tests:**
  - Masker: dot paths, nested paths, `*` wildcard over maps, unresolved paths skipped, non-string leaf types, deterministic fingerprint, fingerprint changes when value changes, fingerprint stable across runs with same key.
  - Restore: masked input restores stored; real input persists; `Create` rejects masked input; mixed fields.
  - Path validator: valid/invalid syntaxes.
  - Provider: per-request caching/dedup; missing-module fail-open.
- [ ] **Integration Tests:**
  - All six resource endpoints return masked `spec.configs`/`state.output`.
  - `GetModule`/`ListModules` return masked `configs` with visible `sensitive_configs`.
  - Round-trip: `Get` → resend whole config on `Update` → stored secret intact.
  - Rotation: send new real value → persisted, fingerprint changes on next read.
- [ ] **Manual Testing:**
  - Firehose module with `env_variables.*` masks all env vars; UI edit of `replicas` preserves secrets.
  - Orphaned-resource (no module) read returns unmasked + logs warning.

### Documentation Tasks
- [ ] **Code Comments:** masker path grammar; HMAC-key sourcing TODO; why masking is boundary-only.
- [ ] **README/Docs Updates:** document `sensitive_configs` in module config docs (`docs/`), masked value format, and the round-trip/rotation behavior.
- [ ] **API Docs:** note masked-field semantics on the six resource endpoints and module endpoints.

## Technical Notes

### Files to Modify
- `internal/server/v1/resources/mappers.go` — mask in `resourceToProto`, `revisionToProto`.
- `internal/server/v1/resources/server.go` — inject masker + provider into `APIServer` (`NewAPIServer`, line 33).
- `internal/server/v1/modules/mappers.go` — mask module `configs` in `moduleToProto`.
- `internal/server/v1/modules/server.go` — inject masker into `APIServer` (`NewAPIServer`, line 28).
- `internal/server/server.go` — construct masker (with HMAC key) + provider; pass to both API servers (lines ~77, ~85).
- `core/write.go` — restore stored sensitive values before persist/plan.
- `core/module/service.go` — validate `sensitive_configs` syntax on module create/update.
- `pkg/masking/` (new) — masker, fingerprint, path parser/validator, provider interface.
- Server config struct + env loading — add `ENTROPY_MASK_HMAC_KEY`.

### Files/Behavior NOT changed
- Module `driverConf` structs (firehose/dagger/kafka/job/etc.) — untouched; `sensitive_configs` read generically.
- Storage layer (`internal/store/postgres`) — secrets remain stored as-is (cleartext); no at-rest change.
- Driver `Plan`/`Sync`/`Output` — keep receiving real values.

### Dependencies
- **Internal:** `core/module.Service.GetModule`; resource/module proto mappers; server wiring.
- **External:** Go stdlib `crypto/hmac`, `crypto/sha256`, `encoding/json`, `encoding/hex`. Secret manager for the HMAC key.

### Risks & Mitigation
- **Risk:** Client resends masked value on `Update` and clobbers the real secret.
  **Mitigation:** fingerprint-aware merge restores stored value for masked-form inputs; drivers see restored value.
- **Risk:** `List` performance from per-resource module lookups.
  **Mitigation:** masker-owned process-level cache keyed by `(kind, project)`, evicted on module write — repeat lookups across requests are free, not just within one `List`.
- **Risk:** Low-entropy secret fingerprint brute-forced from responses.
  **Mitigation:** keyed HMAC (server-only key) + 8-char truncation; no bare hashes.
- **Risk:** HMAC key rotation invalidates all fingerprints (all appear "changed" once).
  **Mitigation:** document that rotating the key resets change-detection baselines; treat as rare, planned.
- **Risk:** Orphaned resource (no module) leaks unmasked (fail-open).
  **Mitigation:** logged warning; such resources are already un-reconcilable; revisit if it proves material.
- **Risk (out of scope, flagged):** `GetLog` can stream secrets in cleartext.
  **Mitigation:** explicitly out of scope; tracked as follow-up.
- **Risk (out of scope, flagged):** secrets remain cleartext at rest in Postgres.
  **Mitigation:** explicitly out of scope; tracked as a separate hardening item.
- **Risk:** a module sets `DisallowUnknownFields` and rejects `sensitive_configs`.
  **Mitigation:** verified inspected modules use non-strict unmarshal; add a check during implementation.

## Definition of Done

- [ ] All acceptance criteria are met.
- [ ] Code is reviewed and approved.
- [ ] Unit + integration tests written and passing.
- [ ] Docs updated (`sensitive_configs`, masked format, round-trip/rotation).
- [ ] Deployed to staging with `ENTROPY_MASK_HMAC_KEY` sourced from the secret manager.
- [ ] Manual verification of the six resource endpoints, module endpoints, round-trip, and rotation.
- [ ] Security/stakeholder sign-off.

## Rollback Plan

**If something goes wrong:**
1. Masking is additive at the response boundary and merge-on-write; disable via a feature flag / config toggle (recommend gating the masker behind a config switch) to revert to pass-through behavior without redeploy.
2. If not flag-gated, revert the masking + merge changes; stored data is unaffected (secrets were never re-encrypted or moved), so no data migration is needed to roll back.
3. Communicate to API/UI consumers that responses temporarily return cleartext again and that `sensitive_configs` remains stored but inert.
