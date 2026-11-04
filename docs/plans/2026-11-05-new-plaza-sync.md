# New API model plaza synchronization repair

> Execute in this session with focused regression tests and a staged production rollout.

**Goal:** Remove the confirmed stale public-group mirror and publish category-default models with their actual source pricing.

**Architecture:** Reuse category-default publication and invalidation checks rather than treating an entry as an account-owning group. Public snapshots must exclude exclusive sources, unpublished/conflicting/invalidated models and preserve source pricing identity. New must retain per-group display pricing so a default entry cannot overwrite ordinary model prices. The legacy channel is hidden only from plaza responses; its credential-bearing route and token remain intact.

**Tech stack:** Go, PostgreSQL, signed HTTP snapshots, Docker, GitHub Actions.

## Scope and decisions

- User approved fixing both diagnosed issues. No additional product/design confirmation is needed.
- Keep source groups, credentials, balances, order data and existing API behavior intact.
- Do not solve the default group by binding accounts or giving it independent prices.
- Use the existing category-default runtime validation rather than duplicating catalog rules in a SQL script.
- Prefer a targeted legacy reconciliation over deleting every untagged New channel; legitimate independent New configuration must remain supported.
- Live credential comparison established that New channel 20 points to Sub custom group 260/source 71, not renamed source 128. Therefore do not retag, migrate or delete channel 20/token 7. Configure a display-only channel-ID exclusion and preserve shared associations exposed by other channels.
- Keep application version 0.2.19; distinguish builds with commit IDs.
- Use wire version 2 for inherited source-price semantics. Deploy the New receiver accepting ordinary v1 and v2 before the Sub v2 producer; an old v1 receiver must reject v2 rather than applying source quotes globally.

## Tasks

1. Capture baseline and write failing Sub tests: category entry without accounts, multiple sources and source rates, exclusion of exclusive/stale/unpublished models, errors, source aliases.
2. Add category-default public route projection, snapshot dependency wiring and additive source-price metadata. Run focused `go test -tags=unit -p 2` checks for public-group sync, category-default and affected handlers/routes.
3. Inspect New's actual source and pricing pipeline; implement/test compatible display handling without changing independent channels' billing. Configure the confirmed legacy channel's display exclusion and verify hidden associations do not leak through pricing, group filters or ordering.
4. Review changes, build in an isolated environment (never on production), and deploy with the current runtime settings, mounts and networks preserved. Keep rollback artifacts.
5. Verify signed source snapshot, New pricing/model associations, source/default effective prices, stale names/models absence, token/channel consistency and both Sub entrances' health across repeated sync cycles.

## Rollback

Keep pre-change options and container configuration backups on the server and old containers/images. Restore previous images/routes if validation fails, rolling back the producer before the receiver. An older receiver safely rejects v2 snapshots. The exclusion can be reverted by removing its environment variable; channel/token data does not require a migration.
