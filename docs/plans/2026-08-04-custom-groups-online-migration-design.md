# Custom Groups Online Migration Design

## Goal

Make the custom-group schema rollout safe for a busy production database so a blue-green application deployment does not block writes to `usage_logs`.

## Context

Production's `usage_logs` relation is approximately 5.6 GB and continues to receive writes. Migration `194_user_custom_model_groups.sql` currently creates a regular index on that table and validates a new foreign key immediately. Both operations are unsuitable for the zero-downtime release path.

## Design

Keep table and nullable-column creation in the transactional migration. Add foreign keys and the single-group check constraint as `NOT VALID`, which enforces them for new writes without scanning existing rows during startup. Move all new indexes into a following `_notx.sql` migration and create them with `CREATE INDEX CONCURRENTLY IF NOT EXISTS`.

The application does not require the constraints to be validated before serving traffic because all new custom-group fields begin as `NULL`, and PostgreSQL enforces `NOT VALID` constraints for new or changed rows. Validation can be performed later as a separate low-impact maintenance operation.

## Verification

Add a migration regression test that requires the transactional migration to contain no index creation, requires each hot-path constraint to use `NOT VALID`, and requires the separate migration to use concurrent, idempotent index creation. Run focused migration tests, all migration tests, full backend tests, frontend type checking/tests/build, and deployment smoke tests.

## Rollout

After verification and push, back up PostgreSQL and verify the dump. Transfer a clean source archive at the pushed commit, build a new inactive-slot image, start it against the shared PostgreSQL and Redis, and verify it directly. Atomically update both Nginx production sites only after health and smoke checks pass. Keep the previous active container available for immediate proxy rollback.
