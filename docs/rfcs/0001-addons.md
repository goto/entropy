# RFC 0001: Add-ons

Status: Draft (revision 5 — add-ons are mutated only through the add-on API)

### Changes since revision 3

- **Write decoupling:** resource writes (create, update, actions, sync) never mutate add-ons, and add-on writes never mutate resources. The add-on API is the only way to create or change an add-on. The **single exception** is resource soft-delete, which also soft-deletes the resource's live add-ons (DB only, same transaction). Add-ons can still be **read** together with their resource.
- Removed as a consequence: parent-spec `addons` block, module `addon_defaults`, merge/reconcile on parent write, the `origin` column, and the `enabled` / `on_failure` spec fields.
- **Add-on URN** is `{parent_urn}:{type}` (was `orn:entropy:addon:<kind>:<project>:<name>:<type>`).
- **Storage:** `id BIGSERIAL` primary key; `(parent_urn, type)` is unique **only among live rows** (partial unique index), so deleting and re-adding an add-on inserts a new row and the old tombstoned row stays as history.
- **Driver contract** gains a `config` argument (project addon-config) on `Apply`/`Delete`, `RequiresConfig()` and `SensitiveConfigKeys()`.
- **Second add-on type `webui`** for dagger: gates the existing Flink Web UI proxy (`/resources/{urn}/webui/`, JobManager `:8081`).
- **Minimal addon-config masking** is in scope.
- **Failure semantics** on the API path clarified: a failed update keeps the previous spec, so no `last_applied_spec` column is needed.

## Context

Entropy orchestrates resources (firehose, dagger, kafka, flink, ...). Each resource usually needs supporting resources: log shipping, alerts, Kafka ACLs. Today these are embedded in each module's config and driver:

- Kafka ACL: firehose (`ACL`, `DLQACL`, `StreamSecurity`, see `modules/firehose/config.go`) and dagger (`ACLMounts`, see `modules/dagger/config.go`) each recompute it on every Plan.
- Monitoring: firehose `Telegraf` and the KEDA autoscaler; flink and dagger carry a `prometheus_url`. There is no alert-rule construct.
- Logs: only a streaming `Log()` method on drivers (`core/read.go`). There is no log-shipping config.

Consequences: the same concern is re-implemented per module and cannot be managed without updating the parent.

This RFC introduces **add-ons**: optional supporting resources attached to a parent resource and managed **independently** through their own API.

## Goals

- A generic add-on framework usable by any module that opts in.
- Two reference add-ons: **logs** (log-shipping registration via an external logging API, firehose) and **webui** (Flink Web UI of the JobManager, dagger).
- Add-ons managed only through a nested add-on API, with no coupling to the resource write path.
- Add-ons readable alongside their parent resource.
- No behaviour change for existing resources.

## Non-goals

- Declaring add-ons in the parent spec or module defaults, or reconciling them on parent writes.
- Cascading add-on changes when a parent is updated, and any external (driver) cleanup when a parent is deleted. The only cascade is the DB soft-delete of add-ons on parent soft-delete.
- Migrating existing Telegraf, ACL or autoscaler config (follow-up RFCs: Kafka ACL, alerts).
- A background retry loop or queued reconciliation for add-ons (timeouts and manual re-apply only).
- A dedicated add-on permission model (reuse existing project/resource authorization; see API).

## Design

### Write ownership

| Writer | May mutate resources | May mutate add-ons |
| ------ | -------------------- | ------------------ |
| Resource create / update / actions (`core/write.go`) | Yes | **No** |
| Resource soft-delete (`SoftDeleteResource`) | Yes | **Soft-delete only** (tombstone live add-ons of that resource) |
| Resource sync loop (`core/sync.go`) | Yes | **No** |
| Add-on API | **No** | Yes |

The add-on service **reads** the parent resource (existence, project, status, `State.Output`) to validate and to give the driver context, but never writes to `resources`.

The only resource-side write to `addons` is the cascade on soft-delete: the store's `SoftDelete` also runs `UPDATE addons SET deleted_at = current_timestamp, deleted_by = $user WHERE parent_urn = $urn AND deleted_at IS NULL` in the **same transaction** as the resource tombstone, so the two can never diverge. `core/write.go` and `core/sync.go` are not changed; the cascade lives in `internal/store/postgres/resource_store.go`. It never calls add-on drivers (see [Deletion](#deletion)).

### Identity and storage

Add-ons live in a separate generic `addons` table, not foreign-keyed to `resources`, since not all modules support the same add-ons. Every row stores `parent_urn` (full resource URN).

| Column | Notes |
| ------ | ----- |
| `id` | `BIGSERIAL` primary key |
| `urn` | `{parent_urn}:{type}`, e.g. `orn:entropy:firehose:team-a:orders-stream:logs`. A resource URN has 5 segments and an add-on URN has 6, so they cannot be confused |
| `parent_urn`, `type` | At most **one live** add-on per parent and type (partial unique index on `deleted_at IS NULL`) |
| `spec` | JSON, validated by the add-on type |
| `output` | JSON, last successful `Apply` result (opaque per type); returned on GET |
| `status`, `last_error` | `COMPLETED` or `ERROR`, set inline by the synchronous call |
| `spec_version` | Monotonic integer incremented on each successful spec persist; used for optimistic concurrency on PUT |
| `created_at/by`, `updated_at/by`, `deleted_at/by` | `deleted_at` is a tombstone, as for resources |

**Deleting and re-adding:** deleting an add-on sets `deleted_at` on its row. Because `(parent_urn, type)` is only unique among live rows, adding the same type to the same parent later inserts a **new** row with a new `id` and the same `urn`; the tombstoned row is kept untouched as history. Lookups by URN or `(parent_urn, type)` always filter `deleted_at IS NULL`.

#### Database schema (DDL)

Shipped in [`internal/store/postgres/schema.sql`](../../internal/store/postgres/schema.sql):

```sql
-- Per-parent add-on instance; at most one live row per (parent_urn, type)
CREATE TABLE IF NOT EXISTS addons
(
    id                BIGSERIAL   NOT NULL PRIMARY KEY,
    urn               TEXT        NOT NULL,          -- '{parent_urn}:{type}'
    parent_urn        TEXT        NOT NULL,
    type              TEXT        NOT NULL,
    spec              bytea       NOT NULL,
    output            bytea       NOT NULL DEFAULT '{}',
    status            TEXT        NOT NULL,
    last_error        TEXT        NOT NULL DEFAULT '',
    spec_version      BIGINT      NOT NULL DEFAULT 1,
    created_at        timestamptz NOT NULL DEFAULT current_timestamp,
    updated_at        timestamptz NOT NULL DEFAULT current_timestamp,
    created_by        TEXT        NOT NULL DEFAULT '<unknown>',
    updated_by        TEXT        NOT NULL DEFAULT '<unknown>',
    deleted_at        timestamptz,
    deleted_by        TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_addons_parent_type_active ON addons (parent_urn, type) WHERE deleted_at IS NULL;

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

The partial unique index also serves list-by-parent lookups. v1 does not support project-wide add-on listing.

**Example `addons` row** (after a successful `PUT .../addons/logs`):

| Column | Example value |
| ------ | ------------- |
| `id` | `17` |
| `urn` | `orn:entropy:firehose:team-a:orders-stream:logs` |
| `parent_urn` | `orn:entropy:firehose:team-a:orders-stream` |
| `type` | `logs` |
| `spec` | `{"labels":{"app":"orders"}}` |
| `output` | `{"registration_id":"orn:entropy:firehose:team-a:orders-stream"}` |
| `status` | `COMPLETED`; `ERROR` if the last `Apply` failed |
| `spec_version` | `1` |

**Chosen (D3):** URN is `{parent_urn}:{type}`. It inherits the parent's kind, project and name (so two resources with the same name under different kinds do not collide) and is readable without an extra prefix. Rows are identified internally by `id`.

### Capability declaration

The parent module's `Descriptor` gains `Addons []string` (similar to `Descriptor.Dependencies`, `core/module/module.go`), for example `firehose` declares `["logs"]`. A PUT naming a type the parent's module does not declare is rejected.

Spec shape is validated by each add-on type (`Driver.Validate`).

### Add-on contract

Separate from the module `Driver`, registered by type in an add-on registry:

```go
type Driver interface {
    Validate(spec json.RawMessage) error
    RequiresParentCompleted() bool
    RequiresConfig() bool
    SensitiveConfigKeys() []string
    Apply(ctx context.Context, parent ParentContext, config, spec json.RawMessage) (json.RawMessage, error)
    Delete(ctx context.Context, parent ParentContext, config, spec json.RawMessage) error
}
```

- **Parent context:** `ParentContext` carries parent URN, kind, project, name and `State.Output` JSON. It is read-only.
- **Config:** `config` is the project addon-config JSON for the type (endpoint, credentials), passed explicitly so drivers never read credentials from global state or the environment. When `RequiresConfig()` is true, missing addon-config fails the call before the driver runs; when false, the lookup is skipped and `config` is `nil`.
- **Sensitive keys:** `SensitiveConfigKeys()` lists top-level addon-config keys masked on read (see [Backend configuration and secrets](#backend-configuration-and-secrets)).
- **Output:** `Apply` returns JSON stored in `output` and exposed on GET. Empty object if the type has nothing to return.
- **Idempotency:** `Apply` and `Delete` must be safe to repeat for the same `(parent_urn, type)` and spec. Implementations should use stable external keys derived from `parent_urn`, not request ids or row ids.
- **Timeouts:** each driver call runs under `addons.timeout` from the server config (default `30s`). Timeout counts as failure.
- **Parent load:** all operations use `GetByURN`, which hides soft-deleted parents. Once a parent is soft-deleted its add-ons are tombstoned too, so there is nothing left to read or delete through the API.

### Apply and failure

`PUT .../addons/{type}`:

1. Load the parent; it must exist and not be soft-deleted.
2. Check the type against the parent module's `Descriptor.Addons`; validate the spec.
3. If `spec_version` is given and differs from the live row, return conflict.
4. If `RequiresParentCompleted()`, the parent must be `STATUS_COMPLETED`.
5. If `RequiresConfig()`, load the project addon-config for the type; missing → invalid.
6. Call `Driver.Apply` under a timeout.
7. Persist the result and, on failure, return the error to the caller:
   - **Success:** save `spec` and `output`, set `status=COMPLETED`, clear `last_error`, bump `spec_version`.
   - **Failure on an update:** keep the previous `spec` and `output`, set `status=ERROR` and `last_error`.
   - **Failure on the first PUT:** insert the row with the requested `spec` and `status=ERROR`, so the failure is visible and can be retried.

   `spec` is therefore always the last spec that was successfully applied, except on a first PUT that failed. `status=ERROR` means the backend may not match `spec`. The spec of a failed update is not stored; v2 `addon_revisions` would record it.

Recovery is an idempotent re-PUT. There is no background retry.

Resource sync (`core/sync.go`) never calls add-on logic, so slow or failing backends cannot block the sync loop (**Chosen (D1)**).

### Deletion

**Chosen (D2):**

| API entrypoint | Resource | Add-on row | External |
| -------------- | -------- | ---------- | -------- |
| DELETE `.../addons/{type}` | — | Tombstoned after `Driver.Delete` succeeds | `Driver.Delete` |
| DELETE resource (default = soft delete) | Soft-deleted, as today | All live add-ons of the resource tombstoned, same transaction | **None** |
| DELETE resource (`force`) | Module delete action, as today (row stays, not tombstoned) | Untouched | None |

The cascade is DB-only: it does not call `Driver.Delete`, so external registrations stay behind. Operators must DELETE add-ons **before** deleting the parent if external cleanup is required; after the cascade the add-on rows are tombstoned and can no longer be deleted through the API.

Force delete does not tombstone the resource row, so it does not cascade either; add-ons follow the resource tombstone, not its status.

### Reading add-ons with the resource

Reads may join resources and add-ons; writes may not.

- **v1:** `GET /v1beta1/resources/{urn}/addons` lists the live add-ons of a resource.
- **Follow-up:** the resource GET response gains a read-only `addons` field (live add-ons for that resource, same shape as the list API, sensitive fields masked). This needs a proton field and is populated in the resource read path only; it is ignored on resource writes.

### API

Nested under the parent; authorization matches the parent resource (same project and resource-level checks as parent GET/PUT).

| Method | Path | Notes |
| ------ | ---- | ----- |
| PUT | `/v1beta1/resources/{urn}/addons/{type}` | Create or update + `Apply`; optional `spec_version` |
| GET | `/v1beta1/resources/{urn}/addons/{type}` | Read |
| GET | `/v1beta1/resources/{urn}/addons` | List live add-ons |
| DELETE | `/v1beta1/resources/{urn}/addons/{type}` | External delete + tombstone |

**addon-config** (project-level backend credentials):

| Method | Path | Notes |
| ------ | ---- | ----- |
| PUT | `/v1beta1/projects/{project}/addon-configs/{type}` | Create/update |
| GET | `/v1beta1/projects/{project}/addon-configs/{type}` | Read (masked secrets) |
| DELETE | `/v1beta1/projects/{project}/addon-configs/{type}` | Delete |

Authorization: same roles as module config CRUD for the project (no new permission product).

**Error cases (representative):** parent not found; parent not `STATUS_COMPLETED` when required; addon-config missing; conflict on `spec_version`; add-on type not supported by module.

HTTP status mapping: invalid → 400, not found → 404, conflict → 409, unsupported → 501, other → 500. Error body: `{"message": "..."}`.

All routes are mounted under the Entropy API prefix (for example `/api/v1beta1/...`). Mutating requests require the same `user-id` header as other Entropy HTTP APIs (400 if missing).

v1 ships plain HTTP handlers; protos in `goto/proton` (`make proto`, `PROTON_COMMIT` bump) follow later.

**Chosen (D4) — v1:** `addon_revisions` and history APIs are not shipped. Audit for v1 uses `updated_at/by` on the live row plus the tombstoned rows of earlier add-ons. See [Revision history](#revision-history).

### Sample payloads

Illustrative JSON for a firehose parent `orn:entropy:firehose:team-a:orders-stream` with a `logs` add-on.

#### Project `addon-config`

Request body for `PUT /api/v1beta1/projects/team-a/addon-configs/logs`:

```json
{
  "configs": {
    "endpoint": "https://logging.example.com/api",
    "token": "<logging-api-token>"
  }
}
```

GET response masks sensitive keys:

```json
{
  "project": "team-a",
  "type": "logs",
  "configs": {
    "endpoint": "https://logging.example.com/api",
    "token": "****"
  }
}
```

A PUT that sends `"token": "****"` keeps the stored token, so clients can round-trip a GET response to change only `endpoint`.

#### Add-on API — apply (`PUT .../addons/logs`)

Request:

```json
{
  "spec": { "labels": { "app": "orders" } },
  "spec_version": 1
}
```

Response `200`:

```json
{
  "urn": "orn:entropy:firehose:team-a:orders-stream:logs",
  "parent_urn": "orn:entropy:firehose:team-a:orders-stream",
  "type": "logs",
  "spec": { "labels": { "app": "orders" } },
  "output": { "registration_id": "orn:entropy:firehose:team-a:orders-stream" },
  "status": "COMPLETED",
  "last_error": "",
  "spec_version": 2,
  "created_at": "2026-03-20T10:00:00Z",
  "updated_at": "2026-03-20T10:05:00Z",
  "created_by": "alice",
  "updated_by": "alice"
}
```

Representative errors:

```json
{ "message": "parent must be in STATUS_COMPLETED before applying add-on 'logs'" }
```

```json
{ "message": "addon-config 'logs' is not configured for project 'team-a'" }
```

#### Add-on API — list (`GET .../addons`)

```json
{
  "addons": [
    {
      "urn": "orn:entropy:firehose:team-a:orders-stream:logs",
      "parent_urn": "orn:entropy:firehose:team-a:orders-stream",
      "type": "logs",
      "spec": { "labels": { "app": "orders" } },
      "output": { "registration_id": "orn:entropy:firehose:team-a:orders-stream" },
      "status": "COMPLETED",
      "spec_version": 1
    }
  ]
}
```

#### External logging API (logs driver)

`PUT {addon-config.endpoint}/registrations`, sent by [`addons/logs`](../../addons/logs/logs.go) with `Authorization: Bearer <addon-config.token>`:

```json
{
  "parent_urn": "orn:entropy:firehose:team-a:orders-stream",
  "namespace": "team-a-orders-stream",
  "spec": { "labels": { "app": "orders" } }
}
```

`namespace` is read from the parent resource `state_output` JSON after firehose sync completes.

### Revision history

Resource revisions today: table `revisions` keyed by `resource_id`, `GET /v1beta1/resources/{urn}/revisions` (gRPC `GetResourceRevisions`), ordered by `created_at DESC`.

**v1 audit:**

| Question | Where to look |
| -------- | ------------- |
| Current spec + apply result for an add-on | `GET .../addons/{type}` |
| Who last changed an add-on | `addons.updated_at` / `updated_by` on the live row |
| Earlier, deleted add-ons of the same type | Tombstoned `addons` rows for the same `(parent_urn, type)` |

**Planned v2 — dedicated add-on revisions + merged read:**

```sql
CREATE TABLE IF NOT EXISTS addon_revisions
(
    id           BIGSERIAL   NOT NULL PRIMARY KEY,
    addon_id     BIGINT      NOT NULL,              -- addons.id
    parent_urn   TEXT        NOT NULL,
    addon_type   TEXT        NOT NULL,
    reason       TEXT        NOT NULL DEFAULT '<unknown>', -- apply | delete
    spec         bytea       NOT NULL,
    output       bytea       NOT NULL DEFAULT '{}',
    status       TEXT        NOT NULL,
    last_error   TEXT        NOT NULL DEFAULT '',
    spec_version BIGINT      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT current_timestamp,
    created_by   TEXT        NOT NULL DEFAULT '<unknown>'
);
CREATE INDEX IF NOT EXISTS idx_addon_revisions_parent_urn ON addon_revisions (parent_urn, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_addon_revisions_addon_id ON addon_revisions (addon_id, created_at DESC);
```

- Append to `addon_revisions` on every add-on API PUT (success or failure) and DELETE.
- **Read:** a new endpoint, for example `GET /v1beta1/resources/{urn}/history`, returns resource revisions and add-on revisions in one list sorted by `created_at DESC`, with discriminated entries:

```json
{
  "entries": [
    {
      "kind": "resource_revision",
      "id": 42,
      "created_at": "2026-03-20T10:00:00Z",
      "created_by": "alice",
      "reason": "action:update",
      "spec": { "configs": { "topic": "orders" } }
    },
    {
      "kind": "addon_revision",
      "id": 7,
      "created_at": "2026-03-20T10:05:00Z",
      "created_by": "bob",
      "reason": "apply",
      "addon_type": "logs",
      "addon_urn": "orn:entropy:firehose:team-a:orders-stream:logs",
      "spec": { "labels": { "app": "orders" } },
      "output": { "registration_id": "orn:entropy:firehose:team-a:orders-stream" },
      "status": "COMPLETED"
    }
  ]
}
```

This is a read-side merge only; neither writer touches the other's revisions. Per-type drill-down: `GET .../resources/{urn}/addons/{type}/revisions` (optional). Pagination uses a stable cursor `(created_at, kind, id)` after merge.

`addon-config` changes remain project-scoped and are not merged into resource history unless a follow-up adds `addon_config_revisions`.

### Backend configuration and secrets

Per-project **addon-config**: one row per `(project, type)`, JSON `configs` (endpoint, credentials).

```yaml
# addon-config (project=team-a, type=logs)
configs:
  endpoint: https://logging.example.com/api
  token: ${LOGS_API_TOKEN}
```

**Minimal masking (v1):**

- Each driver declares `SensitiveConfigKeys()` (top-level keys; `logs` → `["token"]`).
- GET addon-config replaces those values with `"****"`.
- PUT addon-config that sends `"****"` for a sensitive key keeps the stored value; any other value replaces it.
- The HTTP request logger does not log request or response bodies for `/addon-configs/` routes.
- Add-on specs carry no secrets in v1 and are not masked.

Full reuse of the HMAC masker (`pkg/masking`, nested paths, masking add-on specs) is deferred.

### Reference add-on: logs

**Module opt-in:** `firehose` only; `Descriptor.Addons = ["logs"]`.

**Spec:**

| Field | Type | Default | Meaning |
| ----- | ---- | ------- | ------- |
| `labels` | map of string | `{}` | Extra labels attached to the log registration |

**Requires:** project addon-config for type `logs`; parent `STATUS_COMPLETED`; parent `State.Output` with a non-empty `namespace`.

**Example parent `state_output`** (after firehose sync; field names are driver-defined):

```json
{
  "namespace": "team-a-orders-stream",
  "release_name": "orders-stream-firehose"
}
```

**Apply:** register the parent workload with the external logging API using the spec, parent output, and addon-config.

**Delete:** remove the registration. A `404` from the logging API counts as success.

**Not in scope:** replacing streaming `Log()` / driver log tail; add-ons configure shipping registration only.

### Second add-on: webui (dagger)

Exposes the Flink Web UI served by the dagger's JobManager (`<deployment_id>-rest:8081`). It shows that an add-on can switch on an **Entropy-side** capability instead of calling an external API.

**Existing building block:** a read-only (GET/HEAD) reverse proxy at `/api/v1beta1/resources/{urn}/webui/` reaches the JobManager REST service through the kube API server service proxy:

- `internal/server/v1/resources/webui.go` (`RegisterWebUI`): the route
- `core/read.go` (`GetWebUIProxy`) and the module interface `WebUIProxyable` (`core/module/driver.go`)
- `modules/dagger/driver_ui.go` (`WebUIProxy`): resolves the flink dependency's kube cluster, namespace and `<deployment_id>-rest`
- `pkg/kube/client.go` (`ProxyServiceHandler`)

**Module opt-in:** `dagger`; `Descriptor.Addons = ["webui"]`.

**Spec:** empty object `{}` (unknown keys rejected).

**Driver flags:** `RequiresParentCompleted() = true`, `RequiresConfig() = false` (no addon-config), no sensitive keys.

**Apply:** gets the proxy for the parent (`GetWebUIProxy`) and serves an in-process `GET /overview` (Flink REST cluster overview) through it. A non-2xx response or proxy error fails the apply (`status=ERROR`). On success the output records where the UI is served:

```json
{
  "url": "/api/v1beta1/resources/orn:entropy:dagger:team-a:orders-agg/webui/",
  "flink_version": "1.17.1"
}
```

**Delete:** nothing external to remove; the row is tombstoned.

**Gating:** the `/webui/` route serves only while the resource has a **live `webui` add-on with `status=COMPLETED`**. Otherwise it returns `404` with a plain-text body (like the other errors of the proxy route):

```text
webui add-on is not enabled for resource 'orn:entropy:dagger:team-a:orders-agg'
```

So DELETE on the add-on, or soft-deleting the dagger (cascade), turns the UI off. The gate only **reads** the `addons` table, so it fits [Write ownership](#write-ownership).

**Not in scope:** Ingress/HTTPRoute or any direct cluster exposure; write methods (the proxy stays GET/HEAD only, so job cancel/submit from the UI does not work).

### Reference backend: Barito

[Barito](https://github.com/BaritoLog) is the log platform behind GoTo-style log shipping (collector, router, and related tooling maintained under the [BaritoLog](https://github.com/BaritoLog) GitHub organization). It is the **reference backend** for workload log registration add-ons on Entropy.

- **v1 implementation:** the generic **`logs`** add-on driver (`addons/logs`) talks to a project-configured HTTP API (`addon-config`); that API may be operated by Barito or another compatible control plane.
- **Planned add-on type `barito`:** a dedicated driver aligned with legacy Odin barito add-ons (release config upsert/delete). Same `core/addon` framework; not shipped in the first implementation slice.

### Observability

OTel counters for apply/delete results and timeouts, labelled by add-on type; gauge of add-ons in `ERROR`. Structured log per apply/delete with `parent_urn`, `addon_type` and outcome. (Not wired in the first slice.)

### Test plan

- Service: allow-list, wrong project, parent not `STATUS_COMPLETED`, missing addon-config, `spec_version` conflict, driver error/timeout persists `ERROR`.
- Store: delete then re-add inserts a new row and keeps the tombstoned one; two live rows for the same `(parent_urn, type)` are rejected.
- Resource soft-delete tombstones all live add-ons of that resource in the same transaction, calls no driver, and leaves other resources' add-ons alone; force delete leaves add-ons untouched.
- addon-config masking and `"****"` restore.
- `logs` driver against a fake HTTP backend (request shape, auth header, missing namespace, non-2xx, idempotent delete).
- `webui` driver with a fake proxy (output shape, non-2xx probe, proxy error); Apply succeeds without addon-config.
- `/webui/` gate: no add-on → 404, `ERROR` add-on → 404, `COMPLETED` add-on → proxied.
- Existing resource tests unchanged; resource writes and sync never touch the `addons` table.

## Alternatives

**Identity model A: first-class linked resource.** The add-on is a row in `resources` with a parent link.

- Pros: reuses store, API, revisions, masking, and the sync loop.
- Cons: couples add-ons to the resource lifecycle (statuses, sync columns); `Dependencies` are immutable and have no reverse lookup; tombstone and cascade rules get entangled with the parent.

**Identity model B: embedded in the parent spec.** Add-ons are config blocks inside the parent.

- Pros: simplest; one source of truth; no new table.
- Cons: cannot be managed, versioned or applied independently, and couples add-on changes to resource writes.

**Declaring add-ons in the parent spec / module defaults** (earlier revisions): rejected for now. It makes resource writes mutate add-on rows, needs two sources of desired state (`origin`), and complicates failure handling. Can be revisited in a later RFC on top of the add-on API.

**Chosen:** a separate table with its own API; the only coupling to resources is read-only.

**Asynchronous reconciliation** via a dedicated worker loop: rejected for v1.

**Opaque add-on URN** (hash or random id): shorter and fixed-length but unreadable. Rejected in favour of `{parent_urn}:{type}`.

## Rollout

Opt-in with zero migration of existing data. Empty `addons` and `addon_configs` tables at deploy. The resource write and sync paths are unchanged; soft-delete gains the add-on cascade in the store.

## Implementation order

1. This RFC agreed.
2. Postgres schema (`addons`, `addon_configs`).
3. `core/addon`: types, registry, driver contract, service (apply/get/list/delete, addon-config CRUD with masking).
4. HTTP handlers + request-logger exclusion for addon-configs.
5. `logs` add-on + firehose descriptor + wiring in `cli/serve.go`.
6. `webui` add-on + dagger descriptor + `/webui/` gate.
7. Follow-up: read-only `addons` field on resource GET (proton).
8. v2: `addon_revisions`, merged `GET .../history`.

## First implementation (reference example)

This section describes the **first shipped add-on stack** in Entropy and is the reference for later types (for example **`barito`**).

### Scope

| Layer | Location | Role |
| ----- | -------- | ---- |
| Types and service | [`core/addon/`](../../core/addon/) | `Driver` registry, `Service` (apply/get/list/delete, addon-config CRUD + masking), `spec_version` |
| Storage | [`internal/store/postgres/schema.sql`](../../internal/store/postgres/schema.sql), [`addon_store.go`](../../internal/store/postgres/addon_store.go) | `addons`, `addon_configs` tables |
| HTTP | [`internal/server/v1/addons/http.go`](../../internal/server/v1/addons/http.go) | Nested routes under `/api/v1beta1/...` |
| Wiring | [`cli/serve.go`](../../cli/serve.go) | Build registry + service; mount routes in [`internal/server/server.go`](../../internal/server/server.go) |

**Not in this slice:** proton/gRPC, `addons` field on resource GET, `addon_revisions` / history API, OTel metrics, full HMAC masking.

### First add-on type: `logs`

| Item | Detail |
| ---- | ------ |
| Implementation | [`addons/logs/logs.go`](../../addons/logs/logs.go) |
| Module opt-in | [`modules/firehose/module.go`](../../modules/firehose/module.go) — `Descriptor.Addons = ["logs"]` |
| Project config | `PUT /api/v1beta1/projects/{project}/addon-configs/logs` with `configs.endpoint` and `configs.token` |
| Apply | HTTP PUT to `{endpoint}/registrations` with parent URN, namespace from firehose output, and spec |
| Delete | HTTP DELETE on `{endpoint}/registrations/{parent_urn}` (404 = success) |
| Prerequisite | Parent `STATUS_COMPLETED` and output JSON including `namespace` |

### Second add-on type: `webui`

| Item | Detail |
| ---- | ------ |
| Implementation | [`addons/webui/webui.go`](../../addons/webui/webui.go) |
| Module opt-in | [`modules/dagger/module.go`](../../modules/dagger/module.go) — `Descriptor.Addons = ["webui"]` |
| Project config | None |
| Apply | Probe `GET /overview` through the existing `/webui` proxy; record `url` and `flink_version` |
| Delete | No external call; tombstone |
| Gate | `/api/v1beta1/resources/{urn}/webui/` serves only with a live `COMPLETED` `webui` add-on |
| Prerequisite | Dagger `STATUS_COMPLETED` |

### End-to-end flow (firehose + logs)

1. Deploy with `--migrate` so `addons` / `addon_configs` exist.
2. Configure project `addon-config` for `logs`.
3. Create the firehose resource as usual and wait for `STATUS_COMPLETED`.
4. Apply the add-on (header `user-id` required, same as other APIs):

```http
PUT /api/v1beta1/resources/orn:entropy:firehose:team-a:orders-stream/addons/logs
user-id: alice
Content-Type: application/json

{"spec":{"labels":{"app":"orders"}}}
```

5. To tear down: `DELETE .../addons/logs` **before** deleting the parent. Soft-deleting the parent tombstones the add-on rows without calling the logging API.

### What to copy for the next add-on type

1. Implement `core/addon.Driver` under `addons/<type>/`.
2. `addonRegistry.Register(addon.TypeX, ...)` in `cli/serve.go`.
3. Add the type string to the parent module `Descriptor.Addons`.
4. Document the required `addon-config` JSON (and its sensitive keys) and spec shape, or set `RequiresConfig()` to false if the type needs none (like `webui`).
5. Keep all add-on mutations on the add-on API; the only resource-side write is the soft-delete cascade in the store.

## Risks

- **Synchronous apply.** External calls run in the add-on API request; slow backends block that caller only, never resource writes or sync. Mitigated by timeouts and idempotent re-PUT.
- **Parent ERROR / PENDING.** Apply is refused until the parent completes (for types that require output).
- **External orphans.** Soft-deleting a parent tombstones its add-ons without calling `Driver.Delete`. Without a prior add-on DELETE, external registrations remain and need manual cleanup. A follow-up may block parent delete while live add-ons exist, or list them in the delete error.
- **Stale parent output.** An add-on applied against parent output does not re-apply when the parent changes (e.g. namespace). Operators re-PUT the add-on.
- **Long URNs.** The add-on URN is the parent URN plus `:<type>`, so it grows with the parent URN. Accepted for readability; the API addresses add-ons by parent URN and type in the path.

## Open Questions

- Should soft-deleting a parent with live add-ons be blocked (or warn) instead of tombstoning them without external cleanup?
- Should the resource GET embed add-ons by default, or only behind a flag (e.g. `?with_addons=true`)?
- Whether v2 `history` should extend `GetResourceRevisions` (protobuf oneof) or add a new RPC.
- Whether an opt-in background retry worker for add-ons is needed after v1.
