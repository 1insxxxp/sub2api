# Available Channels Group Pricing Fallback Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make group-only models on the user available-channels page display global catalog pricing when channel pricing is absent.

**Architecture:** Expose a display-only global pricing fallback through `ChannelService`, then use it only when the handler cannot match a group model to the channel's supported-model list. Existing channel pricing remains authoritative, and the billing path is unchanged.

**Tech Stack:** Go, Gin handler DTOs, testify.

---

### Task 1: Add regression coverage for group-only models

**Files:**
- Modify: `backend/internal/handler/available_channel_handler_test.go`

**Step 1: Write the failing test**

Add a handler/service integration-style unit test whose group availability contains `claude-opus-5`, whose channel supported models omit it, and whose pricing service catalog contains the model. Assert that the returned group model has non-nil input and output display prices.

Also assert in focused cases that an existing channel price wins and an unknown global model stays unpriced.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler -run 'TestAvailableChannelHandler.*Group.*Pricing' -count=1`

Expected: FAIL because the unmatched group model currently has `Pricing == nil`.

### Task 2: Implement the display-only fallback

**Files:**
- Modify: `backend/internal/service/channel_available.go`
- Modify: `backend/internal/handler/available_channel_handler.go`
- Test: `backend/internal/handler/available_channel_handler_test.go`

**Step 1: Add a display-pricing lookup**

Add a `ChannelService` method that returns a synthesized `ChannelModelPricing` from the global catalog, or nil when unavailable. Reuse `synthesizePricingFromLiteLLM`; do not add database writes or billing behavior.

**Step 2: Apply it only for unmatched group models**

Change the group-model DTO conversion so a channel match remains the first choice. Only when no channel model matches should it call the display fallback and convert that result.

**Step 3: Run the focused tests**

Run: `go test ./internal/handler ./internal/service -run 'AvailableChannel|GlobalPricingFallback' -count=1`

Expected: PASS.

### Task 3: Verify and commit

**Files:**
- Verify all modified Go files.

**Step 1: Format and inspect**

Run: `gofmt -w internal/handler/available_channel_handler.go internal/handler/available_channel_handler_test.go internal/service/channel_available.go`

Run: `git diff --check`

Expected: no output from `git diff --check`.

**Step 2: Run backend verification**

Run: `go test ./...`

Expected: PASS.

**Step 3: Commit**

```bash
git add backend/internal/handler/available_channel_handler.go \
  backend/internal/handler/available_channel_handler_test.go \
  backend/internal/service/channel_available.go \
  docs/plans/2026-08-04-available-channels-group-pricing-fallback-design.md \
  docs/plans/2026-08-04-available-channels-group-pricing-fallback.md
git commit -m "fix: show fallback pricing for group models"
```
