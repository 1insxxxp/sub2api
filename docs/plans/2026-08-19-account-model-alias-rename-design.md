# Account Model Alias Rename Cascade Design

## Context

Admins can edit an account's model mapping in the account edit dialog. For
Antigravity accounts, the left side of each mapping is the public model name
that users request, and the right side is the upstream model sent to the
provider.

Today, changing the left-side model name only updates the account credentials.
Other configuration still points at the old public name, so admins must
manually sync channel pricing and fix user/system custom group routes.

## Goal

When an admin renames a left-side account model mapping, keep downstream
configuration usable without manual cleanup.

## Rename Semantics

The frontend should compare the mapping snapshot loaded when the dialog opens
with the mapping submitted on save. A rename is detected when a row keeps the
same upstream target but changes its left-side public model name.

For each detected rename from `old_model` to `new_model`, scoped to the
account's currently bound source groups:

- Update user custom group routes where `source_group_id` is one of the
  account's groups and `source_model` equals `old_model`.
- If a user custom route's `public_model` also equals `old_model`, update it to
  `new_model`; if the user chose a custom alias, leave `public_model` unchanged.
- Update system custom group routes with the same `source_group_id` and
  `source_model` match, preserving enabled state and public aliases with the
  same rule.
- Copy or extend channel pricing so every channel attached to those groups also
  prices `new_model` using the old model's pricing when present. Keep old model
  pricing in place to avoid breaking other accounts or historical clients.
- Copy channel model mapping entries from `old_model` to `new_model` when the
  old key exists and the new key does not.

Conflicts should be conservative. If changing a custom group's `public_model`
would collide with another public model in that same custom group, keep the
existing public alias and only migrate `source_model`. If changing
`source_model` would violate a unique route constraint, skip that route and
return it in the result as a skipped item.

## API Shape

Add an admin endpoint that can run after account update:

`POST /api/v1/admin/accounts/:id/model-alias-renames`

Request:

```json
{
  "renames": [
    { "old_model": "old-public", "new_model": "new-public" }
  ]
}
```

Response:

```json
{
  "channel_pricing_updated": 1,
  "channel_mappings_updated": 1,
  "user_custom_routes_updated": 2,
  "system_custom_routes_updated": 1,
  "skipped": []
}
```

The endpoint should load the account, derive the affected source groups from
its current bindings, and apply the cascade transactionally where practical.
It should invalidate channel/auth caches for affected groups after successful
changes.

## Frontend Behavior

In `EditAccountModal.vue`, keep the original Antigravity mapping rows when the
dialog opens. On save success, detect left-side renames and call the cascade
endpoint. Show a success message that includes the number of downstream updates,
or a warning if some rows were skipped.

The first implementation should keep cascade enabled by default. A separate
confirmation dialog is not required.

## Testing

Backend tests should cover:

- User custom route source model migration.
- User public model migration only when it equals the old source model.
- System custom route migration.
- Channel pricing copy/extension while preserving old pricing.
- Conflict handling and skipped results.

Frontend tests should cover:

- Rename detection from original to submitted Antigravity mapping.
- No cascade call when only the right-side upstream target changes.
- Cascade call after account save succeeds.
