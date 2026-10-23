# RFC 0001: Add-ons

Status: Draft (revision 2 — implementation-ready)

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
- A **background retry loop** or queued reconciliation for add-ons (timeouts and manual re-apply only in v1). **Sync-adjacent triggers** when the parent reaches `STATUS_COMPLETED` are in scope; see Reconciliation triggers.
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

Parent writes (`execAction` in `core/write.go`) plan and upsert the parent, usually leaving it `STATUS_PENDING`. Resolved `State.Output` is produced in `handleSync` → `SyncState` (`core/sync.go`).

**Chosen (D1):** split reconcile (persist desired add-on rows from merged defaults/spec) from external **Apply**.

| Trigger | Code path | Parent state | Add-on action |
| ------- | --------- | ------------ | ------------- |
| Parent create/update (not dry-run) | After successful parent `upsert` in `execAction` | Any | Reconcile `spec`-origin rows (spec, tombstone removed types); no external `Apply` unless type documents it does not need output |
| Parent reaches `STATUS_COMPLETED` | End of successful `handleSync` when status becomes or remains `STATUS_COMPLETED` after create/update | `STATUS_COMPLETED` | For each enabled add-on row (spec or api origin): run external `Apply` if spec changed since last successful apply or row is new |
| Parent `PUT .../addons/{type}` | Add-on handler | `STATUS_COMPLETED` required for types that need output (e.g. logs); otherwise invalid with clear error | Validate, persist, `Apply` |
| Parent soft-delete | `SoftDeleteResource` | Tombstoned parent | Tombstone all add-on rows for `parent_urn`; **no** external `Delete` |
| Add-on `DELETE` | Add-on handler | Parent may be soft-deleted (tombstone-inclusive get) | External `Delete`, then tombstone add-on row |
| Force delete parent | `DeleteResource` with force | Hard-deleted parent | Tombstone or delete add-on rows in DB; **no** mandatory external `Delete` in v1 |

If the parent stays `STATUS_ERROR` or `STATUS_PENDING`, spec-origin rows exist but external `Apply` does not run until the parent completes successfully. Operators may use API PUT after recovery.

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

Protos live in `goto/proton` (`make proto`, `PROTON_COMMIT` bump).

**Chosen (D4):** `addon_revisions` and `GET .../revisions` are **deferred** from v1. Audit needs are met by `updated_at`, `spec`, and `output` on the add-on row until a follow-up adds revisions.

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

**Requires:** project addon-config for type `logs`; parent `STATUS_COMPLETED`; parent `State.Output` containing at least workload identity fields the logging API expects (namespace and label selectors as produced by firehose sync — exact keys documented in the logs add-on implementation).

**Apply:** register the parent workload with the external logging API using merged add-on spec, parent output, and addon-config.

**Delete:** remove registration.

**Not in scope:** replace streaming `Log()` / driver log tail; add-ons configure shipping registration only.

**Missing addon-config:** `Apply` fails with invalid/precondition error; add-on row may be `ERROR`.

### Observability

OTel counters for apply/delete results and timeouts, labelled by add-on type; gauge of add-ons in `ERROR`. Structured log per reconcile/apply with `parent_urn`, `addon_type`, `origin`, `trigger` (`write_reconcile` | `sync_completed` | `api`), `on_failure`, and outcome.

### Test plan (v1)

- Table-driven: default + spec merge, `enabled: false`, origin rules, spec-origin PUT rejection.
- Integration: dry-run parent create does not call external Apply.
- Integration: add-on Apply runs after parent sync to `STATUS_COMPLETED`.
- Integration: soft-delete parent tombstones add-ons without external delete; add-on DELETE with tombstoned parent still calls external delete.

## Alternatives

**Identity model A: first-class linked resource.** The add-on is a row in `resources` with a parent link.

- Pros: reuses store, API, revisions, masking, and the sync loop.
- Cons: couples add-ons to the resource lifecycle (statuses, sync columns); `Dependencies` are immutable and have no reverse lookup; tombstone and cascade rules get entangled with the parent.

**Identity model B: embedded in the parent spec.** Add-ons are config blocks inside the parent.

- Pros: simplest; one source of truth; no new table.
- Cons: cannot be managed, versioned or applied independently, which is a core goal.

**Chosen:** a separate table with its own API, a hybrid of the two: declared in the parent spec like B, stored and managed as independent records like A.

**Asynchronous reconciliation** via a dedicated worker loop would give retries, backoff and a visible `ERROR` state with less request latency. Rejected for v1; sync-completed trigger is the narrow exception for output-dependent apply.

**addon_revisions on day one:** rejected for v1 (schema, API, masking cost); defer.

## Rollout

Opt-in with zero migration. A parent with no `addons` entry and no module defaults behaves exactly as today. Empty `addons` and `addon_configs` tables at deploy.

## Implementation order

1. This RFC agreed.
2. Proton messages + `PROTON_COMMIT`.
3. Postgres schema (`addons`, `addon_configs`).
4. Core: registry, merge/reconcile, triggers on write + sync, origin rules.
5. HTTP handlers.
6. `logs` add-on + firehose descriptor + addon-config paths.

## Risks

- **Always-sync.** External calls run in request or sync path; slow APIs block callers. Mitigated by timeouts, default `on_failure: continue`, and idempotent manual PUT.
- **Two writers of desired state** (spec and API). Mitigated by `origin` and `spec_version` on API PUT.
- **Parent ERROR / PENDING.** Add-on rows may exist without successful external apply until parent completes or operator intervenes.
- **External orphans.** Soft-delete parent without prior add-on DELETE leaves external registrations until explicit add-on DELETE (if parent row still recoverable) or manual cleanup.
- **Force delete.** Hard parent delete without external add-on delete may orphan external state.
- **Tombstoned parents** require tombstone-inclusive getter for add-on delete teardown.

## Open Questions

- Whether `fail` add-ons should include a retry hint in the error response.
- Whether a future RFC adds `addon_revisions` and/or an opt-in background retry worker for add-ons.
- Hook execution order when multiple add-on types apply on the same sync-completed event (v1: undefined order; types must not depend on each other).
