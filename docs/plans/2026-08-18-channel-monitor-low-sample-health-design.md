# Channel Monitor Low-Sample Health Design

## Context

The five-minute availability matrix currently produces two misleading states:

- A sparse bucket with counted failures can remain green because disabled cache scoring contributes a synthetic score of 100.
- A sparse bucket with only failures can be grey because request error scoring is withheld until `minimum_sample` is reached, making it indistinguishable from no traffic.

Production evidence for group 44 showed both cases: a 3-request bucket with 2 counted failures could be green, while later 100%-failure buckets with 2–6 requests were grey. The failures were `rate_or_capacity` and `upstream_5xx`, not ignored categories.

## Considered approaches

1. Lower `minimum_sample` globally to 1. This reacts quickly but makes every metric, including latency percentiles, excessively noisy.
2. Mark every bucket containing any error as critical. This is simple but overreacts to one failure among many successes and bypasses the configured warning/critical thresholds.
3. Keep the configured sample threshold for confident scoring, but add a conservative low-sample error fallback and remove disabled cache scoring from the composite. This preserves stable high-volume scoring while preventing known failures from appearing healthy. This is the selected approach.

## Selected behavior

### Backend scoring

- When both cache thresholds are zero, cache is disabled and contributes no health component. It must not contribute a synthetic score of 100.
- Normal error-rate scoring remains unchanged when `request_count >= minimum_sample`.
- When `0 < request_count < minimum_sample`:
  - no counted errors keeps error health `unknown`;
  - counted error rate at or above the configured critical threshold yields a critical low-sample error score;
  - counted error rate at or above the warning threshold yields a warning low-sample error score;
  - a lower counted error rate remains unknown.
- Ignored error categories continue to be removed before this decision, so customer-caused errors do not mark the channel unhealthy.
- TTFT and enabled cache scoring continue to require their existing sample thresholds.

### Frontend presentation

- A bucket with a backend score uses its normal health color, including low-sample warning/critical buckets.
- A bucket with traffic but no score remains `sample insufficient` and receives a distinct neutral style.
- A missing bucket remains `no traffic` and keeps the existing lighter grey style.
- Tooltips explicitly label low-sample buckets so operators do not interpret them as statistically confident.

## Tests

- Backend unit tests reproduce the disabled-cache green bug and sparse 100%-failure grey bug before the fix.
- Tests prove ignored sparse failures remain unknown and normal above-threshold scoring does not change.
- Frontend tests distinguish no traffic, insufficient samples, and low-sample critical buckets, including tooltip text.
- Run focused service/frontend tests, broader affected suites, typecheck, lint, vet, and build checks before committing.
