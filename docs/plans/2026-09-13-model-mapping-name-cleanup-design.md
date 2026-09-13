# Model Mapping Upstream Name Cleanup Design

**Goal:** Let account mapping users normalize upstream model names by removing a shared prefix and/or suffix while preserving the existing request-name aliases.

**Scope:** Add a pure, immutable cleanup helper and UI controls to the existing reusable mapping toolbar used by Create/Edit account forms. Cleanup applies only to the right-side upstream (`to`) value. It remains an explicit batch action and writes the existing explicit `model_mapping` entries; no backend schema or wildcard semantics change.

**Interaction:** Add optional “remove upstream prefix” and “remove upstream suffix” inputs, a preview list showing changed and unchanged rows, and an apply button. Empty rules are no-ops; each rule removes at most one leading/trailing occurrence. Existing rows with blank values are left unchanged. If two rows would produce the same upstream target, keep both mappings and show a collision count so no manual mapping is silently changed or deleted. Applying twice is idempotent.

**Alternatives:** A backend dynamic rule format would avoid regenerating mappings when upstream models change, but would require changes to model support checks, routing, billing, and cache keys. Regex/capture-group mappings cannot be represented by the current `map[string]string` contract. Explicit client-side generation is safest and is consistent with the current toolbar.

**Validation:** Add Vitest coverage for prefix/suffix removal, whitespace trimming, empty rules, idempotence, and collision reporting. Extend the toolbar component tests for preview/apply and ensure existing prefix-generation behavior remains green.
