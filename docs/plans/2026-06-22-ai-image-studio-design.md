# AI Image Studio Design

## Goal

Add a first-party AI image generation workspace to Passion API. Users can generate images from prompts, edit from reference images, choose common aspect ratios, pay from their existing USD wallet balance, and keep a lightweight image history stored outside the application server.

## Context

The backend already has an OpenAI-compatible image gateway for:

- `/v1/images/generations`
- `/v1/images/edits`
- `/images/generations`
- `/images/edits`

The current codebase also already has group-level image controls and image pricing fields:

- `allow_image_generation`
- `image_price_1k`
- `image_price_2k`
- `image_price_4k`

This means the feature should not create a separate image gateway or a separate billing system. The first version should wrap the existing gateway with user-facing APIs, storage, UI, and admin settings.

## Reference Products

Use these projects as product references, not as code dependencies:

- LibreChat: prompt-first image generation inside an AI workspace.
- Open WebUI: admin-configured image generation providers and image settings.
- LobeChat: image model abstraction and model capability registration.

The local product direction should stay closer to a compact SaaS tool than a full creative suite. The first version should be useful, polished, and controlled, without adding folders, community sharing, advanced workflow graphs, or permanent asset management.

## Confirmed Direction

Build an integrated "AI Image Studio" in the existing authenticated user app.

Confirmed choices:

- Use the user's existing site balance for billing.
- Reuse current OpenAI-compatible image routes and image billing.
- Start with OpenAI-compatible image models only.
- Store generated image assets in Cloudflare R2 in production.
- Use public R2 URLs with random, non-guessable object keys for the first version.
- Keep database records for metadata and ownership, not image binary data.
- Let users view, download, copy, continue editing from, and delete their generated images.

## User Experience

Add a user-side image workspace page with:

- Text-to-image and image-edit modes.
- Prompt input.
- Model selector filtered to admin-allowed image models.
- Aspect ratio selector: `1:1`, `16:9`, `9:16`, `4:3`, `3:4`.
- Reference image upload, drag-and-drop, and paste support for edit mode.
- Estimated cost before submission.
- Generation loading state with clear progress messaging.
- Result gallery with preview, download, copy link, delete, and edit-again actions.
- Recent image history with model, ratio, cost, and created time.

The UI must follow the current Passion API theme, support Chinese and English, preserve light and dark modes, and work on mobile.

## Billing

Billing should use the existing USD wallet balance and current image usage accounting.

Rules:

- Fail fast when the user balance is insufficient.
- Charge only after upstream generation succeeds.
- Do not charge failed generations or failed edits.
- Record image usage in the existing usage history.
- Use group pricing from the user's current group.

Initial aspect-ratio price mapping:

- `1:1`, `4:3`, and `3:4` use the `1K` image price.
- `16:9` and `9:16` use the `2K` image price.
- Future high-resolution modes can use the `4K` image price.

The backend remains the source of truth for final cost. The frontend estimate is only a preview.

## Storage

Production storage should use Cloudflare R2. Development can use a local filesystem storage adapter.

Add a small storage abstraction so the feature does not depend directly on R2 everywhere:

- `ImageStorage.Put(ctx, object, contentType, bytes) -> publicURL`
- `ImageStorage.Delete(ctx, object) -> error`

Recommended production settings:

- `IMAGE_STORAGE_DRIVER=r2`
- `R2_ACCOUNT_ID`
- `R2_ACCESS_KEY_ID`
- `R2_SECRET_ACCESS_KEY`
- `R2_BUCKET`
- `R2_PUBLIC_BASE_URL`

Recommended development settings:

- `IMAGE_STORAGE_DRIVER=local`
- `IMAGE_STORAGE_LOCAL_DIR`
- `IMAGE_STORAGE_PUBLIC_BASE_URL`

Object keys should be random and non-guessable, for example:

```text
images/user-123/2026/06/<uuid>.png
```

R2 objects should be publicly readable through a custom domain such as `assets.passionapi.com`. This keeps image delivery off the application server and gives users direct image previews and downloads.

## Data Model

Add an image records table for generated assets.

Suggested fields:

- `id`
- `user_id`
- `mode`: `generation` or `edit`
- `model`
- `prompt`
- `aspect_ratio`
- `size`
- `image_url`
- `storage_driver`
- `storage_object_key`
- `mime_type`
- `bytes`
- `cost`
- `usage_log_id`, if available
- `source_image_count`
- `expires_at`
- `deleted_at`
- `created_at`
- `updated_at`

The first version does not need to store source reference images permanently unless the upstream edit response requires it for audit or replay. If stored later, source images should use the same R2 lifecycle and ownership model.

## Backend APIs

Add authenticated user APIs under `/api/v1/user/images`.

Suggested endpoints:

- `GET /api/v1/user/images/config`
  - returns enabled state, allowed models, default model, aspect ratios, estimated price mapping, upload limits, and storage policy.
- `POST /api/v1/user/images/generate`
  - JSON prompt generation.
- `POST /api/v1/user/images/edit`
  - multipart form request with prompt and reference images.
- `GET /api/v1/user/images`
  - paginated user image history.
- `DELETE /api/v1/user/images/:id`
  - soft delete DB record and delete R2 object when possible.

The user APIs should call existing image gateway/service logic instead of duplicating upstream routing, failover, moderation, and billing behavior.

## Admin Settings

The first version should expose or reuse these controls:

- Global image studio enabled/disabled.
- Allowed image models.
- Default image model.
- R2 configuration status.
- Storage retention days.
- Max saved images per user.
- Max reference image upload size.

Group-level settings should continue to control:

- Whether a group can use image generation.
- `1K`, `2K`, and `4K` image prices.

## Error Handling

User-visible errors should distinguish:

- Image generation disabled.
- Group not allowed to use image generation.
- No available image model.
- Insufficient balance.
- Unsupported aspect ratio or model.
- Reference image too large or invalid.
- Upstream provider failure.
- Storage upload failure.

If upstream generation succeeds but storage upload fails, the request should not leave the user with a charged-but-invisible image. Prefer either not charging until storage succeeds, or refunding/rolling back the charge in the same request flow if storage cannot be completed.

## Cleanup And Limits

The design should prevent unlimited storage growth.

First-version controls:

- Retention days through `expires_at`.
- Per-user saved image limit.
- Delete oldest expired or over-limit records through a scheduled cleanup command/job.
- User deletion removes visible history and attempts R2 object deletion.

R2 lifecycle rules can also be configured as a second layer, but the application should keep its own `expires_at` and cleanup logic so the UI remains consistent.

## Non-Goals

Do not include these in the first version:

- Public community gallery.
- Permanent user albums or folders.
- Advanced canvas editor.
- ComfyUI workflow builder.
- Multi-provider non-OpenAI adapters such as Gemini, Flux, or Stability.
- Private signed URL access.
- NSFW or compliance review workflows beyond existing moderation hooks.

## Testing

Backend tests should cover:

- Aspect ratio to size and price-tier mapping.
- Balance insufficient path.
- Group disallowed path.
- Successful generation stores image metadata and charges once.
- Storage failure does not silently consume user balance.
- User can only list and delete their own images.
- R2/local storage adapter object key generation and delete behavior.

Frontend tests should cover:

- Config loading and disabled states.
- Ratio/model selection.
- Cost estimate rendering.
- Upload, paste, and drag reference images.
- Successful generation gallery update.
- Delete action removes an image from history.
- Chinese and English copy for main states.

Manual verification should include:

- Light and dark mode.
- Desktop and mobile layouts.
- Text-to-image and edit mode.
- R2 public URL preview.
- Usage history and balance changes after success.
