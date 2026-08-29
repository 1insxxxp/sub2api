# Balance Group Image Pricing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `gpt-image-*` generation cost exactly `$0.375` per image in the three OpenAI balance groups.

**Architecture:** Add a scoped group-level image pricing override, which takes precedence over the Codex channel's token pricing. Set the image multiplier independently to `1.0` so text multipliers do not affect image charges.

**Tech Stack:** PostgreSQL, Go administrative group service, Redis-backed authentication cache, Docker Compose deployment

---

### Task 1: Capture the rollback baseline

**Files:**
- No repository files modified

1. Read groups 2, 9, and 25, including `model_pricing`, image prices, image multiplier settings, limits, routing, and status.
2. Save the returned JSON outside the application database for rollback during this operation.

### Task 2: Apply scoped group pricing

**Files:**
- No repository files modified

1. Preserve every existing model-pricing entry.
2. Add one OpenAI rule matching `gpt-image-*` with `billing_mode=image` and `per_request_price=0.375`.
3. Set `image_rate_independent=true` and `image_rate_multiplier=1.0`.
4. Update each group through the administrative service path with its complete current limits and settings.

### Task 3: Verify effective pricing

**Files:**
- No repository files modified

1. Re-read all three database rows and verify the persisted JSON.
2. Verify group and authentication caches were invalidated.
3. Resolve a one-image `gpt-image-2` calculation and expect a final charge of `$0.375` in each target group.
4. Confirm groups 48, 49, 50, 51, 58, 62, and 70 are unchanged.

### Task 4: Roll back on any mismatch

**Files:**
- No repository files modified

1. Restore the captured `model_pricing`, `image_rate_independent`, and `image_rate_multiplier` values.
2. Invalidate the same caches.
3. Re-read the rows and report the rollback result.
