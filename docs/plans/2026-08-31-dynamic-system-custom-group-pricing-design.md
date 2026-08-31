# Dynamic System Custom Groups And Pricing Design

## Goal

Simplify system custom subscription groups so administrators select source groups instead of maintaining individual model routes. The custom subscription group must automatically follow model additions, removals, and renames in its selected source groups. At the same time, adding a model to a source group must detect missing pricing and let the administrator complete pricing in the same form and save operation.

## Current Behavior

- A system custom subscription group persists one `system_custom_group_models` row for every public/source model route.
- The administrator selects source groups, then manually selects models and maintains aliases.
- A later model change in a source group is not reflected until the custom group is manually synchronized.
- Duplicate public model names are rejected, so several source groups cannot provide ordered fallback routes for the same model.
- Source-group model visibility and `model_pricing` are separate form sections. A newly added model can be saved without an effective price and later fail admission with `pricing not found`.
- Production data currently contains no model aliases in system custom groups, so changing to real source model names does not rename any active public model.

## Considered Approaches

### Persist A Refreshed Model Snapshot

Keep the current route table and rebuild it whenever a source group changes. This minimizes runtime changes, but every source-group mutation path must emit a reliable synchronization event. Missed events, bulk imports, and direct database changes can leave stale routes. It also keeps the manual snapshot concept the feature is intended to remove.

### Resolve Selected Source Groups Dynamically

Persist only ordered source-group references. Derive the current public model catalog and route candidates from live source groups at list and request time. This makes source changes immediately authoritative and naturally supports duplicate model names as fallback candidates. It requires bounded runtime catalog queries and careful cache invalidation, but it has one source of truth and is the selected approach.

### Copy Pricing Automatically From A Template

When a model has no price, copy another model's pricing without administrator review. This is quick but financially unsafe because similarly named models can have materially different input, output, cache, image, or per-request rates. Template copy may remain an optional convenience, but copied values must be reviewed before save.

## Selected Data Model

Add `system_custom_group_sources` with:

- `group_id`: the system custom subscription container.
- `source_group_id`: a direct, non-system-custom source group.
- `priority`: deterministic administrator-selected fallback order.
- timestamps.

The pair `(group_id, source_group_id)` is unique. The pair `(group_id, priority)` is unique. A custom group must have at least one source and cannot reference itself, another system custom group, an inactive/deleted group, or an unsupported source group.

Keep `system_custom_group_models` during rollout. Existing rows remain a rollback snapshot and support compatibility while every runtime caller is moved to dynamic resolution. New create/update requests write source references; they do not require or regenerate per-model rows.

## Migration

For every existing system custom group, extract distinct `source_group_id` values from its route rows. Preserve the first existing route occurrence as source priority. Insert those references idempotently.

The migration does not change group IDs, names, limits, subscriptions, API keys, usage logs, orders, or balances. Existing route rows are not deleted in the first release. Because production has no aliases, the dynamic public model names match current source model names.

If an old custom group has no valid source reference after migration, leave its old routes intact and report it as a migration warning rather than silently creating an unusable dynamic group.

## Dynamic Model Catalog

The public catalog is the case-insensitive union of current models exposed by all selected source groups. The catalog includes only routes that are:

- attached to an active and valid source group;
- allowed by that source group's model-list configuration;
- supported by at least one schedulable account using the existing gateway predicates; and
- accepted by the same effective pricing admission logic used before upstream dispatch.

Model names are public source names. There is no per-model alias editor.

When several selected source groups expose the same case-insensitive model name, return it once in `/models`. Keep an ordered list of route candidates by source priority. At request time, choose the first candidate that is currently valid, priced, and schedulable. If it has no eligible account, continue to the next source candidate.

Existing account-level and group-level retries continue after a source is chosen. Do not replay a request across a second source after an upstream may have accepted it, because that can duplicate side effects or billing.

When all candidates are unavailable, return the existing system custom source-unavailable error. When the model does not exist in any selected source, return the existing model-not-allowed error.

## Pricing-Aware Source Group Editing

Model selection and pricing remain part of one source-group create/update payload, but the interface treats them as one workflow:

1. The administrator adds or selects models.
2. The form compares those models with existing explicit group pricing and the effective base/fallback pricing resolver.
3. Models with effective prices show `Priced` and require no duplicate entry.
4. Missing models are automatically added to a `Pricing required` section.
5. The administrator enters token, request, image, or video pricing in the same dialog. Optional template copy pre-fills fields but does not bypass review.
6. One save submits the model-list and pricing changes without a page reload.

Newly added models without effective pricing cannot be published. The backend repeats the pricing-coverage validation so a stale or custom client cannot bypass the rule. Existing legacy models that were already unpriced remain visible to administrators as warnings, but dynamic subscription groups expose only valid and billable routes.

The effective-price check must use the gateway's pricing semantics rather than duplicating field checks in the browser. It accounts for explicit group pricing, channel overrides, the LiteLLM catalog, configured fallback pricing, billing mode, and normalized model names.

## Administration API

System custom group create/update requests replace `models` with ordered `source_group_ids`. Responses return selected source summaries and a derived model count. Keep the legacy `models` response during the compatibility window only where older clients need it.

Remove the manual sync-preview operation from the new interface. The endpoint may remain temporarily for old clients but must not be required by the dynamic path.

Add a pricing-coverage preview for source-group forms. It accepts the prospective platform, model list, and unsaved `model_pricing` payload and returns per-model status:

- `priced` with the effective source;
- `missing` with the required billing fields;
- `invalid` with a stable reason.

Group create/update performs the same validation in the write transaction or immediately before the atomic group update.

## Cache And Consistency

System custom model catalogs may be cached by custom-group ID, but cache keys must include or be invalidated by source-group changes. Creating, updating, deleting, enabling, or disabling a source group; changing its model list, mappings, accounts, or pricing; and updating custom-group source references invalidates affected custom-group catalogs and API-key authentication caches.

Catalog reads use one batched account/source snapshot rather than one query per model. A short cache is acceptable for load reduction, but invalidation is the correctness mechanism.

Source references and custom-group fields are written atomically. Source-group model-list and pricing changes are already submitted in one group update and remain atomic. A failed pricing validation writes neither part.

## Administrative UI

The system custom group dialog contains:

- basic subscription settings;
- an ordered source-group selector;
- a read-only derived summary showing unique model count, duplicate/fallback count, unpriced routes, and unavailable sources.

It removes individual model checkboxes, aliases, and manual synchronization controls. Reordering selected source groups changes fallback priority.

The source-group dialog links selected models with pricing status. Missing-price rows open inline pricing controls and support applying one reviewed price configuration to several selected models.

Desktop uses a compact two-column summary where space permits. Mobile uses a single column in the order `Models`, `Pricing required`, `Review`, with full-width controls, stable button heights, no horizontal scrolling, and no page reload after save.

## Compatibility

- Existing system custom group IDs and subscription relations remain unchanged.
- Existing API keys continue to bind the same custom group.
- Existing model names remain unchanged because current production routes do not use aliases.
- Added source models become visible automatically only after they are schedulable and priced.
- Removed source models disappear automatically.
- Renamed models appear under the new name and the old name stops resolving. This is an intentional consequence of removing aliases.
- Historical usage and billing records remain unchanged.
- Old route rows are retained for one release as rollback data.

## Error Handling

- Reject empty source selections and invalid/self/nested source references.
- Return stable metadata for an unavailable source and missing pricing.
- Omit one invalid fallback candidate while another valid candidate exists.
- Reject the public model only when no valid candidate remains.
- Fail source-group save when a newly published model lacks effective pricing.
- Preserve administrator-entered form state when preview or save fails.

## Testing

Backend tests cover migration, source validation, deterministic priority, case-insensitive union, duplicate-model fallback, model add/remove/rename propagation, unavailable source handling, pricing filtering, cache invalidation, and unchanged subscription billing identity.

Pricing tests cover existing explicit prices, fallback catalog prices, missing token/request/image prices, normalized model names, template-prefilled but incomplete entries, and atomic rejection of model-list plus pricing changes.

Frontend tests cover source-only create/edit requests, source ordering, derived summaries, removal of alias/sync controls, automatic missing-price entries, one-save behavior, preserved state on failure, and responsive layout.

End-to-end verification creates a source group and custom subscription group, adds a uniquely named priced model, confirms it appears without editing the custom group, removes it, confirms it disappears, and verifies an unpriced model is blocked before exposure.

## Rollout And Rollback

Deploy the additive schema and migration first, then the compatible backend, then the simplified frontend. After deployment, compare dynamic catalogs with retained route snapshots for the five current production custom groups before enabling dynamic resolution globally.

Rollback switches resolution back to retained `system_custom_group_models` rows. No subscription or API-key data migration is required.
