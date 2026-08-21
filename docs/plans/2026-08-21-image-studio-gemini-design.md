# Image Studio Gemini Models Design

## Goal

Allow AI Image Studio to show and run Gemini image models such as `gemini-3.1-flash-image-preview` when the selected user API key belongs to a Gemini image-enabled group.

## Current Behavior

The admin Image Studio setting can save `gemini-3.1-flash-image-preview` in `allowed_models`, but the user page still only shows `gpt-image-2`. The saved setting is not the problem. The options API filters models through the OpenAI images helper, which currently only accepts `gpt-image-*` and Grok image model IDs. The Image Studio executor also schedules images through the OpenAI images scheduler, which forces `PlatformOpenAI`.

## Approach

Add a provider-aware Image Studio model classifier and execution path.

For OpenAI and Grok image models, keep the existing `/v1/images/generations` gateway path unchanged.

For Gemini image models, add a Gemini execution branch inside `ImageStudioGatewayExecutor.Generate`. It will select the user-selected Gemini group API key, validate billing through the existing billing checker, build a Gemini-compatible `generateContent` request, forward it with the selected Gemini account, extract the returned inline image bytes, and defer existing usage recording until storage succeeds.

The user options API should expose Gemini image models only for Gemini groups and OpenAI/Grok image models only for their supported groups. This avoids showing a model that cannot actually run through the selected group.

## Supported Models

The first pass should recognize Gemini image model IDs already used elsewhere in the repo:

- `gemini-2.5-flash-image`
- `gemini-2.5-flash-image-preview`
- `gemini-3-pro-image`
- `gemini-3-pro-image-preview`
- `gemini-3.1-flash-image`
- `gemini-3.1-flash-image-preview`
- `gemini-3.1-flash-lite-image`
- `gemini-2.0-flash-exp-image-generation`
- `gemini-2.0-flash-preview-image-generation`

The classifier should also accept `models/<id>` by normalizing the prefix away.

## Data Flow

1. Admin config stores global `allowed_models`.
2. User opens AI Image Studio.
3. `/user/images/options` loads image-enabled groups available to the user.
4. For each group, the service derives image models for that group's platform and custom model list.
5. The frontend keeps its existing behavior: selected API key determines selected group, and model options come from that group.
6. Generation submits `api_key_id`, `group_id`, `model`, prompt, ratio, quality, output format, and background.
7. Backend validates the selected model for the selected group.
8. Gemini requests use the Gemini branch; OpenAI/Grok requests keep the existing branch.
9. Storage succeeds before usage is committed.

## Error Handling

If a Gemini model is configured on a non-Gemini group, it should be hidden from options and rejected at submit time. If no schedulable Gemini account exists, generation should return the same no-available-account style error used by the gateway. If Gemini returns no inline image bytes, return `IMAGE_GENERATION_EMPTY_RESULT`.

## Tests

Add tests for:

- Gemini image IDs are accepted by the Image Studio classifier.
- Gemini image IDs appear in options for Gemini groups.
- OpenAI groups still expose GPT image models.
- Gemini image generation builds a Gemini request and defers usage commit.
- Unsupported models remain hidden and rejected.
