# Requirements: Mask Sensitive Values in Entropy API Responses

**Complexity:** L — new cross-cutting subsystem (`pkg/masking/`) touching eight endpoints (six resource, two module) across the server, write-path, and module-validation layers, with security-sensitive design decisions (HMAC fingerprinting, fail-open policy) and a dependent companion change.

## 1. Core Intent & Value

### 1.1 The Ultimate Goal
No secret leaves Entropy in cleartext via the API, while consumers can still detect when a credential changes.

### 1.2 The Problem Statement
`spec.configs` and module `configs` expose secrets (passwords, credentials, tokens) verbatim in API responses.

### 1.3 Target Audience
Platform operators/security owners; indirectly all API consumers (UI, CLI, Dex, automation).

---

## 2. User Experience & Workflow (The "Happy Path")

1. **Trigger**: Client reads a resource or module.
2. **Action**: Entropy resolves `sensitive_configs` paths for that kind/project.
3. **Reaction**: Response returns `****-<fingerprint>` in place of sensitive values; everything else is untouched.
4. **Success**: Client sees masked-but-distinguishable secrets; resending a masked value on `Update` never clobbers the real one.

---

## 3. High-Level Rules & Constraints

* **Business**: `sensitive_configs` is a per-kind policy; masking is response-boundary only — reconciliation always sees real values; round-trip on `Update` must never overwrite a real secret.
* **Data Fields**: `sensitive_configs: []string` (dot-path, optional trailing `*`); HMAC key from env/secret manager, never hardcoded.
* **Regs**: Security-driven — no sensitive value in cleartext in any response.

---

## 4. Exceptions & "What Ifs"

* **`ListResources` without `with_spec_configs`** → masking skipped entirely (spec.configs is not hydrated by the store, so there is nothing to mask). Masking runs on `ListResources` only when `with_spec_configs=true`.
* **Module not found (orphaned resource)** → fail open: unmasked + warning logged.
* **Masked value resent on `Update`** → stored real value restored.
* **Masked value sent on `Create`** (nothing stored yet) → field dropped, not persisted, not an error.
* **HMAC key rotated** → all fingerprints change (accepted, documented tradeoff).
* **Masking needs to be disabled** → empty/absent HMAC key disables masking everywhere.

---

## 5. Success Metrics

- [ ] No sensitive value in cleartext in `spec.configs` (six resource endpoints) or module `configs` (two module endpoints); `ListResources` masks only when `with_spec_configs=true`.
- [ ] UI round-trip (Get → edit → Update) never clobbers a real secret.
- [ ] Fingerprint changes when the credential changes.
- [ ] Drivers always receive real values.

---

## 6. Out of Scope

- [ ] `resource.state.output` masking (sensitive paths target `spec.configs`; output masking is not needed).
- [ ] `state.module_data`.
- [ ] `GetLog` streaming (flagged risk, not fixed here).
- [ ] At-rest encryption (secrets stay cleartext in Postgres).
- [ ] Array indexing in path syntax.
- [ ] Validating that `sensitive_configs` paths actually resolve.

---

## 7. Codebase Dependencies & References

| Repo | Path (`:line`) | Why it's relevant |
|------|----------------|--------------------|
| `github.com/goto/entropy` | `internal/server/v1/resources/mappers.go` | Mask applied in `resourceToProto`/`revisionToProto` |
| `github.com/goto/entropy` | `internal/server/v1/modules/mappers.go` | Mask applied in `moduleToProto` |
| `github.com/goto/entropy` | `core/write.go` | Write-path choke point (`planChange`) restoring/dropping masked fields |
| `github.com/goto/entropy` | `core/module/service.go` | `sensitive_configs` syntax validation on `Create`/`Update` |
| `github.com/goto/entropy` | `pkg/masking/` (new) | Masker: mask, restore, fingerprint, path validation |

* **Related spec**: `goto-de-wiki:wiki/entropy/entropy-current-state.md`
* **Related ADR**: [job-module-env-override-and-kube-driver-fix](../adr/0003-job-module-env-override-and-kube-driver-fix.md) (Dex `Create` client impact)

---

## 8. Unresolved Questions

* Bring `GetLog` streaming into masking scope later?
* Pursue at-rest encryption as a follow-up?
* Note: write-path was simplified post-implementation (single `RestoreSecrets` helper, masker-owned cache) — see change record's "Simplification Pass."
