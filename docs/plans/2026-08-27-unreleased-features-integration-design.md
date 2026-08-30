# Unreleased Features Integration Design

## Scope

Port the completed but unreleased Sub2API behavior from the stale `codex/balance-transfer-redeem-codes` line into the current `dev` tree:

1. User self-service account deletion with password confirmation, API-key cleanup, auth cache invalidation, and session revocation.
2. Registration tombstones so deleted email identities and normalized aliases cannot register again, including verified-email OAuth flows.
3. Read-only model-list access when the account balance or subscription allowance is exhausted.
4. Native Gemini image generation in Image Studio, including prompt/reference-image conversion, upstream dispatch, response extraction, and usage recording.

Card storefront branding cleanup is explicitly outside this integration because it belongs to separate repositories whose local trees do not match production.

## Integration Strategy

Do not merge the stale feature branch wholesale. Port each behavior onto current interfaces and generated wiring so later production fixes remain intact. Existing equivalent features such as balance-transfer redeem codes, custom-group bulk selection, compensation flows, and model-plaza mobile work are left untouched.

## Behavior

### Account deletion

Expose `DELETE /api/v1/user/account`. The authenticated user must provide the current password. Admin accounts are rejected. The service deletes the user's API keys and soft-deletes the user in one repository transaction when available, invalidates key/user auth caches, and revokes all sessions after success. The profile page presents this action in a danger-zone card with an explicit confirmation.

Registration checks use a repository capability that includes soft-deleted rows. Exact emails and normalized aliases remain reserved after deletion. Normal administrative user creation keeps its existing semantics; only guarded registration and verified-email OAuth creation use the tombstone check.

### Model lists

GET model-list endpoints remain authenticated and still enforce active user/key status. They bypass balance and subscription-limit enforcement because listing models has no billable upstream work. Other endpoints retain current billing gates.

### Gemini Image Studio

When the selected Image Studio group is Gemini and the requested model is an image-generation model, convert the request to Gemini `generateContent`, route through the existing Gemini gateway, extract inline image data, and record usage through the existing OpenAI-compatible usage pipeline. Non-Gemini Image Studio requests keep their current path unchanged.

## Error Handling

Account deletion returns existing typed password, authorization, and persistence errors. Gemini returns typed bad-request errors for missing prompts and service-unavailable errors for missing gateways, unsupported accounts, or empty image output. No new silent fallback is introduced.

## Verification

Each slice starts by porting its tests and observing the expected failure on `dev`. After implementation, run focused Go/Vitest tests, backend package tests for touched packages, frontend type checks, and the existing critical test suite.
