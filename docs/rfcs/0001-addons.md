# RFC 0001: Add-ons

Status: Draft (revision 3 — implementation-ready, expanded samples)

## Context

Entropy orchestrates resources (firehose, dagger, kafka, flink, ...). Each resource usually needs supporting resources: log shipping, alerts, Kafka ACLs. Today these are embedded in each module's config and driver:

- Kafka ACL: firehose (`ACL`, `DLQACL`, `StreamSecurity`, see `modules/firehose/config.go`) and dagger (`ACLMounts`, see `modules/dagger/config.go`) each recompute it on every Plan.
- Monitoring: firehose `Telegraf` and the KEDA autoscaler; flink and dagger carry a `prometheus_url`. There is no alert-rule construct.
- Logs: only a streaming `Log()` method on drivers (`core/read.go`). There is no log-shipping config.

Consequences: the same concern is re-implemented per module, cannot be managed without updating the parent, and the core has no **lifecycle** hook for post-commit side effects. `resource.MutationHook` exists on the store interface and runs **inside the resource DB transaction** (`internal/store/postgres/resource_store.go`); callers in `core/write.go` do not pass hooks today. Add-ons need **post-commit** external I/O, not `MutationHook`.

This RFC introduces **add-ons**: optional supporting resources of a parent resource, managed independently via API or reconciled during the parent's lifecycle.

## Goals

- A generic add-on framework usable by any module that opts in.
- One reference add-on, **logs** (log-shipping configuration via an external logging API).
- Add-ons manageable independently through a nested API, and attachable declaratively from the parent spec or module defaults.
- No behaviour change for existing resources.

## Non-goals

- Migrating existing Telegraf, ACL or autoscaler config (follow-up RFCs: Kafka ACL, alerts).
- A **background retry loop** or queued reconciliation for add-ons (timeouts and manual re-apply only in v1). Add-on **Apply** is not coupled to the resource sync loop; see Reconciliation triggers.
- Cascading external deletes when a parent is soft-deleted.
- A dedicated add-on permission model (reuse existing project/resource authorization; see API).

## Design

### Identity and storage

Add-ons live in a separate generic `addons` table, loosely coupled to `resources` (no foreign key), since not all modules support the same add-ons. Every row stores `parent_urn` (full resource URN). Apply and list operations validate that the parent exists, belongs to the same project as the nested API path, and is not hard-deleted.

| Column | Notes |
| ------ | ----- |
| `urn` | `orn:entropy:addon:<parent-kind>:<project>:<parent-name>:<type>` — mirrors parent identity from `resource.GenerateURN` plus add-on type; unique and unambiguous across kinds |
| `parent_urn`, `type` | unique `(parent_urn, type)`: one add-on per parent and type |
| `spec` | JSON, validated by the add-on type |
| `output` | JSON, last successful `Apply` result (opaque per type); returned on GET |
| `status`, `last_error` | `COMPLETED` or `ERROR`, set inline by the synchronous call |
| `origin` | `spec` (declared in parent spec / module defaults) or `api` |
| `spec_version` | Monotonic integer incremented on each successful spec persist; used for optimistic concurrency on API PUT |
| `created_at/by`, `updated_at/by`, `deleted_at/by` | `deleted_at` is a tombstone, as for resources |

Index: `(parent_urn)` for list-by-parent. v1 does not support project-wide add-on listing.

**Parent missing or wrong project:** validation on create/reconcile returns invalid argument. Rows are not created for a non-existent parent.

#### Database schema (DDL)

v1 ships the tables below in [`internal/store/postgres/schema.sql`](../../internal/store/postgres/schema.sql). Add-ons are **not** foreign-keyed to `resources` so rows can exist across module kinds without schema churn; integrity is enforced in application code.

```sql
-- Per-parent add-on instance (one row per parent_urn + type)
CREATE TABLE IF NOT EXISTS addons
(
    urn               TEXT        NOT NULL PRIMARY KEY,
    parent_urn        TEXT        NOT NULL,
    type              TEXT        NOT NULL,
    spec              bytea       NOT NULL,
    output            bytea       NOT NULL DEFAULT '{}',
    status            TEXT        NOT NULL,
    last_error        TEXT        NOT NULL DEFAULT '',
    origin            TEXT        NOT NULL,          -- 'spec' | 'api'
    spec_version      BIGINT      NOT NULL DEFAULT 1,
    last_applied_spec bytea,                          -- JSON last successfully applied spec (optional)
    created_at        timestamptz NOT NULL DEFAULT current_timestamp,
    updated_at        timestamptz NOT NULL DEFAULT current_timestamp,
    created_by        TEXT        NOT NULL DEFAULT '<unknown>',
    updated_by        TEXT        NOT NULL DEFAULT '<unknown>',
    deleted_at        timestamptz,
    deleted_by        TEXT,
    UNIQUE (parent_urn, type)
);
CREATE INDEX IF NOT EXISTS idx_addons_parent_urn ON addons (parent_urn);

-- Project-level backend credentials (one row per project + type)
CREATE TABLE IF NOT EXISTS addon_configs
(
    project     TEXT        NOT NULL,
    type        TEXT        NOT NULL,
    configs     bytea       NOT NULL,               -- JSON: endpoint, token, ...
    created_at  timestamptz NOT NULL DEFAULT current_timestamp,
    updated_at  timestamptz NOT NULL DEFAULT current_timestamp,
    PRIMARY KEY (project, type)
);
```

**Example `addons` row** (after parent reconcile; external apply not run yet):

| Column | Example value |
| ------ | ------------- |
| `urn` | `orn:entropy:addon:firehose:team-a:orders-stream:logs` |
| `parent_urn` | `orn:entropy:firehose:team-a:orders-stream` |
| `type` | `logs` |
| `spec` | `{"enabled":true,"on_failure":"continue","labels":{"app":"orders"}}` |
| `output` | `{}` until `PUT .../addons/logs` runs external registration |
| `status` | `COMPLETED` after reconcile (row persisted); becomes `ERROR` if an API `Apply` fails |
| `origin` | `spec` |
| `spec_version` | `1` |

Parent resource `spec_configs` still contains the same desired block under `addons` (see [Sample payloads](#sample-payloads)); the `addons` table is the normalized runtime record used by the nested API.

**Planned v2 — `addon_revisions` (not in v1 migrate):**

```sql
CREATE TABLE IF NOT EXISTS addon_revisions
(
    id                 BIGSERIAL   NOT NULL PRIMARY KEY,
    parent_urn         TEXT        NOT NULL,
    addon_type         TEXT        NOT NULL,
    addon_urn          TEXT        NOT NULL,
    reason             TEXT        NOT NULL DEFAULT '<unknown>',
    trigger            TEXT        NOT NULL,           -- write_reconcile | api | apply | delete
    parent_revision_id BIGINT,                        -- optional link to revisions.id
    spec               bytea       NOT NULL,
    output             bytea       NOT NULL DEFAULT '{}',
    status             TEXT        NOT NULL,
    last_error         TEXT        NOT NULL DEFAULT '',
    origin             TEXT        NOT NULL,
    spec_version       BIGINT      NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT current_timestamp,
    created_by         TEXT        NOT NULL DEFAULT '<unknown>'
);
CREATE INDEX IF NOT EXISTS idx_addon_revisions_parent_urn ON addon_revisions (parent_urn, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_addon_revisions_addon_urn ON addon_revisions (addon_urn, created_at DESC);
```

Write rules for v2 are in [Revision history](#revision-history).

**Chosen (D3):** URN embeds parent **kind** and name, not name alone, so two resources with the same name under different kinds do not collide.

### Capability declaration

The parent module's `Descriptor` gains `Addons []string` (similar to `Descriptor.Dependencies`, `core/module/module.go`), for example `firehose` declares `["logs"]`. A parent spec or default naming an unsupported type is rejected at validation.

Validation ownership:

- **Allow-list:** core checks type against module `Descriptor.Addons` at parent spec validate time.
- **Shape:** each add-on type registers a JSON schema (or Go `Validate`) for merged spec after defaults; proton carries nested `addons` as `Struct` in parent spec messages.

### Add-on contract

Separate from the module `Driver`, registered by type in an add-on registry:

```go
type Addon interface {
    Validate(ctx context.Context, spec json.RawMessage) error
    Apply(ctx context.Context, parent resource.Resource, spec json.RawMessage) (json.RawMessage, error)
    Delete(ctx context.Context, parent resource.Resource, spec json.RawMessage) error
}
```

- **Output:** `Apply` returns JSON stored in `output` and exposed on GET. Empty object if the type has nothing to return.
- **Idempotency:** `Apply` and `Delete` must be safe to repeat for the same `(parent_urn, type)` and effective spec. Implementations should use stable external keys derived from `parent_urn` (and type-specific fields in spec), not request ids.
- **Timeouts:** server default context timeout with per-type override. Timeout counts as failure.
- **Parent load:** normal operations use `GetByURN` (hides tombstoned parents). `Delete` on an add-on whose parent is soft-deleted uses `getByURNIncludingDeleted` (`internal/store/postgres/resource_store.go`) so external teardown can still run.

### Declaration and precedence

- Module row: `addon_defaults` in `configs` states which add-ons attach to each parent of that kind.
- Parent spec: `addons: { <type>: {...} }`.
- Merge is per add-on: a spec entry replaces the module default wholesale. `enabled: false` opts out of a default and tombstones any existing `spec`-origin row for that type on reconcile.

```yaml
# module (project=team-a, name=firehose-default)
project: team-a
name: firehose-default
configs:
  addon_defaults:
    logs: { enabled: true, on_failure: continue }
---
# parent spec (kind=firehose)
spec:
  addons:
    logs: { enabled: false }
```

Module `addon_defaults` follow the same sensitive-config masking and restore rules as module configs when paths are registered for the module kind.

### Source of truth

Each row records `origin`.

- A parent update reconciles only `spec`-origin add-ons (create/update/tombstone rows and persisted spec; external `Apply` follows Reconciliation triggers).
- A direct PUT on a `spec`-origin add-on is rejected with a message to change the parent spec.
- Removing an entry from the spec tombstones the add-on row on reconcile.
- `api`-origin add-ons are never touched by parent spec reconciliation.

**Concurrency:** API PUT includes optional `spec_version`. If it does not match the stored `spec_version`, the server returns conflict so an in-flight PUT does not silently lose to a parent update reconcile. Parent reconcile does not bump `spec_version` on `api`-origin rows.

### Reconciliation triggers

Hook logic calls the same apply/delete paths as the HTTP API. Add-ons do **not** call external systems on parent **dry-run** (`WithDryRun`).

Parent writes (`execAction` in `core/write.go`) plan and upsert the parent, usually leaving it `STATUS_PENDING`. Resolved `State.Output` is produced in `handleSync` → `SyncState` (`core/sync.go`); the sync loop does **not** invoke add-on logic.

**Chosen (D1):** split reconcile (persist desired add-on rows from merged defaults/spec) from external **Apply**. External **Apply** is only triggered by the add-on API (or a future optional worker), not by resource sync.

| Trigger | Code path | Parent state | Add-on action |
| ------- | --------- | ------------ | ------------- |
| Parent create/update (not dry-run) | After successful parent `upsert` in `execAction` | Any | Reconcile `spec`-origin rows (spec, tombstone removed types); **no** external `Apply` |
| Parent `PUT .../addons/{type}` | Add-on handler | `STATUS_COMPLETED` required for types that need output (e.g. logs); otherwise invalid with clear error | Validate, persist, `Apply` |
| Parent soft-delete | `SoftDeleteResource` | Tombstoned parent | Tombstone all add-on rows for `parent_urn`; **no** external `Delete` |
| Add-on `DELETE` | Add-on handler | Parent may be soft-deleted (tombstone-inclusive get) | External `Delete`, then tombstone add-on row |
| Force delete parent | `DeleteResource` with force | Hard-deleted parent | Tombstone or delete add-on rows in DB; **no** mandatory external `Delete` in v1 |

If the parent stays `STATUS_ERROR` or `STATUS_PENDING`, spec-origin rows may exist but external `Apply` only runs when an operator calls the add-on API (after the parent is `STATUS_COMPLETED` when output is required).

**Failure:** each add-on spec may set `on_failure: fail | continue` (default `continue`). `fail` returns an error to the caller for that trigger, leaves the parent saved, and marks the add-on `ERROR`. `continue` lets the parent operation succeed and records `ERROR` on the add-on. Recovery: idempotent manual PUT on the add-on API.

There is no rollback of the parent after external `Apply`, since external changes cannot be reliably undone.

### Parent and add-on deletion

Add-ons never cascade external teardown on parent delete.

**Chosen (D2):**

| API entrypoint | Parent | Add-on DB | External |
| -------------- | ------ | --------- | -------- |
| DELETE resource (default) | Soft-delete | Tombstone all add-ons | None — operators must DELETE add-ons **before** soft-delete if external cleanup is required |
| DELETE resource (force) | Hard delete | Tombstone/delete add-on rows | None in v1 (document leak risk) |
| DELETE `.../addons/{type}` | — | Tombstone after `Addon.Delete` | `Addon.Delete` |

On soft-deleted parent: GET list/get add-ons remains allowed for ops; DELETE add-on remains allowed for external teardown using tombstone-inclusive parent load.

### API

Nested under the parent; authorization matches the parent resource (same project and resource-level checks as parent GET/PUT).

| Method | Path | Notes |
| ------ | ---- | ----- |
| PUT | `/v1beta1/resources/{urn}/addons/{type}` | Apply; optional `spec_version` |
| GET | `/v1beta1/resources/{urn}/addons/{type}` | Read |
| GET | `/v1beta1/resources/{urn}/addons` | List |
| DELETE | `/v1beta1/resources/{urn}/addons/{type}` | External delete + tombstone |

**addon-config** (project-level backend credentials):

| Method | Path | Notes |
| ------ | ---- | ----- |
| PUT | `/v1beta1/projects/{project}/addon-configs/{type}` | Create/update |
| GET | `/v1beta1/projects/{project}/addon-configs/{type}` | Read (masked secrets) |
| DELETE | `/v1beta1/projects/{project}/addon-configs/{type}` | Delete |

Authorization: same roles as module config CRUD for the project (no new permission product).

**Error cases (representative):** parent not found; parent not `STATUS_COMPLETED` when output required; spec-origin PUT forbidden; addon-config missing; conflict on `spec_version`; add-on type not supported by module.

All routes are mounted under the Entropy API prefix (for example `/api/v1beta1/...`). Requests that mutate state require the same `user-id` header as other Entropy HTTP APIs.

Protos live in `goto/proton` (`make proto`, `PROTON_COMMIT` bump).

**Chosen (D4) — v1:** `addon_revisions` and merged history APIs are **not** shipped in the first slice. Audit for v1 uses `updated_at`, current `spec` / `output` on the add-on row, and parent **resource** revisions (which include `spec.configs.addons` when the parent is versioned). See [Revision history](#revision-history) for the planned v2 design.

### Sample payloads

Illustrative JSON for a firehose parent `orn:entropy:firehose:team-a:orders-stream` with a `logs` add-on.

#### Parent resource `spec.configs` (stored in `resources.spec_configs`)

The `addons` key is stripped before module `PlanAction`, then re-injected before persist so drivers never see it.

```json
{
  "topic": "orders",
  "replicas": 3,
  "addons": {
    "logs": {
      "enabled": true,
      "on_failure": "continue",
      "labels": { "app": "orders" }
    }
  }
}
```

#### Module row `configs` (`modules.configs` bytea)

```json
{
  "replicas": 1,
  "addon_defaults": {
    "logs": {
      "enabled": true,
      "on_failure": "continue"
    }
  }
}
```

Merge rule: parent `spec.configs.addons.logs` **replaces** the module default for `logs` wholesale (not deep-merged field by field).

#### Project `addon-config` (`addon_configs.configs`)

Request body for `PUT /api/v1beta1/projects/team-a/addon-configs/logs`:

```json
{
  "configs": {
    "endpoint": "https://logging.example.com/api",
    "token": "REDACTED"
  }
}
```

GET responses mask `token` (same pattern as module configs once masking is wired).

#### Add-on API — apply (`PUT .../addons/logs`)

Request (optional `spec_version` for optimistic concurrency on `api`-origin updates):

```json
{
  "spec": {
    "enabled": true,
    "on_failure": "continue",
    "labels": { "app": "orders" }
  },
  "spec_version": 1
}
```

Response `200` (shape matches persisted row):

```json
{
  "urn": "orn:entropy:addon:firehose:team-a:orders-stream:logs",
  "parent_urn": "orn:entropy:firehose:team-a:orders-stream",
  "type": "logs",
  "spec": { "enabled": true, "on_failure": "continue", "labels": { "app": "orders" } },
  "output": { "registration_id": "orn:entropy:firehose:team-a:orders-stream:logs" },
  "status": "COMPLETED",
  "last_error": "",
  "origin": "spec",
  "spec_version": 2,
  "last_applied_spec": { "enabled": true, "on_failure": "continue", "labels": { "app": "orders" } },
  "created_at": "2026-03-20T10:00:00Z",
  "updated_at": "2026-03-20T10:05:00Z",
  "created_by": "alice",
  "updated_by": "alice"
}
```

Representative errors:

```json
{ "message": "add-on 'logs' is managed by parent spec; update the parent resource" }
```

```json
{ "message": "parent must be in STATUS_COMPLETED before applying add-on 'logs'" }
```

#### Add-on API — list (`GET .../addons`)

```json
{
  "addons": [
    {
      "urn": "orn:entropy:addon:firehose:team-a:orders-stream:logs",
      "parent_urn": "orn:entropy:firehose:team-a:orders-stream",
      "type": "logs",
      "spec": { "enabled": true, "on_failure": "continue" },
      "output": {},
      "status": "COMPLETED",
      "origin": "spec",
      "spec_version": 1
    }
  ]
}
```

#### External logging API (logs driver)

`PUT {addon-config.endpoint}/registrations` body sent by [`addons/logs`](../../addons/logs/logs.go):

```json
{
  "parent_urn": "orn:entropy:firehose:team-a:orders-stream",
  "namespace": "team-a-orders-stream",
  "spec": { "enabled": true, "on_failure": "continue", "labels": { "app": "orders" } }
}
```

`namespace` is read from the parent resource `state_output` JSON after firehose sync completes.

#### Parent resource revision (`revisions.spec_configs`)

When the parent is created/updated with `saveRevision: true`, a row in `revisions` snapshots the full parent spec including `addons`:

```json
{
  "topic": "orders",
  "replicas": 3,
  "addons": {
    "logs": { "enabled": true, "on_failure": "continue" }
  }
}
```

That snapshot records **declared** add-on desired state at the parent change; it does not record external apply output or `api`-only edits unless the parent spec was also updated.

### Revision history

Resource revisions today: table `revisions` keyed by `resource_id`, `GET /v1beta1/resources/{urn}/revisions` (gRPC `GetResourceRevisions`), ordered by `created_at DESC`.

**v1 audit:**

| Question | Where to look |
| -------- | ------------- |
| What add-ons were declared on the parent at revision *N*? | `revisions.spec_configs` → `addons` |
| Current desired + apply result for an add-on | `GET .../addons/{type}` |
| Who changed add-on via API only? | `addons.updated_at` / `updated_by` (no history in v1) |

**Planned v2 — dedicated add-on revisions + merged read:**

- Append to `addon_revisions` on material changes (see DDL above).
- **Write policy (deduplication):** when a parent update creates `revisions.id = R` and reconcile updates the add-on row in the same flow, set `addon_revisions.parent_revision_id = R` and **do not** expose a separate timeline entry for that reconcile (declared state already appears in the resource revision). Still write `addon_revisions` for: `api` PUT, successful/failed `Apply` when `output` / `status` / `last_applied_spec` change, and `delete`.
- **Read:** new endpoint, for example `GET /v1beta1/resources/{urn}/history`, returns a single list sorted by `created_at DESC` with discriminated entries:

```json
{
  "entries": [
    {
      "kind": "resource_revision",
      "id": 42,
      "created_at": "2026-03-20T10:00:00Z",
      "created_by": "alice",
      "reason": "action:update",
      "spec": { "configs": { "topic": "orders", "addons": { "logs": { "enabled": true } } } }
    },
    {
      "kind": "addon_revision",
      "id": 7,
      "created_at": "2026-03-20T10:05:00Z",
      "created_by": "bob",
      "reason": "apply",
      "trigger": "api",
      "addon_type": "logs",
      "addon_urn": "orn:entropy:addon:firehose:team-a:orders-stream:logs",
      "parent_revision_id": null,
      "spec": { "enabled": true, "labels": { "app": "orders" } },
      "output": { "registration_id": "orn:entropy:firehose:team-a:orders-stream:logs" },
      "status": "COMPLETED",
      "origin": "api"
    }
  ]
}
```

Per-type drill-down: `GET .../resources/{urn}/addons/{type}/revisions` (optional). Pagination uses a stable cursor `(created_at, kind, id)` after merge. Mask sensitive fields in add-on entries the same as live GET.

`addon-config` changes remain project-scoped; they are **not** merged into parent history unless a follow-up adds `addon_config_revisions`.

### Backend configuration and secrets

Per-project **addon-config**: one row per `(project, type)`, JSON `configs` (endpoint, credentials). Separate from module rows, which only hold `addon_defaults`.

```yaml
# addon-config (project=team-a, type=logs)
configs:
  endpoint: https://logging.example.com/api
  token: ${LOGS_API_TOKEN}
```

Sensitive fields use the same masking and `restoreSensitive` flow as modules (`masking.go`). Each add-on type registers sensitive paths for addon-config and add-on spec.

### Reference add-on: logs

**Module opt-in (v1):** `firehose` only; `Descriptor.Addons = ["logs"]`.

**Common add-on spec fields** (all types may use; drivers may ignore unknown keys):

| Field | Type | Default | Meaning |
| ----- | ---- | ------- | ------- |
| `enabled` | bool | `true` | `false` tombstones spec-origin row on reconcile |
| `on_failure` | string | `continue` | `fail` \| `continue` — whether reconcile/apply errors fail the parent operation |

**Requires:** project addon-config for type `logs`; parent `STATUS_COMPLETED`; parent `State.Output` containing at least workload identity fields the logging API expects (namespace and label selectors as produced by firehose sync — exact keys documented in the logs add-on implementation).

**Example parent `state_output`** (after firehose sync; field names are driver-defined):

```json
{
  "namespace": "team-a-orders-stream",
  "deployment": "orders-stream-firehose"
}
```

The **logs** driver requires non-empty `namespace` in this JSON.

**Apply:** register the parent workload with the external logging API using merged add-on spec, parent output, and addon-config.

**Delete:** remove registration.

**Not in scope:** replace streaming `Log()` / driver log tail; add-ons configure shipping registration only.

**Missing addon-config:** `Apply` fails with invalid/precondition error; add-on row may be `ERROR`.

### Reference backend: Barito

[Barito](https://github.com/BaritoLog) is the log platform behind GoTo-style log shipping (collector, router, and related tooling maintained under the [BaritoLog](https://github.com/BaritoLog) GitHub organization). It is the **reference backend** for workload log registration add-ons on Entropy.

- **v1 implementation:** the generic **`logs`** add-on driver (`addons/logs`) talks to a project-configured HTTP API (`addon-config`); that API may be operated by Barito or another compatible control plane.
- **Planned add-on type `barito`:** a dedicated driver aligned with legacy Odin barito add-ons (release config upsert/delete, cluster `addons.barito` defaults on the kubernetes module). Same `core/addon` framework; not shipped in the first implementation slice.

For product context and upstream projects, see [github.com/BaritoLog](https://github.com/BaritoLog).

### Observability

OTel counters for apply/delete results and timeouts, labelled by add-on type; gauge of add-ons in `ERROR`. Structured log per reconcile/apply with `parent_urn`, `addon_type`, `origin`, `trigger` (`write_reconcile` | `api`), `on_failure`, and outcome.

### Test plan (v1)

- Table-driven: default + spec merge, `enabled: false`, origin rules, spec-origin PUT rejection.
- Integration: dry-run parent create does not call external Apply.
- Integration: parent sync does not invoke add-on Apply; Apply only via add-on API.
- Integration: soft-delete parent tombstones add-ons without external delete; add-on DELETE with tombstoned parent still calls external delete.

## Alternatives

**Identity model A: first-class linked resource.** The add-on is a row in `resources` with a parent link.

- Pros: reuses store, API, revisions, masking, and the sync loop.
- Cons: couples add-ons to the resource lifecycle (statuses, sync columns); `Dependencies` are immutable and have no reverse lookup; tombstone and cascade rules get entangled with the parent.

**Identity model B: embedded in the parent spec.** Add-ons are config blocks inside the parent.

- Pros: simplest; one source of truth; no new table.
- Cons: cannot be managed, versioned or applied independently, which is a core goal.

**Chosen:** a separate table with its own API, a hybrid of the two: declared in the parent spec like B, stored and managed as independent records like A.

**Asynchronous reconciliation** via a dedicated worker loop would give retries, backoff and a visible `ERROR` state with less request latency. Rejected for v1; add-on Apply is decoupled from resource sync.

**addon_revisions on day one:** rejected for v1 (schema, API, masking cost); v2 design documented under [Revision history](#revision-history).

## Rollout

Opt-in with zero migration. A parent with no `addons` entry and no module defaults behaves exactly as today. Empty `addons` and `addon_configs` tables at deploy.

## Implementation order

1. This RFC agreed.
2. Proton messages + `PROTON_COMMIT`.
3. Postgres schema (`addons`, `addon_configs`).
4. Core: registry, merge/reconcile on write, origin rules (no sync coupling).
5. HTTP handlers.
6. `logs` add-on + firehose descriptor + addon-config paths.
7. (v2) `addon_revisions` store, write hooks, merged `GET .../history`, proton oneof messages.

## First implementation (reference example)

This section describes the **first shipped add-on stack** in Entropy. It is the concrete reference for how later add-on types (for example **`barito`** for [Barito](https://github.com/BaritoLog)) should plug into the same framework.

### Scope

| Layer | Location | Role |
| ----- | -------- | ---- |
| Engine and types | [`core/addon/`](../../core/addon/) | `Driver` registry, spec merge/extract, `Engine` (reconcile, API apply/delete), `origin` / `spec_version` |
| Parent hooks | [`core/addon_hooks.go`](../../core/addon_hooks.go), [`core/write.go`](../../core/write.go) | Strip `addons` from configs before `PlanAction`; re-inject into stored spec; **reconcile** spec-origin rows after parent upsert |
| Storage | [`internal/store/postgres/schema.sql`](../../internal/store/postgres/schema.sql), [`addon_store.go`](../../internal/store/postgres/addon_store.go) | `addons`, `addon_configs` tables |
| HTTP (pre-proton) | [`internal/server/v1/addons/http.go`](../../internal/server/v1/addons/http.go) | Nested routes under `/api/v1beta1/...` |
| Wiring | [`cli/serve.go`](../../cli/serve.go) | Register engine on `core.Service`; mount HTTP routes in [`internal/server/server.go`](../../internal/server/server.go) |

**Not in this slice:** gRPC/protobuf in `goto/proton` (HTTP only for now), `addon_revisions` / merged `history` API (see [Revision history](#revision-history)), OTel metrics (described above but not wired), addon-config response masking.

### Driver contract (as implemented)

```go
type Driver interface {
    Validate(spec json.RawMessage) error
    RequiresParentCompleted() bool
    Apply(ctx context.Context, parent ParentContext, spec json.RawMessage) (json.RawMessage, error)
    Delete(ctx context.Context, parent ParentContext, spec json.RawMessage) error
}
```

`ParentContext` carries parent URN, kind, project, name, and `State.Output` JSON. Types that need planned config or dependencies should extend this in a follow-up change; the **logs** driver only needs completed parent output.

### Decoupling from resource sync

- **Parent create/update:** updates add-on **rows** only (reconcile). No external I/O.
- **Resource sync** (`core/sync.go`): does **not** call the add-on engine.
- **External Apply/Delete:** only via add-on **HTTP API** (or a future worker). After reconcile, operators `PUT .../addons/{type}` once the parent is `STATUS_COMPLETED` when `RequiresParentCompleted()` is true.

This matches **Chosen (D1)** and avoids blocking or coupling the sync loop to Barito, logging APIs, or other backends.

### First add-on type: `logs`

| Item | Detail |
| ---- | ------ |
| Implementation | [`addons/logs/logs.go`](../../addons/logs/logs.go) |
| Module opt-in | [`modules/firehose/module.go`](../../modules/firehose/module.go) — `Descriptor.Addons = ["logs"]` |
| Project config | `PUT /api/v1beta1/projects/{project}/addon-configs/logs` with `configs.endpoint` and `configs.token` |
| Apply | HTTP PUT to `{endpoint}/registrations` with parent URN, namespace from firehose output, and spec |
| Delete | HTTP DELETE on `{endpoint}/registrations/{parent_urn}` |
| Prerequisite | Parent `STATUS_COMPLETED` and output JSON including `namespace` |

The **`logs`** driver is separate from the planned **`barito`** add-on type (see [Reference backend: Barito](#reference-backend-barito)). **`barito`** will target [Barito](https://github.com/BaritoLog) release-config APIs like Odin; **`logs`** remains a thin HTTP registration example for the first implementation.

### End-to-end flow (firehose + logs)

1. Deploy with `--migrate` so `addons` / `addon_configs` exist.
2. Configure project `addon-config` for `logs`.
3. Optional: set `addon_defaults` on the firehose **module** row, or declare `addons.logs` in the firehose resource `spec.configs`.
4. Create or update the firehose resource (reconcile creates/updates the add-on row; parent may stay `PENDING` until sync completes).
5. When the firehose is `STATUS_COMPLETED`, call `PUT /api/v1beta1/resources/{urn}/addons/logs` with desired spec (header `user-id` required, same as other APIs).

```http
PUT /api/v1beta1/resources/orn:entropy:firehose:team-a:orders-stream/addons/logs
user-id: alice
Content-Type: application/json

{"spec":{"enabled":true,"on_failure":"continue"}}
```

6. To tear down externally: `DELETE .../addons/logs` (works even if the parent is soft-deleted, using tombstone-inclusive parent load).

### What to copy for the next add-on type

1. Implement `core/addon.Driver` under `addons/<type>/`.
2. `addonRegistry.Register(addon.TypeX, ...)` in `cli/serve.go`.
3. Add the type string to the parent module `Descriptor.Addons`.
4. Document required `addon-config` JSON and spec shape.
5. Keep external calls on the **API apply path** unless a future RFC adds an async worker.

## Risks

- **Always-sync on API apply.** External calls run in the add-on API request path; slow APIs block that caller only (not resource sync). Mitigated by timeouts, default `on_failure: continue`, and idempotent manual PUT.
- **Two writers of desired state** (spec and API). Mitigated by `origin` and `spec_version` on API PUT.
- **Parent ERROR / PENDING.** Add-on rows may exist without successful external apply until parent completes or operator intervenes.
- **External orphans.** Soft-delete parent without prior add-on DELETE leaves external registrations until explicit add-on DELETE (if parent row still recoverable) or manual cleanup.
- **Force delete.** Hard parent delete without external add-on delete may orphan external state.
- **Tombstoned parents** require tombstone-inclusive getter for add-on delete teardown.

## Open Questions

- Whether `fail` add-ons should include a retry hint in the error response.
- Whether v2 `history` should extend existing `GetResourceRevisions` (protobuf oneof) or add a new RPC to avoid breaking clients.
- Whether an opt-in background retry worker for add-ons is needed after v1.
- Hook execution order when multiple add-on types are applied in one operation (v1: undefined order; types must not depend on each other).
