# User Custom Model Groups Design

## Goal

Allow a user to combine models from multiple groups they can already access
into one personal virtual group, bind one API key to it, and use that key from
clients such as SillyTavern without repeatedly switching groups or keys.

## Phase 1 Scope

- User-owned custom groups with create, rename, enable/disable, edit, and delete
  workflows.
- A unique mapping from each exposed model name to one concrete source group.
- API keys bound either to one concrete group or one user custom group.
- Merged model listing for a custom-group key.
- Text request routing for OpenAI Responses, Chat Completions, and Anthropic
  Messages compatibility paths.
- Existing source-group authorization, scheduling, quotas, pricing, billing,
  and usage accounting remain authoritative.
- User-facing management UI and minimal administrator visibility/control.

Image, video, and Gemini-native request paths are deferred to a later phase.
Model aliases and automatic fallback between source groups are also out of
scope.

## Domain Model

`user_custom_groups` stores the owner, display name, enabled state, timestamps,
and soft lifecycle metadata.

`user_custom_group_models` stores one row per exposed model:

- custom group id;
- public model name (the unchanged source model name);
- source concrete group id;
- source model name.

The pair `(custom_group_id, public_model)` is unique. Source groups remain the
owners of accounts, channels, pricing, multipliers, subscriptions, limits, and
provider behavior. A custom group owns none of those resources.

An API key references either `group_id` or `custom_group_id`, never both. The
database and service validation enforce this invariant.

## Request Resolution

Authentication identifies the API key and its binding type. For a custom-group
key, the gateway reads the requested model, loads the matching custom model
route, and validates on every request that:

1. the custom group is enabled and owned by the key's user;
2. the source group is enabled and concrete;
3. the user currently has access to the source group;
4. the source group still exposes/allows the requested model;
5. the current endpoint is supported in phase 1.

After validation, the resolved source group becomes the effective group for the
rest of the request. Existing channel mapping, account scheduling, stickiness,
breakers, quota checks, and upstream forwarding operate only within that source
group. Multiple accounts inside the group remain subject to the existing
scheduler; users do not pin individual accounts.

Resolution fails closed. It never deletes mappings, selects another group, or
falls back to a same-named model elsewhere.

## Billing and Usage

The source group is authoritative for all user billing:

`source-group model/channel price × source-group effective user multiplier × applicable peak multiplier`

The selected upstream account does not change the user's price. Account
multipliers continue to affect administrator cost/profit accounting and
scheduling only.

Usage records retain the effective source group and additionally record the
custom group id so administrators and users can trace both the personal entry
point and the actual billing group. Subscription eligibility, balance charging,
RPM, concurrency, daily/weekly/monthly limits, and platform quota checks all use
the resolved source group.

## API and UI

User APIs provide:

- CRUD for owned custom groups;
- atomic replacement of model mappings;
- candidate source groups and models filtered to current user access;
- validation summaries for mappings that became unavailable.

API key create/update APIs accept a concrete group id or custom group id. The
models endpoint returns the custom group's valid merged model set for a bound
key.

The user UI adds a Custom Groups page. It shows source-group provenance for
every model and marks stale mappings unavailable without silently changing
them. The API Key form presents concrete groups and personal custom groups as
separate choices. A custom group referenced by an API key cannot be deleted
until it is unbound; disabling it immediately blocks its keys.

Administrators can list and disable custom groups for support and abuse
response, but phase 1 does not let administrators silently rewrite mappings.

## Errors

Stable errors distinguish:

- model not configured in the custom group;
- custom group disabled;
- source group disabled;
- source-group access lost or subscription expired;
- source group no longer supports the model;
- no eligible upstream account;
- endpoint unsupported for custom groups in phase 1.

No error response exposes internal account credentials or private routing
details beyond the source group name already visible to the user.

## Testing

- Schema constraints and repository ownership isolation.
- Candidate groups/models respect current access.
- Duplicate public model mappings are rejected.
- API keys enforce exactly one binding type.
- Requests resolve only the configured source group and reuse its scheduler.
- Billing uses source-group pricing, user override, and peak multiplier.
- Access expiry and configuration drift fail closed.
- Models responses merge only currently valid mappings.
- Existing concrete-group API keys retain identical behavior.
- Vue tests cover CRUD, mapping selection, provenance, stale states, and API key
  binding.
