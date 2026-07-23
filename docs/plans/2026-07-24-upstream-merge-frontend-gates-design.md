# Upstream Merge Frontend Gates Design

## Context

Merging upstream `v0.1.164` preserved the local frontend quality gates while reintroducing a small set of upstream UI fragments and locale changes that do not satisfy them. The production build succeeds, but 12 assertions fail across API contract, locale completeness, and shared admin surface tests.

## Decision

Keep the upstream runtime behavior and features. Update the stale rollback API assertions to include the intentional 15-minute request timeout, add the missing Chinese account messages and error-detail messages in both locales, and replace only the legacy surface fragments reported by the existing tests with the established shared admin classes.

## Boundaries

- Do not change rollback runtime behavior.
- Do not weaken or remove quality-gate assertions.
- Do not revert upstream features.
- Do not change backend, database, deployment, or production configuration.
- Do not deploy as part of this repair.

## Verification

Run the affected Vitest files first, then frontend lint, typecheck, full tests, and production build. Re-run backend tests and the embedded server build before declaring the branch deployable.
