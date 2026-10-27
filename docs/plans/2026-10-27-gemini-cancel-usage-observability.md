# Gemini canceled request usage repair

**Goal:** Settle Gemini text strictly from complete provider measurements, retain unmeasured canceled requests in normal usage history as pending review, and keep estimation diagnostics out of ordinary user views.

**Approved scope (updated 2026-10-27):** The user explicitly selected: “按上游准确 usage 计费；未收到则标记待核对，不猜测扣费”. Complete input and candidate-output field presence (including explicit zero) permits settlement even if delivery fails or no answer reaches the client. Optional provider thought tokens remain part of output. Missing input or candidate-output measurement prevents wallet/quota/dedup mutation; local output tokenization is diagnostic only. Images and other platforms retain existing policy. Historical balances are not changed; deployment remains a separate action.

**Architecture:** The three Gemini protocols share a measured-usage gate before any accounting write. Cancellation can recover bounded metadata only from the current actual upstream attempt, guarded by attempt sequence/account/platform. A separate `usage_pending_records` store uses the same request/API-key identity and no cost/dedup writes. Normal history uses a filtered, paginated union with negative pending IDs and explicit `billing_status=pending_review`; a matching settled usage row suppresses its pending row. Statistics remain based on settled usage. Ordinary users see unknown tokens as “—” and pending fees as “待核对”; admin views retain per-field provenance and Ops delivery diagnostics. Missing measurements are not inferred from screenshots or local text, and no ledger reconciliation API is invented.

**Tech stack:** Go/Gin/PostgreSQL, existing durable usage outbox and Ops queue, Vue/TypeScript and Vitest.

## Implementation and verification

1. Test provider measurements vs locally delivered output across Chat, Messages and native Gemini, including metadata-only cancellation, thought-only output, write failure, missing fields, explicit zero and account failover.
2. Preserve provenance through response metadata and durable outbox. Source-only metadata must never imply response outcome evidence.
3. Gate settlement before wallet/dedup, route incomplete measurement into separate pending persistence, and preserve final attempt identity.
4. Test pending upsert, ownership, detail, filters, count/pagination and replacement by late settlement in real PostgreSQL; do not count pending as settled revenue.
5. Keep normal user history complete while removing estimation diagnostics and canceled-request placement from the ordinary error list. Admin Ops can retain technical diagnostics without changing SLA health definitions.
6. Run affected backend suites, database integration, frontend tests/typechecks/build and independent review. Previously recorded validation below predates the policy change; rerun before completion.

Publication requires the normal release workflow. A successful local verification does not mean production has changed.

## Durable payload compatibility and release checks

New writers emit usage-persistence payload version 2. Readers accept the strict deployed version 1 format and version 2; legacy version 1 has `unknown` provenance, and its schema does not acquire the two version 2 source keys. Unsupported versions fail delivery before usage insertion or acknowledgment. The deployed version 1 worker was inspected at commit `cf3a8cb69`: a decode failure rolls back the delivery transaction, then `RetryClaimed` retains the row and releases its claim with a bounded retry delay. If retry or shutdown interrupts that update, the existing 60-second claim lease makes it recoverable. There is no path that acknowledges an undecodable row.

A mixed deployment can still delay persistence: old workers may claim version 2 snapshots, reject them, and schedule another attempt. The worker starts automatically and currently has no supported configuration switch to disable only that worker; do not invent an environment flag for deployment. Use the following release sequence:

1. Apply migrations 253 and 254 through the normal application migration process and start the new API/direct instances. Verify health and that their workers can read both payload versions before sending new inference traffic.
2. Switch both public entrances to the new instances, allow requests on the old instances to drain, then stop the old application containers. Preserve their images and deployment parameters; leave PostgreSQL and Redis running. Keeping old workers running indefinitely would let them repeatedly defer version 2 delivery.
3. Keep the new workers available until pending snapshots have drained, including claimed rows and retries. Check the pending count, oldest creation time, claim count and attempts by payload version. A zero available-now count is insufficient because leased or delayed rows remain pending.
4. Before any rollback, stop routing new traffic to the version 2 writers and drain their in-flight requests. Keep at least one new instance running on its private port until all version 2 outbox rows, including claims, have been delivered. Never delete pending rows or relabel their version to force compatibility.
5. Use a compatibility rollback build containing the version 1/2 reader and the `collector_version > 0` evidence guards. An older binary can misclassify the new provenance-only response rows as empty responses even after the outbox is drained; those rows intentionally use collector version 0 and are not response evidence. The guards are required in model health, the activity heatmap and empty-response claim evidence queries.

Read-only pending-work check (run against the actual deployment database during release verification):

```sql
SELECT payload->>'version' AS payload_version,
       COUNT(*) AS pending,
       COUNT(*) FILTER (WHERE claimed_by IS NOT NULL) AS claimed,
       MIN(created_at) AS oldest_pending,
       MAX(attempts) AS max_attempts
FROM usage_persistence_outbox
GROUP BY payload->>'version';
```

The local repair does not execute this release procedure or alter historical financial records.


## Local verification (updated policy)

- Full affected backend unit packages passed across the final runs: service, handler, admin handler, DTO, repository and migrations. After recovery metadata and terminal-error adjustments, all Gemini service regressions passed again; handler billing tests cover the three real protocol adapters.
- Real PostgreSQL integration passed for pending history, provenance read paths, source-only analytics/claims, and Ops final-attempt behavior. Pending coverage verifies ownership, filters, pages, absence of financial/dedup writes, idempotence, settlement replacement, batch/outbox-compatible triggers, and two forced concurrent write orders followed by retention without dedup protection.
- Pending persistence uses a READ COMMITTED transaction and a separate advisory-lock statement. Actual usage INSERTs use a matching shared advisory lock; resolution is recorded by an AFTER INSERT trigger. Projection SQL types and INSERT SELECT parameter types match the actual schema.
- Frontend regression tests, application/test TypeScript checks, changed-file lint and production build passed. Ordinary user and admin usage error tabs exclude final cancellations; dedicated admin Ops retains diagnostics.
- No historical token backfills, balance adjustments, production operations, commits or pushes were performed. Historical missing measurements still need an authoritative upstream ledger integration for reconciliation; this change does not invent one.
- Local validation temporarily exhausted disk space. Only verified, reproducible old Sub2API Go build artifacts were cleared; the failed local Docker Desktop runtime was recovered, and the database tests then ran successfully.
