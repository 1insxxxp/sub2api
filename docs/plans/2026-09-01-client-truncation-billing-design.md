# Client Truncation Billing Design

## Goal

When a downstream streaming client disconnects, stop the matching upstream generation immediately and bill the user only for output tokens that were successfully delivered downstream.

## Behavior

- A canceled downstream request or downstream write failure freezes the delivered-output collector and cancels the upstream request context.
- The usage record stores delivered output tokens in `output_tokens` and provider-reported output tokens in `upstream_output_tokens` when available.
- For a disconnected stream, customer-facing output cost, total cost, and actual charge are calculated from delivered output tokens.
- For a normally completed stream, provider usage remains authoritative for billing.
- Account-side cost diagnostics continue to use provider usage when a terminal usage event was received. If cancellation prevents terminal usage, the best available partial usage is retained without inventing provider tokens.
- HTTP/SSE and OpenAI-compatible streaming paths share the cancellation mechanism. WebSocket relays close or cancel their upstream turn when the downstream session disappears.

## Constraints

The server cannot reliably distinguish a user clicking Stop from a browser close or network loss. All downstream cancellations therefore use the same immediate-cancel and delivered-token billing behavior.

## Verification

- A regression test proves downstream cancellation reaches the upstream request promptly.
- Billing tests prove a disconnected request with provider output larger than delivered output is charged using delivered output.
- Normal completion tests prove unchanged provider-based billing.
- Usage-log tests prove displayed output tokens and output cost use the same token basis.

