# Disable Downstream Billing Probe Design

## Goal

Prevent downstream services that possess a valid API key from reading this
deployment's billing multiplier through `GET /v1/sub2api/billing`.

## Scope

- Add an explicit gateway configuration switch for the billing probe endpoint.
- Keep the switch disabled by default.
- Return `404 Not Found` when the endpoint is disabled, including for a valid API
  key, so callers treat the capability as unsupported.
- Preserve normal model requests, billing calculations, usage records, and the
  existing user and administrator dashboard displays.
- Do not change database data or existing group and user-specific multipliers.

## Architecture and Data Flow

The route remains registered so deployments can opt in without rebuilding. The
handler checks the configuration before resolving the authenticated key's group
or user multiplier. When disabled, it returns the existing safe `not_found_error`
shape and performs no multiplier lookup. When enabled, the current response and
authentication behavior remain unchanged.

The setting belongs to the gateway configuration because it controls a gateway
capability. Its zero value is false, making new and upgraded deployments private
by default. The example configuration documents the setting and its implications.

## Error Handling

Disabled requests return HTTP 404 with no multiplier-related fields. Existing
errors for missing keys, unassigned groups, simple mode, and unavailable billing
services remain unchanged when the feature is enabled.

## Testing

- Verify the default/disabled configuration returns 404 for an authenticated key.
- Verify an explicitly enabled configuration returns the existing billing data.
- Keep route-level coverage to ensure the public path cannot reveal multiplier
  fields while disabled.
- Run focused backend handler and route tests.

## Deployment

This change is local only for the initial implementation. A later production
deployment should use the server's existing blue-green process and verify both
public domains return 404 for the endpoint before completing the cutover.
