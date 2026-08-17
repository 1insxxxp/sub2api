# OpenAI-Compatible Stream Mode Relay Design

## Context

A mobile Tavern-like client called `1320094588@qq.com` through the per-request Gemini group and did not receive a response. Production evidence showed the upstream eventually succeeded, but the client disconnected first:

- Downstream endpoint: `/v1/chat/completions`
- Client mode: `stream=false`
- Actual group: `按次【gemini 反重力】`
- Actual account: `https://maxapi.hanyue.xyz`
- Upstream endpoint: `/v1/responses`
- Latest observed request: 132.9s server completion, while Nginx recorded client disconnect at about 125s.

The competing relay's proposed solution is a set of alternate base URLs that force or bridge stream behavior. This is useful when clients cannot configure `stream`, or when a model/upstream only behaves well in one transport mode.

## Goals

- Add OpenAI-compatible base URL paths that clients can choose without changing API keys or model names.
- Keep the default `/v1` behavior unchanged.
- Support Chat Completions and Responses text endpoints first, because the incident and competing guidance are both OpenAI-compatible conversation API scoped.
- Record usage, error, and request-type attribution correctly after mode coercion.

## Non-Goals

- Do not make slow upstreams faster by itself.
- Do not alter Gemini native `/v1beta` routes in this change.
- Do not change pricing, model mapping, group selection, or account credentials.
- Do not add DNS or Nginx dependency for the first version.

## Public Paths

The first version adds path aliases under the existing domain:

- Normal: `https://api.passionapi.com/v1`
- Force stream: `https://api.passionapi.com/relay-stream/v1`
- Force non-stream: `https://api.passionapi.com/relay-nonstream/v1`

For clients whose base URL must include `/v1`, they can use the alias directly, for example:

- `https://api.passionapi.com/relay-stream/v1`
- `https://direct.passionapi.com/relay-stream/v1`

## Behavior

### Force Stream

For `/relay-stream/v1/chat/completions` and `/relay-stream/v1/responses`:

- If the client sends `stream=false` or omits `stream`, the gateway rewrites the parsed request body to `stream=true`.
- The response is streamed back to the client using the normal protocol for the inbound endpoint.
- Usage and outcome should be recorded as stream because the downstream wire response is stream.

This is the path most relevant to the mobile timeout case: it lets the client receive early SSE events instead of waiting for one large JSON response.

### Force Non-Stream

For `/relay-nonstream/v1/chat/completions` and `/relay-nonstream/v1/responses`:

- If the client sends `stream=true`, the gateway rewrites the parsed request body to `stream=false`.
- The response is returned as one JSON object.
- Usage and outcome should be recorded as sync because the downstream wire response is JSON.

This helps clients that cannot consume SSE when a model alias or provider defaults to stream.

## Architecture

Introduce a small request-scoped relay mode:

- `normal`
- `force_stream`
- `force_nonstream`

The route layer sets the mode in `gin.Context` based on the path prefix. The existing OpenAI-compatible handlers read the mode after body parsing and before validation/audit/routing. They rewrite only the top-level `stream` field and leave all other request data untouched.

This keeps behavior local to OpenAI-compatible conversation handlers and avoids modifying account scheduling, channel mapping, billing, or upstream clients.

## Components

- `backend/internal/handler/relay_stream_mode.go`
  - Context constants and helpers.
  - JSON body helper that sets `stream` to a boolean using structured JSON.
- `backend/internal/server/routes/gateway.go`
  - Register `/relay-stream/v1` and `/relay-nonstream/v1` OpenAI-compatible route groups.
  - Reuse the same middleware chain as `/v1`.
- `backend/internal/handler/openai_chat_completions.go`
  - Apply the relay mode after JSON validity checks and before `parseOpenAICompatibleStream`.
- `backend/internal/handler/openai_gateway_handler.go`
  - Apply the same behavior in `Responses`.

## Error Handling

- Invalid JSON remains invalid JSON; relay mode does not mask parse errors.
- Invalid `stream` types are replaced by a valid boolean only in relay mode, because the explicit purpose is mode coercion.
- Unsupported subpaths keep the existing `Unsupported responses subpath` behavior.
- Non-OpenAI-compatible groups continue to route through the existing platform auto-dispatch after the same mode marker is set.

## Testing

- Unit tests for mode helper:
  - Missing `stream` becomes `true` in force-stream mode.
  - `stream=false` becomes `true`.
  - `stream=true` becomes `false` in force-nonstream mode.
  - Invalid JSON returns an error.
- Handler-level tests:
  - `/relay-stream/v1/chat/completions` converts `stream:false` before validation.
  - `/relay-nonstream/v1/chat/completions` converts `stream:true`.
  - `/relay-stream/v1/responses` converts `stream:false`.
  - Normal `/v1/chat/completions` remains unchanged.
- Route tests:
  - Aliases are registered and share API key/auth middleware.

## Rollout

Deploy as an opt-in feature. Existing customers stay on `/v1`. For the affected mobile client, ask them to use:

`https://api.passionapi.com/relay-stream/v1`

If their app still times out or does not support SSE, the remaining solution is upstream/account improvement or client timeout adjustment.
