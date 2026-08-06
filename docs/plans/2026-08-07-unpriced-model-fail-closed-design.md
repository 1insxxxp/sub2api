# Unpriced model fail-closed design

## Goal

Prevent successful requests from being recorded at zero cost when neither the
requested model nor its concrete mapped model has resolvable pricing.

## Production configuration

- Price `gemini-3-pro` at the same token rates as `gemini-3-pro-preview`:
  `$2 / MTok` input, `$12 / MTok` output, and `$0.2 / MTok` cache read.
- Remove `gemini-2.5-flash-image-preview` from account model mappings and remove
  its empty channel-pricing row so it is neither schedulable nor displayed.

## Request admission

After channel and account model mapping have selected the concrete billing
model, resolve pricing in this order: explicit channel pricing, requested model
pricing, concrete mapped/upstream model pricing, global catalog pricing, and
the existing built-in family fallback. If every candidate is unresolved, reject
the request before contacting the upstream provider.

Image and per-request channel pricing remain valid even when token prices are
absent. Known aliases continue to use their canonical base-model prices.

## Error handling

Return a clear non-retryable error stating that pricing is unavailable for the
model. Do not mark the selected account rate-limited or temporarily
unschedulable, because this is a local configuration problem rather than an
upstream failure.

## Verification

- Unknown token models fail before the upstream client is invoked.
- Known Gemini aliases resolve their canonical prices.
- Explicit image/per-request channel prices remain accepted.
- `gemini-3-pro` records non-zero cost with the group multiplier.
- The removed preview image model no longer appears in available channels.
- Production usage and error logs show no new zero-cost successful unknown-model
  requests after deployment.
