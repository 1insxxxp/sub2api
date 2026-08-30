# Tavern Stream Reliability Design

## Context

SillyTavern traffic currently follows `client -> Sub2 -> CPA -> upstream`. Recent
production outcomes show three user-visible failure modes: incomplete streams
reported as successful, terminal responses with no visible content, and Gemini
requests rejected because the converted conversation ends with a `model` turn.
Client cancellations are also retried by the Gemini compatibility layer, which
adds load after the downstream connection is already gone.

## Goals

- Never convert a premature upstream EOF into a normal `[DONE]` response.
- Fail over before committing downstream headers when the upstream terminates
  without visible text, media, or a tool call.
- Stop same-account retries immediately when the request context is canceled.
- Accept assistant-prefill conversations commonly sent by SillyTavern.
- Preserve all current account routing, billing, model mapping, and reasoning
  behavior.
- Allow long-lived streams to remain open for up to 900 seconds at Nginx.

## Design

### Context-aware retries

After `httpUpstream.Do` fails, inspect both the returned error and `ctx.Err()`.
`context.Canceled` and `context.DeadlineExceeded` return immediately and never
enter Gemini backoff. Other transport failures retain the existing retry policy.

### Terminal-aware streaming

Delay the downstream HTTP 200 and initial stream event until the first visible
text or tool call is available. Track whether Gemini supplied a real
`finishReason`.

- EOF before a terminal reason and before visible output returns an
  `UpstreamFailoverError`, allowing the gateway to select another credential.
- EOF after partial output emits a structured SSE `incomplete_stream` error and
  does not emit a normal stop event or `[DONE]` marker.
- A terminal response with no visible output also returns an
  `UpstreamFailoverError` before headers are committed.
- A complete response preserves the existing OpenAI-compatible stream shape.

This keeps first-token buffering bounded to the first useful content event and
avoids replaying a request after the client has already received generated text.

### Assistant-prefill compatibility

When converted Gemini contents end in a `model` role, append a neutral user turn
asking the model to continue without repeating existing text. This preserves the
assistant prefill while satisfying Gemini's requirement that a request end in a
user turn.

### Empty non-streaming responses

Before writing a successful JSON response, require at least one visible text,
media, or tool-call output. Empty terminal responses return an
`UpstreamFailoverError` so the existing group-level failover and empty-response
compensation can select another credential.

### Proxy timeout

Raise the Sub2 Nginx read, send, and client send timeouts from 300 to 900 seconds.
Keep buffering disabled. Application-level stream inactivity detection remains
the primary guard against abandoned connections.

## Verification

- Unit tests cover canceled requests, assistant-prefill conversion, premature
  EOF before output, premature EOF after partial output, empty terminal output,
  and a complete stream.
- Run the focused Gemini compatibility suite and the broader service tests.
- Deploy Sub2 with the existing production reasoning customization preserved.
- Exercise direct and Cloudflare endpoints with streaming and non-streaming
  requests, then inspect response outcomes and container health.

## Rollback

Retain the previous Sub2 image tag and Nginx configuration backup. Rollback is a
container image switch plus Nginx config restore; no database migration is
required.
