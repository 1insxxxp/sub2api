# Custom Group Model Call Aliases Design

## Goal

Allow a user custom group to include the same real model from multiple source
groups while keeping routing and billing explicit and predictable.

## Decision

Each custom-group model mapping has three distinct concepts:

- **Call name**: the model identifier exposed to and submitted by the user;
- **Real model**: the concrete model sent to the selected source group;
- **Source group**: the concrete group that controls authorization, channel
  scheduling, pricing, multipliers, quotas, and provider behavior.

The existing `public_model` field becomes the editable call name. The existing
`source_model` and `source_group_id` fields remain the real model and source
group. The unique `(custom_group_id, public_model)` constraint already enforces
the required rule: call names must be unique inside one custom group, while the
same `source_model` may appear more than once when the source group or call name
differs.

Example:

| Call name | Real model | Source group |
| --- | --- | --- |
| `claude-opus-4-6-balance` | `claude-opus-4-6` | Balance group |
| `claude-opus-4-6-discount` | `claude-opus-4-6` | Discount group |

## Alternatives Considered

### Automatic weighted or random selection

One public model could hold several source groups and select one at request
time. This was rejected because different source groups may have different
prices, multipliers, quotas, and quality. A user could not predict the route or
cost of a request.

### Priority with automatic fallback

One public model could use a primary source and fall back to another source.
This may be useful as a separate future feature, but it changes billing during
failures and needs an explicit fallback policy. It is outside this change.

### Explicit call aliases

Each mapping exposes a unique user-controlled call name. This is selected
because routing and billing remain deterministic and the current schema already
models nearly all required data.

## User Experience

The custom-group editor shows these fields for every selected mapping:

1. **Call name**: editable and used in API requests;
2. **Real model**: read-only model selected from the source group;
3. **Source group**: read-only provenance for that mapping.

When a model is first added, its call name defaults to the real model name. If
that name is already present, the client suggests a stable suffix derived from
the source-group name. The user may replace the suggestion before saving.

The editor may contain multiple rows for the same real model. Selection state
must therefore be keyed by the source mapping identity, at minimum
`source_group_id + source_model`, rather than by model name alone.

Validation errors are displayed next to the affected call name. On mobile, the
call-name input and source metadata stack vertically without requiring a wide
table.

## API And Validation

Create and update requests continue to submit:

```json
{
  "public_model": "claude-opus-4-6-discount",
  "source_group_id": 42,
  "source_model": "claude-opus-4-6"
}
```

The backend treats `public_model` as the call name and validates:

- it is non-empty after trimming;
- it satisfies the existing model-name length and character rules;
- it is unique case-insensitively within the custom group;
- the source group is concrete, enabled, and accessible to the owner;
- the source group currently exposes the exact real model;
- duplicate source mappings are rejected unless they have a distinct purpose
  supported by a later design.

The service normalizes surrounding whitespace but does not silently rename a
call name. Conflicts return a field-specific validation error.

## Request Resolution

For an API key bound to a custom group:

1. Read the request's `model` as the call name.
2. Resolve the unique mapping by custom group and call name.
3. Revalidate custom-group ownership/status and source-group access/status.
4. Set the concrete source group as the effective group.
5. Rewrite the dispatched model to the mapping's real model.
6. Reuse the existing concrete-group scheduler and provider path.

Resolution never randomly selects another mapping with the same real model. If
the configured source becomes unavailable, the request returns the existing
explicit custom-group/source error.

## Models Endpoint And Privacy

The models endpoint for a custom-group API key exposes only valid call names.
It does not expose the real model, source-group identifier, source-group name,
or multiplier to downstream clients. The custom-group management UI may show
source provenance because the owning user already has access to that group.

## Billing And Audit

Billing remains authoritative to the resolved source group:

`source-group price × effective source-group user multiplier × peak multiplier`

No alias-level multiplier is introduced. Usage records continue to retain the
effective source `group_id` and the entry `custom_group_id`. Existing request
model fields should preserve the submitted call name where currently expected,
while provider/model audit fields preserve the resolved real model. Tests must
verify both identities are traceable without leaking source details through
downstream model discovery.

## Compatibility And Migration

Existing records require no data rewrite: their `public_model` already equals
the currently exposed model and remains a valid call name. The current unique
constraint is retained.

The main compatibility change is application behavior and UI state: duplicate
real models from different source groups must no longer collapse into one
selection. If case-insensitive uniqueness is not currently enforced in the
database, service validation is mandatory and a small online-safe expression
index may be added after checking existing data for case-only conflicts.

## Error Handling

The API distinguishes:

- duplicate call name;
- invalid call-name format;
- duplicate source mapping;
- source group unavailable or no longer accessible;
- real model no longer exposed by the source group;
- call name not configured for the custom group.

Errors must not reveal channel credentials, upstream account data, or hidden
multiplier details.

## Testing

Coverage includes:

- service validation for unique call names and repeated real models from
  different source groups;
- repository persistence and case-insensitive conflict behavior;
- resolver rewriting from call name to real model and fixed source group;
- scheduler, subscription, quota, and billing inheritance from each selected
  source group;
- usage/audit traceability for call name, real model, source group, and custom
  group;
- models endpoint privacy and alias-only output;
- desktop and mobile editor behavior with two rows sharing one real model;
- regression coverage proving all existing custom groups continue working
  unchanged.
