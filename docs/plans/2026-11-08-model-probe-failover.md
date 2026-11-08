# Model Probe Request Failover Implementation Plan

> Execute with executing-plans and test-driven-development. The user explicitly requested the same retry/switch rules as real requests; implementation proceeds under that instruction.

**Goal:** Workbench probes must determine availability through normal account selection and request failover, not terminate after an arbitrary single-account native test.

**Architecture:** Retain service-side staff authorization, enabled-group policy, public catalog validation and category-source resolution. Delegate execution to the gateway handler, reusing real per-account forwarding (typed failures, first-output guards) and the protocol's existing failover policy. The probe owns a bounded discard writer and independent internal session; no customer API key, billing operation or usage record is created. Public DTOs remain closed and unchanged.

**Tech Stack:** Go/Gin, Vue/TypeScript, existing gateway schedulers and forwarders.

## Decisions
- A native-test retry wrapper was considered but rejected because tests erase typed failures and do not share first-output enforcement.
- Dispatching through public handlers would require a billing/authentication bypass; instead call the same forwarding services and shared retry-policy helpers directly.
- Success needs complete upstream completion evidence. A response that has already produced semantic output cannot be replayed simply because the probe discards content.
- All retry attempts remain in the resolved source/routing scope. No account or upstream identifiers enter the public response.
- Keep existing three-model UI concurrency and 120-second overall administrative safety timeout. Within this budget use the configured per-account first-output timeout and existing account-switch limits. Clarify the total safety cap in UI/docs; no blanket retries for nonretryable failures.

## Task 1: Lock the service execution boundary
Files: backend/internal/service/model_probe.go, model_probe_test.go, model_probe_category_default_test.go, model_probe_visibility_test.go; backend/internal/server/routes/model_probes_test.go; backend/cmd/server/wire_gen.go.
1. Add failing regression proving authorized Run delegates to full gateway execution rather than the legacy single-account runner.
2. Run `go test -p 2 -tags=unit ./internal/service -run TestModelProbeUsesGatewayExecution -count=1` and observe failure.
3. Replace the single-account service orchestration with the full execution interface; preserve authorization/source checks and safe result projection.
4. Update fixtures to exercise the new boundary, rerun focused service/routes tests.

## Task 2: Gateway failover orchestration
Files: backend/internal/handler/model_probe_execution.go and model_probe_execution_test.go; model_probe_gateway.go.
1. Cover retryable failure then success, nonretryable error, total cancellation, timeout pool traversal, switch exhaustion, write-after-output safety, no-source-crossing, per-attempt slot release.
2. Reuse gateway selection with exclusions and independent sticky session, existing FailoverState for native Gemini/Anthropic paths, existing OpenAI helpers/OAuth429 policy for OpenAI-compatible paths.
3. Use one first-output traversal budget and restore request context between attempts; separate real timeout from cancellation.
4. Keep bounded response capture and require successful completion evidence; no error body or content is returned.

## Task 3: Real forwarding adapters (parallel, disjoint files)
Files: backend/internal/handler/model_probe_forward.go and model_probe_forward_test.go.
1. Use real forwarding services for supported text/media routes, honoring aliases and mappings once.
2. Preserve typed errors, native endpoint restrictions and explicit unsupported modes.
3. Preserve supported media behavior; never resubmit an accepted asynchronous job after a poll failure.
4. Add fake-upstream regression coverage; parent serializes tests.

## Task 4: Explain and verify
Files: frontend/src/i18n/locales/{zh,en}/modelProbes.ts, AGENTS.md.
1. Update wording to explain normal-request failover, three-model concurrency and overall administrative limit.
2. Run affected handler/service/routes tests with Go `-p 2` and frontend model-probe tests; then appropriate broader checks, only one heavy group at a time.
3. Review authorization, privacy, timeout, no-billing and resource-release invariants before integration. No unrelated request-outcome draft is included.

## Local verification environment
Use per-command GOMAXPROCS=2, GOFLAGS=-p=2, GOCACHE=/Volumes/ORICO/sub2-support-model-checks/go-cache, GOTMPDIR=/Volumes/Sub2SupportChecks/tmp. These avoid main-disk pressure and exFAT executable corruption; do not change Go global env or production. Coordinate all heavy checks with other agents.

## Implemented and verified

- Complete gateway execution replaces the single native-account runner. Actual production forwarders preserve typed failures and native/compatible endpoint conversion.
- Per-channel retry policy, source-scoped exclusions, account admission, independent sticky session and request-scoped timeout/profit state are preserved. Actor account exclusions also apply to probes.
- Success requires completed output; partial output and accepted asynchronous tasks cannot be replayed. Media stops retain cancellation instead of entering customer billing drain.
- Customer usage and accounting repositories remain untouched (non-nil write guards cover success, failed and interrupted production forwarding).
- Targeted backend regressions passed in handler, service, routes and admin packages, including existing timeout, same-account, OAuth429, single-account and upstream-context cases. Frontend probe/API/locale tests: 45 passed in 4 files.
- Server binary compiled successfully with `go build -p 2 ./cmd/server` (output stored outside the repo).
- Independent review findings were resolved and regression tested. No production configuration, deployment or database changes were performed.
