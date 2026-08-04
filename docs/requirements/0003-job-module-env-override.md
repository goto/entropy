# Requirements: Job Module Env-Variable Override (Dex-safe Masking Companion)

**Complexity:** S — single component (job module `Plan`), reuses the existing `CloneAndMergeMaps` merge pattern with precedence flipped; no new integration points.

## 1. Core Intent & Value

### 1.1 The Ultimate Goal
A Dex-created job runs with real secrets, even though masking now returns placeholders on read.

### 1.2 The Problem Statement
Dex reads a masked firehose resource and dumps its (placeholder) values into a new job's env vars on `Create`. Masking's `Restore` only helps on `Update` — on `Create` there's no stored value, so the masked-form input is dropped, leaving no real secret.

### 1.3 Target Audience
Platform operators running Dex-driven job creation; any job workload depending on a firehose-to-job Dex handoff.

---

## 2. User Experience & Workflow (The "Happy Path")

1. **Trigger**: Dex creates a job, re-dumping masked placeholder values into container env vars.
2. **Action**: Job module config declares `containers.{name}.env_variables` overrides with real secret values.
3. **Reaction**: On `Create`/`Update`, Entropy overlays those values per-key onto the matching container, module always wins, before planning.
4. **Success**: Job runs with real secrets in every container; no placeholders reach the driver; unrelated keys pass through unchanged.

---

## 3. High-Level Rules & Constraints

* **Business**: For keys declared in module config, module wins regardless of whether the client value is masked or real. Secret values must come from secret manager `${...}` placeholders, never hardcoded. Independent of masking's `sensitive_configs`.
* **Data Fields**: `configs.containers.{container_name}.env_variables` (per-container override map).
* **Regs**: None beyond existing secret-management conventions.

---

## 4. Exceptions & "What Ifs"

* **Container name has no override entry** → untouched, no error.
* **Override names a container absent from spec** → ignored.
* **`containers` map empty/absent** (today's default) → no-op, backward compatible.
* **Client sends a real value for an overridden key** → module config still wins.
* **Container's `env_variables` is nil** → created and populated, not an error.

---

## 5. Success Metrics

- [ ] Dex-created job runs with real secrets in every container.
- [ ] No `****-` placeholder reaches the driver.
- [ ] Existing jobs with empty configs unaffected.
- [ ] Non-overridden keys pass through unchanged.

---

## 6. Out of Scope

- [ ] Validation beyond basic type checking.
- [ ] Changes to masking's `sensitive_configs` behavior.
- [ ] Wholesale replacement of a container's `env_variables` map.

---

## 7. Codebase Dependencies & References

| Repo | Path (`:line`) | Why it's relevant |
|------|----------------|--------------------|
| `github.com/goto/entropy` | `modules/job/config.go` | Job driver config struct — override field added here |
| `github.com/goto/entropy` | `modules/job` driver `Plan` | Where the per-key overlay is applied |
| `github.com/goto/entropy` | `modules/utils.go:18-27` | `CloneAndMergeMaps` — existing merge pattern, precedence flipped |
| `github.com/goto/dex` | `entropy/job.go:29` | Confirms env vars are per-container, keyed by name |

* **Related spec**: `goto-de-wiki:wiki/entropy/entropy-current-state.md`
* **Related ADR**: [mask-sensitive-values-in-api-response](../adr/0001-mask-sensitive-values-in-api-response.md) (the ADR this is a companion to)

---

## 8. Unresolved Questions

* Generalize this override to other modules (e.g. firehose) facing the same Dex-create problem?
* Note: ships alongside/after masking — depends on that design being finalized first.
