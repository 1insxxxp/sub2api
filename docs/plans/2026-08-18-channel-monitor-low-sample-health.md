# Channel Monitor Low-Sample Health Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent sparse counted failures from appearing green or indistinguishable from no traffic in the Channel Monitor V2 availability matrix.

**Architecture:** Keep the existing configured sample threshold for statistically stable scoring, but add a low-sample error fallback for counted errors. Treat zero/zero cache thresholds as a disabled signal rather than a perfect signal, expose a privacy-safe `low_sample` flag, and render no traffic separately from insufficient samples.

**Tech Stack:** Go service/repository tests, Vue 3 + TypeScript, Vitest, Tailwind CSS, Go unit tests/vet.

---

### Task 1: Reproduce backend scoring defects

**Files:**
- Modify: `backend/internal/service/channel_monitor_v2_test.go`
- Modify: `backend/internal/service/channel_monitor_v2.go`

**Step 1: Write failing tests**

Add tests that model the production buckets:

```go
func TestChannelMonitorV2SparseCountedFailuresCannotBeHealthy(t *testing.T) {
    thresholds := DefaultChannelMonitorV2HealthThresholds()
    health := ChannelMonitorV2HealthForWithThresholds(ChannelMonitorV2Metric{
        RequestCount: 3,
        ErrorRate: 2.0 / 3.0,
        CacheRateDenominator: 204341,
    }, thresholds)
    require.True(t, health.LowSample)
    require.Equal(t, "critical", health.ErrorRate)
    require.Equal(t, "critical", health.Overall)
    require.Nil(t, health.CacheScore)
}
```

Also prove a sparse all-ignored bucket (`ErrorRate: 0`) remains unknown, zero traffic is not low-sample, and above-threshold behavior remains unchanged.

**Step 2: Run tests to verify RED**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'ChannelMonitorV2.*(Sparse|DefaultHealth)' -count=1
```

Expected: FAIL because disabled cache contributes 100 and sparse error scoring is absent.

**Step 3: Implement minimal backend behavior**

- Add `LowSample bool \`json:"low_sample"\`` to `ChannelMonitorV2Health`.
- Set it when `0 < RequestCount < MinimumSample`.
- For low samples, add an error component only when the counted `ErrorRate` crosses warning/critical thresholds.
- Skip the cache component entirely when both cache thresholds are zero.

**Step 4: Run focused tests to verify GREEN**

Run the same focused command; expect PASS.

### Task 2: Reproduce and fix frontend state ambiguity

**Files:**
- Modify: `frontend/src/api/channelMonitorV2.ts`
- Modify: `frontend/src/features/channel-monitor-v2/monitorFormat.ts`
- Modify: `frontend/src/features/channel-monitor-v2/RelayPulseMatrix.vue`
- Modify: `frontend/src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts`
- Modify: `frontend/src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/channelMonitorV2.ts`
- Modify: `frontend/src/i18n/locales/en/channelMonitorV2.ts`

**Step 1: Write failing frontend tests**

Add assertions that:

- `low_sample=true` with no score maps to `health-insufficient`;
- a missing matrix slot maps to `health-no-traffic`;
- low-sample critical score stays red and its tooltip includes a low-sample warning;
- the legend exposes distinct no-traffic and insufficient-sample entries.

**Step 2: Run tests to verify RED**

Run:

```bash
pnpm --dir frontend exec vitest run \
  src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
  src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts
```

Expected: FAIL because both cases currently use `health-unknown` and no low-sample text exists.

**Step 3: Implement minimal presentation changes**

- Add optional `low_sample` to `MonitorHealth`.
- Return `health-insufficient` when a bucket is low-sample and has no selected-mode score.
- Render missing slots as `health-no-traffic`.
- Add separate CSS colors, legends, and localized tooltip text.
- Keep redacted payload score behavior and ignored-category behavior unchanged.

**Step 4: Run focused tests to verify GREEN**

Run the same Vitest command; expect PASS.

### Task 3: Verify affected behavior and commit

**Files:**
- Verify all files from Tasks 1 and 2.

**Step 1: Run backend verification**

```bash
cd backend
go test -tags=unit ./internal/service ./internal/repository ./internal/handler -run 'ChannelMonitorV2' -count=1
go vet -tags=unit ./internal/service ./internal/repository ./internal/handler
```

Expected: PASS.

**Step 2: Run frontend verification**

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run typecheck:tests
pnpm --dir frontend run lint:check
pnpm --dir frontend exec vitest run src/features/channel-monitor-v2
pnpm --dir frontend run build
```

Expected: PASS; only repository-known non-failing warnings are acceptable.

**Step 3: Review and commit**

```bash
git diff --check
git status --short
git add backend/internal/service/channel_monitor_v2.go \
  backend/internal/service/channel_monitor_v2_test.go \
  frontend/src/api/channelMonitorV2.ts \
  frontend/src/features/channel-monitor-v2/monitorFormat.ts \
  frontend/src/features/channel-monitor-v2/RelayPulseMatrix.vue \
  frontend/src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
  frontend/src/features/channel-monitor-v2/__tests__/RelayPulseMatrix.spec.ts \
  frontend/src/i18n/locales/zh/channelMonitorV2.ts \
  frontend/src/i18n/locales/en/channelMonitorV2.ts
git commit -m "fix: surface sparse channel failures"
```

Do not add or modify unrelated untracked workspace files.
