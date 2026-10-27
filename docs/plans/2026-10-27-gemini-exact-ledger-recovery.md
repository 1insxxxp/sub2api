# Recover missing Gemini input from the direct upstream ledger

Policy update: the user's later 2026-10-27 request replaces this plan's final pending/no-estimate branch with `2026-10-27-gemini-final-estimation-fallback.md`. Exact response usage and ledger recovery remain first priority; the completed new policy adds bounded counting and final semantic estimation when both are unavailable.

The user clarified that marking missing usage as pending is insufficient: correct input usage must be recovered where available, without differences of tens of thousands of tokens. The previous measured-only billing and pending-history work remains intact.

## Verified cause and upstream capability

Real HTTP tests reproduce the transport cancellation: all three Gemini adapters inherit the downstream cancellation context, so a cancellation/write failure closes the upstream before its delayed final usage. Buffered-tail tests alone did not exercise that failure.

A read-only production capability check of the affected account verified that its existing model API key can read `GET /api/log/token` with Bearer authentication. All five reported examples were found, including 21,916 input and 4,055 output. The provider returns `X-Oneapi-Request-Id`, but the account's optional custom request-ID header was unset. Historic affected records therefore have no trustworthy exact-ID join; timestamps alone will not authorize automated back-charges.

## Design

1. Capture the actual direct New API response ID on every real Gemini attempt. Independently hash the actual inference URL origin and credential, retaining only that hash for identity checking. Never substitute a next-hop ID, invented ID, prior attempt, or truncated ID.
2. During the existing asynchronous usage task, if complete response usage is absent, query the same account's direct origin `/api/log/token`. Keep immediate upstream cancellation; do not run generation longer or replay inference.
3. Use bounded detached lookup with strict redirect refusal, URL safety, existing proxy configuration, response-size limit, and bounded retries for not-yet-visible ledger rows. A lookup cannot mutate wallets by itself.
4. Select a unique consumption record with equal direct `request_id` and the exact model sent upstream. Require explicit valid input/output counters. Apply provider cache-inclusion metadata correctly. Do not use row numbers, approximate time, local tokenization, or `countTokens` as financial evidence.
5. Feed a recovered measurement into the original server-generated request identity and normal dedup settlement. Persist the direct upstream ID for audit. Complete response usage stays authoritative and does not trigger ledger lookup. Failed/unsupported/ambiguous lookup remains pending without a guessed charge.
6. Preserve measured Gemini input/cache categories even after a sticky-session account switch. The legacy `ForceCacheBilling` rewrite must not turn an exact measured input into zero input plus synthetic cached input.

Identity review also requires checking the final HTTP request after redirects, preferring the captured direct ledger ID over an optional next-hop header in persisted records, and refusing to reuse an ended HTTP retry attempt when cancellation happens before the next request is sent.

The endpoint exposes only the latest 1000 records and has no useful pagination/filter support in the verified provider contract. This initial repair makes bounded attempts immediately during usage recording; it does not introduce an unbounded background scanner or claim that arbitrary historic bills can always be recovered. Historical correction requires separately reviewed linkage and is outside this automatic future-request fix.

## Validation

- Pure parser tests cover exact ID/model matching, different next-hop IDs, duplicate rows, true zero, missing/null/fractional/negative counts, cached input and malformed metadata.
- Client tests cover same-origin/key identity, no redirects/credential leakage, bounded replies/retries, and no charge on unavailable evidence.
- Real HTTP tests cover Chat, Messages and native Gemini cancellation/write failure: upstream stops promptly, the final response usage is absent, the ledger supplies exact 21,916 / 4,055, and no later downstream output occurs.
- Handler tests exercise measured-ledger recovery and normal wallet commands once for all three protocols, including thought-only cancellation. Existing pending, ownership, concurrency and outbox tests remain gates.
- No production inference, configuration changes, historical balance edits, commits, pushes or deployments are included in this local repair.

### Completed local checks

- New real-HTTP recovery tests failed with the recovery stub, then passed with the implementation. All six actual HTTP abort cases recovered 21,916 input / 4,055 output while still canceling the inference connection and issuing exactly one inference request.
- All six new handler recovery cases first failed before wiring recovery into settlement. The complete interrupted-usage handler matrix now passes: `go test -p=1 -tags=unit ./internal/handler -run '^TestGeminiHandlers_InterruptedUsageBilling' -count=1 -timeout=120s` (4.366s).
- Seven new account-switch assertions first failed with input rewritten to zero, then passed after bypassing synthetic cache conversion for measured Gemini usage. Existing non-Gemini force-cache tests still pass.
- The final service regression selection passes: `go test -p=1 -tags=unit ./internal/service -run 'Gemini|UpstreamRequestID|ForceCacheBilling|RecordUsage_MissingInput' -count=1 -timeout=240s` (41.741s).
- Independent review fixes for final-response identity, direct-ID persistence and ended retry isolation were verified by dedicated regressions. `git diff --check` is clean.

This recovery runs only when input/output usage fields are incomplete. It does not reconcile every already-measured interrupted stream against a later final bill, and it does not schedule a persistent retry for ledger rows appearing after the bounded lookup window. Such unresolved requests retain the prior pending/no-estimate behavior.
