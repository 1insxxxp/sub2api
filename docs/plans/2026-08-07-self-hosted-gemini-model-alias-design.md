# Self-hosted Gemini model alias design

## Goal

Allow the Gemini account named `自营号池` to accept the public model name
`gemini-3.5-flash` while forwarding it to the upstream-supported model
`gemini-3.5-flash-low`.

## Scope

- Add an account-level model mapping only to `自营号池`.
- Preserve every existing mapping and leave other Gemini accounts unchanged.
- Clear the stale rate-limit state created by the earlier 404-to-429 conversion.

## Data flow

Requests for `gemini-3.5-flash` routed to this account are rewritten to
`gemini-3.5-flash-low` before being sent to Antigravity-Manager. Direct requests
for `gemini-3.5-flash-low` remain unchanged.

## Verification

Confirm the persisted mapping, call the upstream target directly, call the
public alias through Sub2API, and verify that the account remains schedulable
without a new rate-limit timestamp.
