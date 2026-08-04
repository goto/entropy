# ADR: Update Resource Labels (Tags)

**Complexity:** S
**Priority:** Medium
**Assignee:** Muhammad Abduh
**Created:** 2026-07-21
**Status:** Draft
**MR:** [link once opened]

*(Legacy field retired 2026-08 — kept here for the historical record: Estimated Effort: ~1 day — not part of the current ADR template.)*

## Overview

**What:** A dedicated API to add, update, or remove labels ("tags") on an existing entropy resource without changing its spec or triggering a module re-plan/sync.
**Why:** Today labels can only change through `UpdateResource`/`ApplyAction`, both of which require a spec and drive the resource through the module planner and a sync cycle. Users need lightweight metadata edits (ownership/team/env tags) that don't disturb the running workload.
**Success Metric:** Labels can be patched via a single call; the resource's spec and `state.status` are unchanged and no sync is triggered, while an audit revision is recorded.

## Requirements

### User Story
**As an** entropy user/operator
**I want** to create, update, or delete a label on a resource independently of its spec
**So that** I can manage resource metadata without redeploying or re-syncing the workload.

### Acceptance Criteria
- [ ] `PATCH /v1beta1/resources/{urn}/labels` with `{"labels": {...}}` merges the provided keys into the resource's existing labels (add/overwrite).
- [ ] A key sent with an empty string value deletes that label (merge/patch semantics, consistent with existing `mergeLabels` in `core/write.go:174-193`).
- [ ] Untouched existing labels are preserved.
- [ ] The operation does not modify the spec, does not change `state.status`, and does not increment the pending counter or trigger a sync.
- [ ] The operation is permitted in any resource state (no terminal-state guard).
- [ ] A revision is recorded with reason `"update labels"` (spec snapshot + new labels).
- [ ] Unknown URN returns NotFound.

### Constraints
- **Technical:** Proto source lives in `goto/proton` (https://github.com/goto/proton); entropy generates from a pinned `PROTON_COMMIT`. The proton change must be merged to `main` before entropy regenerates. `make proto` fetches `https://github.com/goto/proton/archive/${PROTON_COMMIT}.zip`.
- **Business:** Label semantics must stay consistent with the existing `mergeLabels` convention (empty value = delete).
- **Timeline:** None.

## Implementation Plan

### Prerequisites
- [ ] Merge access to `goto/proton` (https://github.com/goto/proton) and `goto/entropy` (https://github.com/goto/entropy).
- [ ] Local Postgres + protobuf toolchain available (`make setup` / `make proto`; buf + protoc-gen-* installed per Makefile).

### Core Tasks

1. **Proton RPC (PR 1 → goto/proton)** - Add the `UpdateResourceLabels` RPC and its request/response messages.
   - **Estimate:** 1h
   - **Details:** In `gotocompany/entropy/v1beta1/resource.proto` (local checkout `/Users/muhammad.abduh/repo/goto/proton/gotocompany/entropy/v1beta1/resource.proto`), add to `service ResourceService`, mirroring the `UpdateResource` HTTP style:
     ```proto
     rpc UpdateResourceLabels(UpdateResourceLabelsRequest) returns (UpdateResourceLabelsResponse) {
       option (google.api.http) = {
         patch: "/v1beta1/resources/{urn}/labels"
         body: "*"
       };
     }
     ```
     Add messages near the existing `UpdateResource*` messages:
     ```proto
     message UpdateResourceLabelsRequest {
       string urn = 1;
       map<string, string> labels = 2;
     }

     message UpdateResourceLabelsResponse {
       Resource resource = 1;
     }
     ```
     Open a PR against `goto/proton`, merge to `main`, and note the resulting commit SHA for PR 2.

2. **Regenerate protobuf (PR 2 → goto/entropy)** - Bump the pinned proton commit and regenerate.
   - **Estimate:** 0.5h
   - **Details:** Bump `PROTON_COMMIT` in `Makefile:4` to the merged `goto/proton` SHA. Run `make proto` to regenerate `proto/gotocompany/entropy/v1beta1/resource.pb.go`, `resource.pb.gw.go`, `resource_grpc.pb.go`, and `proto/entropy.swagger.yaml`, adding `UpdateResourceLabels` + the two messages.

3. **Store layer** - Add a labels-only persistence method.
   - **Estimate:** 2h
   - **Details:**
     - Interface: add to `resource.Store` in `core/resource/resource.go:20-31`:
       ```go
       UpdateLabels(ctx context.Context, r Resource, saveRevision bool, reason string, hooks ...MutationHook) error
       ```
     - Postgres impl: add `Store.UpdateLabels` in `internal/store/postgres/resource_store.go`, modeled on `Update` (lines 206-269) but **only**:
       - update `updated_at`/`updated_by` on `tableResources` (NOT `spec_configs` or any `state_*` column),
       - call existing `setResourceTags(ctx, tx, id, r.Labels)` (`resource_store.go:434`) — already does full delete-then-reinsert of the merged label set,
       - if `saveRevision`, call existing `insertRevision` with `r.Spec` + `r.Labels` + `reason`.
       - Do **not** touch dependencies or state. Reuses `withinTx`, `translateURNToID`, `setResourceTags`, `insertRevision`.

4. **Core service** - Add the labels-only service method.
   - **Estimate:** 1.5h
   - **Details:** Add `Service.UpdateResourceLabels` in `core/write.go`:
     ```go
     func (svc *Service) UpdateResourceLabels(ctx context.Context, urn string, labels map[string]string, userID string) (*resource.Resource, error)
     ```
     Behavior:
     1. `res, err := svc.GetResource(ctx, urn)` (returns `ErrNotFound` as today; **no** terminal-state guard).
     2. `res.Labels = mergeLabels(res.Labels, labels)` — reuse existing `mergeLabels` (merge + empty-value deletion).
     3. `res.UpdatedBy = userID`; `res.UpdatedAt = svc.clock()`.
     4. `svc.store.UpdateLabels(ctx, *res, true, "update labels")`.
     5. Return the updated `res`. Does **not** call `planChange`/`PlanAction`, does **not** increment the pending counter, does **not** change `State`.

5. **gRPC handler** - Wire the RPC into the server.
   - **Estimate:** 1h
   - **Details:** In `internal/server/v1/resources/server.go`:
     - Add to the `ResourceService` interface (lines 15-26): `UpdateResourceLabels(ctx context.Context, urn string, labels map[string]string, userID string) (*resource.Resource, error)`.
     - Add handler method `UpdateResourceLabels(ctx, *entropyv1beta1.UpdateResourceLabelsRequest)` modeled on `UpdateResource` (lines 67-97): resolve `userIdentifier` via `serverutils.GetUserIdentifier(ctx)`, call `server.resourceSvc.UpdateResourceLabels(...)`, map result with `resourceToProto`, return `&entropyv1beta1.UpdateResourceLabelsResponse{Resource: ...}`, errors via `serverutils.ToRPCError`.

6. **Regenerate mocks** - Refresh generated mocks for the new interface methods.
   - **Estimate:** 0.5h
   - **Details:** Run `go generate ./...` to regenerate `core/mocks/resource_store.go` (`//go:generate` at `core/resource/resource.go:3`) and `internal/server/mocks/resource_service.go` (`//go:generate` at `server.go:3`).

### Testing Tasks
- [ ] **Unit Tests:** Core table test for `UpdateResourceLabels` — merge adds/overwrites keys, empty value deletes a key, `PlanAction` is never invoked, state unchanged, `UpdateLabels` store mock called with `saveRevision=true` and reason `"update labels"`. Server handler test mirroring existing `UpdateResource` handler tests.
- [ ] **Integration Tests:** Postgres `UpdateLabels` persists tags + revision and leaves `spec_configs`/`state_*`/dependencies untouched.
- [ ] **Manual Testing:** PATCH labels endpoint add/update/delete; verify revisions recorded and `state.status` unchanged.

### Documentation Tasks
- [ ] **Code Comments:** Document that `UpdateResourceLabels` deliberately bypasses the module planner and sync.
- [ ] **README Updates:** Note the new endpoint if the API surface is documented in the repo README.
- [ ] **API Docs:** Regenerated `proto/entropy.swagger.yaml` covers the new route.

## Technical Notes

### Codebase Dependencies

| Repo | Path (`:line`) | Role / why it matters |
|------|----------------|------------------------|
| `github.com/goto/entropy` | `core/write.go:174-193` (`mergeLabels`) | Existing merge/patch helper (empty value deletes a key) that `UpdateResourceLabels` reuses for consistent label semantics. |
| `github.com/goto/entropy` | `core/resource/resource.go:20-31` (`resource.Store` interface) | Interface `UpdateLabels` is added to. |
| `github.com/goto/entropy` | `internal/store/postgres/resource_store.go:206-269` (`Store.Update`) | Existing update method the new `Store.UpdateLabels` is modeled on — same shape, but must skip `spec_configs`/`state_*` columns. |
| `github.com/goto/entropy` | `internal/store/postgres/resource_store.go:434` (`setResourceTags`) | Existing delete-then-reinsert tag persistence, reused unchanged by `UpdateLabels`. |
| `github.com/goto/entropy` | `internal/server/v1/resources/server.go:67-97` (`UpdateResource` handler) | Existing gRPC handler the new `UpdateResourceLabels` handler is modeled on (user identifier resolution, error mapping). |
| `github.com/goto/proton` | `gotocompany/entropy/v1beta1/resource.proto` | Proto service definition; the new RPC + request/response messages must be added and merged here (PR 1) before entropy can regenerate. |
| `github.com/goto/entropy` | `Makefile:4` (`PROTON_COMMIT`) | Pinned proton commit; must be bumped to the merged proton SHA before `make proto` regenerates the new RPC (PR 2). |

### Files to Modify

**PR 1 — goto/proton**
- `gotocompany/entropy/v1beta1/resource.proto` - new RPC + `UpdateResourceLabelsRequest`/`UpdateResourceLabelsResponse` messages.

**PR 2 — goto/entropy**
- `Makefile` - bump `PROTON_COMMIT` to the merged proton SHA.
- `proto/gotocompany/entropy/v1beta1/*.pb.go`, `*.pb.gw.go`, `*_grpc.pb.go`, `proto/entropy.swagger.yaml` - regenerated.
- `core/resource/resource.go` - add `UpdateLabels` to the `Store` interface.
- `internal/store/postgres/resource_store.go` - `UpdateLabels` Postgres impl.
- `core/write.go` - `Service.UpdateResourceLabels`.
- `internal/server/v1/resources/server.go` - interface method + gRPC handler.
- `core/mocks/resource_store.go`, `internal/server/mocks/resource_service.go` - regenerated.

### Dependencies
- **Internal:** `goto/proton` proto definitions (must merge first); existing `mergeLabels` (`core/write.go`), `setResourceTags`/`insertRevision` (`internal/store/postgres`).
- **External:** buf, protoc-gen-go, protoc-gen-go-grpc, protoc-gen-grpc-gateway, protoc-gen-openapiv2 (installed via Makefile targets).

### Risks & Mitigation
- **Risk:** Reusing `Store.Update` would clobber spec/state columns.
  **Mitigation:** Dedicated `UpdateLabels` that writes only labels + `updated_at`/`updated_by`.
- **Risk:** Proto/entropy version skew from the pinned commit.
  **Mitigation:** Merge the proton PR to `main` first, then bump `PROTON_COMMIT` and regenerate.
- **Risk:** Callers expect full-replace semantics.
  **Mitigation:** Documented merge/patch semantics (empty value deletes), consistent with existing label behavior.

## Definition of Done

- [ ] All acceptance criteria are met
- [ ] Code is reviewed and approved
- [ ] Tests are written and passing
- [ ] Documentation is updated
- [ ] Feature is deployed to staging
- [ ] Manual testing is complete
- [ ] Stakeholder approval received

## Rollback Plan

**If something goes wrong:**
1. Revert the entropy PR (handler, core, store, generated code, `Makefile` bump).
2. Optionally revert the proton PR; the removed endpoint is additive and unused elsewhere.
3. Notify the team in the entropy channel; no data migration is involved (labels use the existing `resource_tags`/`revision_tags` tables).
