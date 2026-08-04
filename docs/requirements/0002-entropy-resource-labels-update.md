# Requirements: Entropy Resource Labels Update

**Complexity:** S — single new RPC in one system, reusing existing `mergeLabels`/persistence helpers; only open questions are naming conventions, not design.

## 1. Core Intent & Value

### 1.1 The Ultimate Goal
Let operators edit resource labels without triggering re-plan/re-sync.

### 1.2 The Problem Statement
Labels only change via `UpdateResource`/`ApplyAction`, which requires a full spec and drives a sync cycle — even for a metadata-only edit.

### 1.3 Target Audience
Entropy operators/platform users managing ownership/team/env tags on live resources.

---

## 2. User Experience & Workflow (The "Happy Path")

1. **Trigger**: Operator wants to add/change labels on a resource.
2. **Action**: PATCH labels only — no spec required.
3. **Reaction**: Labels merge immediately; spec and state untouched; no sync triggered.
4. **Success**: Labels updated, resource keeps running, change is audited.

---

## 3. High-Level Rules & Constraints

* **Business**: Merge semantics (empty value = delete) match existing behavior; no reconciliation triggered; every change recorded as a revision.
* **Data Fields**: `urn`, `labels` (map, empty value = delete key).
* **Regs**: None.

---

## 4. Exceptions & "What Ifs"

* **Unknown URN** → NotFound.
* **Delete a label that doesn't exist** → no-op (idempotent).
* **Resource in error/terminal state** → allowed anyway (no state guard).
* **Label vs. spec conflict** → impossible; labels are independent of spec.

---

## 5. Success Metrics

- [ ] Labels patchable in one call, no spec needed.
- [ ] Spec/`state.status` unchanged; no sync triggered.
- [ ] Revision recorded per change.
- [ ] Works on any resource state.

---

## 6. Out of Scope

- [ ] Label validation/schema.
- [ ] Batch updates across resources.
- [ ] Label templating/inheritance.

---

## 7. Codebase Dependencies & References

| Repo | Path (`:line`) | Why it's relevant |
|------|----------------|--------------------|
| `github.com/goto/entropy` | `core/write.go:174-193` | `mergeLabels` — reused merge/delete semantics |
| `github.com/goto/entropy` | `internal/store/postgres/resource_store.go` | `setResourceTags`/`insertRevision` — persistence to reuse |
| `github.com/goto/proton` | `gotocompany/entropy/v1beta1/resource.proto` | New `UpdateResourceLabels` RPC lives here |

* **Related spec**: `goto-de-wiki:wiki/entropy/entropy-current-state.md`

---

## 8. Unresolved Questions

* API verb (PATCH) — confirm team convention.
* Revision reason string — generic or itemized?
* Should label updates emit webhooks/events?
