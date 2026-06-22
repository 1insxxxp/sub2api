# AI Image Studio Selectable Groups Design

## Goal

Evolve AI Image Studio from a single implicit image route into a user-selectable image workspace. Users should choose an image-enabled group and an allowed image model before generating or editing images, while billing still uses the user's USD wallet balance and the selected group's image pricing.

## Context

The current local Image Studio implementation uses the first active API key owned by the user. That API key's `group_id` determines:

- Whether image generation is allowed.
- Which account pool can be scheduled.
- Which image prices apply.
- Which channel mapping may apply.

This is workable for API-key-native users, but it is weak for a first-party UI feature. A normal user may not have an API key, may not understand group binding, or may have a key bound to a text-only group. The image page should make the route explicit.

The codebase already has the core primitives needed for this:

- `groups.allow_image_generation`
- `groups.image_price_1k`
- `groups.image_price_2k`
- `groups.image_price_4k`
- `groups.image_rate_multiplier`
- `account_groups`
- `channel_groups`
- channel model mapping and restriction
- user allowed-group logic
- existing image gateway and image billing

## Product Direction

Add group and model selection to the user image page.

The user flow:

1. User opens AI Image Studio.
2. Backend returns the groups this user can use for image generation.
3. User selects a group.
4. Model selector updates to that group's allowed image models.
5. User selects a model and aspect ratio.
6. Page shows estimated cost from selected group pricing.
7. Backend validates the same group, model, balance, and schedulable account pool before forwarding.
8. Successful generations are stored and billed against the user's wallet.

This keeps the feature flexible while preserving admin control.

## Admin Experience

Keep the global Image Studio settings page, but change its responsibility:

- Global enable/disable.
- Storage settings.
- Retention and upload limits.
- Optional global fallback model list.
- Optional default group.

Do not make the global setting the only source of model truth. Group-level model availability should drive the user selector.

Group management remains the primary place to configure image commercial rules:

- Enable or disable image generation for that group.
- Configure 1K, 2K, and 4K image prices.
- Configure image rate multiplier.
- Bind image-capable accounts.
- Bind channel mapping/restriction when needed.

Later we can add a compact "Image Studio Groups" admin table inside the Image Studio settings panel, but V2 can reuse existing group/account/channel settings if the user-side options endpoint gives clear diagnostics.

## User Eligibility

A group is available to a user when all of these are true:

- The group is active and not deleted.
- `allow_image_generation` is true.
- The user is allowed to bind/use the group according to existing group permission rules.
- The group has at least one active, schedulable image-capable account.
- The group exposes at least one allowed image model.

For standard groups, reuse existing allowed-group and exclusive-group logic. For subscription groups, require an active subscription. Admin users can see groups that satisfy operational readiness, but generation should still follow normal billing and scheduling rules.

## Model Options

Models should be resolved per group.

Recommended resolution order:

1. If the group's channel has model restriction enabled, use the channel allowed models that are valid image models.
2. Else if group model routing or models list config contains image-capable models, use those.
3. Else use Image Studio global allowed models as fallback.
4. Remove duplicates and keep stable display order.

The backend should include both request model and mapped model hints when available, but the frontend should submit only the request model.

The first version can keep a conservative image-model allowlist helper so text-only models do not appear in the selector.

## Pricing Preview

The options endpoint should return estimated prices per group and aspect ratio.

Aspect-ratio tier mapping:

- `1:1`, `4:3`, `3:4` -> `1K`
- `16:9`, `9:16` -> `2K`

Price source:

1. Selected group image price for the tier.
2. Channel image pricing override, if available and already used by gateway billing.
3. Existing default image price fallback.

The frontend estimate is advisory. The backend remains the source of truth and records the actual cost after the upstream succeeds.

## Backend API Changes

Add:

- `GET /api/v1/user/images/options`

Response shape:

```json
{
  "enabled": true,
  "default_group_id": 2,
  "default_model": "gpt-image-1",
  "groups": [
    {
      "id": 2,
      "name": "Image Fast",
      "description": "Fast image route",
      "platform": "openai",
      "models": [
        {
          "model": "gpt-image-1",
          "label": "gpt-image-1",
          "mapped_model": "",
          "capabilities": ["generation", "edit"]
        }
      ],
      "prices": [
        { "ratio": "1:1", "size": "1024x1024", "billing_tier": "1K", "estimated_cost": 0.08 },
        { "ratio": "16:9", "size": "1536x864", "billing_tier": "2K", "estimated_cost": 0.12 }
      ]
    }
  ]
}
```

Modify:

- `POST /api/v1/user/images/generate`
- `POST /api/v1/user/images/edit`

Request additions:

```json
{
  "group_id": 2,
  "model": "gpt-image-1"
}
```

Validation rules:

- `group_id` is required when more than one image group is available.
- If `group_id` is omitted and only one image group is available, use that group.
- If no groups are available, return a user-friendly disabled/unconfigured error.
- The selected model must be valid for the selected group.
- The selected group must be available to the user at request time.

## Internal Execution Model

Move Image Studio gateway execution away from "first active user API key decides the group".

Recommended V2 execution context:

- User ID
- Selected group ID
- Selected model
- Optional API key ID only for attribution when needed
- User wallet billing identity

The gateway already expects an API-key-like object in many billing and usage paths. Build an internal Image Studio request context that contains:

- User
- Selected group
- Synthetic or selected APIKey metadata for compatibility
- Group ID for account selection
- User balance billing fields

If a compatible API key exists for the user and group, the service may reuse it for attribution. If not, it should create an in-memory synthetic context and avoid exposing any synthetic key to the user.

## Frontend Changes

User image page:

- Load `/user/images/options` with existing config/history.
- Add a group selector above or beside model selector.
- Disable model selector until a group is selected.
- Filter model choices by selected group.
- Show selected group's price preview for the selected aspect ratio.
- Show empty state when no groups are available.
- Keep text-to-image and reference edit modes.
- Keep upload, paste, drag-and-drop behavior.

Admin settings page:

- Keep existing panel.
- Add explanatory copy that image groups are configured in group/account/channel management.
- Optional: show a read-only readiness list of image-enabled groups.

## Error Handling

User-visible errors:

- Image Studio is disabled.
- No image-enabled group is available.
- Selected group is no longer available.
- Selected model is not available in this group.
- This group has no schedulable image account.
- Insufficient balance.
- Upstream image provider failed.
- Storage failed after generation.

The UI should keep errors short and actionable.

## Testing

Backend tests:

- Options endpoint returns only available image groups.
- Options endpoint excludes groups without image permission.
- Options endpoint excludes groups with no schedulable account.
- Options endpoint filters models per group.
- Generate requires or resolves `group_id`.
- Generate rejects unavailable groups.
- Generate rejects models outside selected group.
- Generate uses selected group for account scheduling.
- Usage logs store selected group ID and cost.
- Users without API keys can still generate when they are allowed to use an image group.

Frontend tests:

- Renders group selector.
- Model selector changes after group selection.
- Cost preview changes by group and ratio.
- No-group empty state.
- Generate payload includes `group_id` and `model`.
- Edit payload includes `group_id` and `model`.

Manual checks:

- One available group auto-selects.
- Multiple groups require clear selection.
- Chinese and English copy.
- Light and dark mode.
- Mobile layout.

## Non-Goals

Do not add these in V2:

- Public image marketplace.
- Per-user custom image model creation.
- Advanced provider-specific settings in the user page.
- Separate bonus wallet.
- Admin auto-provisioning of hidden API keys unless the compatibility layer cannot avoid it.
