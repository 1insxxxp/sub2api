# Gemini final estimation fallback implementation plan

**Goal:** Apply the user's updated policy: response usage first, exact ledger second, same-account input token counting third, and semantic estimation last; missing usage no longer automatically creates pending review.

**Architecture:** Keep real measurements field by field and perform fallback only in the existing asynchronous usage-record task. Capture the final actual native request and received native response semantics, never the serialized transport envelope as token text. Source metadata remains internal; estimates must never be labeled as provider usage. Reuse normal request identity, billing, quota and dedup.

**Tech stack:** Go services/handlers, existing offline O200K tokenizer, native Gemini countTokens, existing provenance/outbox fields.

The user explicitly replaced the prior no-estimate/pending policy. This is authorization to implement this local behavior change; deployment and historical monetary edits are outside this turn.

## Input evidence

- Capture each final request via GetBody, bounded to 4MiB; unwrap Code Assist request envelopes. Read semantic text, system instructions, tool schemas, calls/results and response schemas. Exclude JSON transport keys, thought signatures, URLs and base64.
- For supported static API-key origins, build a same-origin/same-model countTokens request only from the captured inference endpoint; no scheduler/account changes, generation call, redirect or disguised local-success response. Bound lookup to 2 seconds; clear request body immediately afterward.
- countTokens is a more useful estimate, not this inference's final usage. Explicit measured zero remains authoritative. Preserve reported cache tokens and subtract them from estimated gross input once.
- Optional local calibration is restricted to text-only/no-external-cache request shapes and exact account/origin/key/model. Use at most 32 recent pairs per 512 bounded keys. Enable a median correction only after at least 16 measured samples, stable training ratios and held-out evidence of improvement; otherwise retain the raw semantic estimate. No fixed future error guarantee.

## Output evidence

- Count received candidate text/tool arguments separately from received thought text, joining streamed segments before tokenizing. Do not use only successfully delivered output, because cancellation can occur during a downstream write.
- Preserve reported output totals (candidates plus reported thoughts), including explicit zero; do not add local counts to an already reported output total. When candidates are missing, estimate received candidate text and add either reported thoughts or received thought text, never both. Do not infer hidden thought tokens from elapsed time or max_tokens.
- A separate finalized flag permits estimated settlement without claiming MeasuredUsageComplete. Requests with no upstream acceptance or generation evidence do not acquire speculative input charges.

## Tasks and tests

1. Add input semantic/count helper with unit and local HTTP tests: mixed-language discontinuity, tools, opaque data exclusion, exact final URL/body/auth, redirect/timeout/invalid JSON/explicit zero, fallback source and body cleanup.
2. Add received output snapshot tests for all protocols and cancellation, function calls, split chunks, thinking separation, retry isolation and scalar-only result.
3. Add bounded calibration tests for insufficient history, held-out improvement/rejection, outliers, TTL, eviction, isolation and concurrency.
4. Test settlement precedence and source selection before changing the handler; update the existing three-protocol cancellation matrix to require normal usage records and zero pending rows. Keep ledger and measured usage authoritative.
5. Run affected service/handler regressions, normal force-cache protections, privacy/provenance checks and diff verification. Do not change production or historical balances.

## Accuracy limits

The deployed provider may not support countTokens for its private model aliases. The repository's local tokenizer is not Gemini's vocabulary; hidden upstream prompt transformations, unavailable cached content and unreported hidden reasoning cannot be reconstructed exactly. These cases must remain explicitly estimated internally. The implementation reduces known large-error causes but must not claim a guaranteed percentage without production comparisons.

Reference: https://ai.google.dev/api/tokens (full generateContentRequest support; token counting is distinct from response usage).

## Completed validation

- Input, output-evidence, and finalization tests first failed against their stubs; all targeted cases now pass. Additional regressions caught and fixed explicit-cache double subtraction, structured/cumulative/parallel tool duplication, misleading retry attribution and transient request-body serialization.
- The remote count tests use local HTTP servers and verify the count endpoint only, original authorization scheme, strict redirect refusal, explicit zero and the two-second deadline. No production inference was issued.
- The three-protocol handler matrix verifies ledger priority, count fallback, local estimate settlement, zero new pending rows, one billing command, actual-field precedence, and zero speculative charge without upstream acceptance.
- Calibration isolated tests and race tests pass. Ordinary-user/admin DTO tests first reproduced source exposure, then passed after the user-only projection change.
- Final command passed: `go test -p=1 -tags=unit ./internal/service ./internal/handler ./internal/handler/dto -run 'Gemini|UpstreamRequestID|ForceCacheBilling|RecordUsage_MissingInput|UsageLogFromService' -count=1 -timeout=240s`. Service 43.840s, handler 4.453s, DTO 0.551s.
- `git diff --check` is clean. No commit, push, deployment, production configuration change or historical balance edit occurred.

Resource limits: input count snapshot at most 4MiB, count response at most 64KiB; output fallback buffers at most 4MiB/4096 semantic streams, with truncation recorded internally. Real output measurements immediately discard unused fallback buffers. Local estimates omit unavailable media/external-cache contents and do not invent hidden reasoning. These bounds and unavailable provider-side data remain accuracy limitations, not proof of a fixed error percentage.
