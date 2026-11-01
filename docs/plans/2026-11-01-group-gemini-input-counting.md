# Group Gemini input discount implementation plan

**Goal:** Let administrators enable the existing Vertex-like input discount for a selected group from the group form.

**Architecture:** Persist `gemini_input_counting_enabled` as a default-false group boolean and expose it only in administrator group DTOs. Gemini and ordinary composite groups support the switch. The request's frozen billing group controls eligibility; scheduler fallback/source groups and shared account objects cannot override it. A bound group with the switch off overrides legacy account opt-ins. Ungrouped legacy calls keep the exact-model account setting. All existing pure-text, measured-usage, explicit-zero-cache, customer-cost ceiling and raw upstream-cost rules remain in force.

**Tech stack:** Go, Ent/PostgreSQL, Redis authentication snapshots, Vue/TypeScript and Vitest.

### 1. Persistence and administration

- Add migration 258, the Ent group field and generated code.
- Carry the value through service create/update/duplicate, repositories, administrator requests/responses and API-key authentication selects/snapshots.
- Reject enabling on unsupported platforms, preserve omitted update values, require explicit false when switching away from supported platforms, and invalidate existing group auth caches.
- Verify creation, true/false/omission updates, duplication, restricted DTO visibility, real database reads and cache round trips.

### 2. Request isolation and accounting

- Freeze eligibility at `BeginGeminiUpstreamAttempt` using the original billing group; keep only a request-local boolean and retain no mutable shared group/account configuration.
- Feed it into final-request preparation. Keep native count/calibration recovery unchanged for group opt-ins; legacy exact-model account configuration keeps its channel-specific exclusions. System-custom subscription containers remain ineligible rather than inheriting a source group switch.
- Verify an enabled and disabled group sharing one account, frozen parent versus composite/fallback source, missing group data, unsupported accounts, strict body guards, mapped final model and all three response protocols.
- Re-run customer billing/raw account cost and quota regression tests.

### 3. Group form

- Add an independent neutral control in new/edit forms for Gemini/composite groups, default off.
- Carry it through loading, reset, platform changes and save; keep simple mode and ordinary user DTOs unchanged.
- State that only supported pure-text Gemini API-key calls receive a customer-side input discount, output and supplier cost stay unchanged, and tools/media/cache retain original accounting.
- Verify actual form interaction/save behavior, locale/type checks and production build.

### 4. Review and local delivery

- Run focused backend unit and PostgreSQL integration tests, frontend behavior tests, lints and builds, then independently review the full diff.
- Commit and merge into local `dev`; keep official version `0.2.19`.
- Back up the local group configuration before the additive migration and update the isolated original-data preview. Do not enable any real group automatically, push, or deploy production.
