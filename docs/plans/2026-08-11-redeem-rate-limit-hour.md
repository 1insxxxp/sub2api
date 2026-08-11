# Redeem Rate Limit Hour Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Correct the user redemption failure window from 24 hours to the documented one hour without changing the 20-failure threshold or redemption semantics.

**Architecture:** Keep the existing Redis-backed counter and fail-open behavior. Add a unit policy assertion around the repository TTL constant, then make the minimal constant change so the repository and service policy agree.

**Tech Stack:** Go, Redis repository, testify, Go build tags.

---

### Task 1: Correct the redemption rate-limit TTL

**Files:**
- Modify: `backend/internal/repository/redeem_cache_test.go`
- Modify: `backend/internal/repository/redeem_cache.go`

**Step 1: Write the failing test**

Add a unit test that locks the documented policy:

```go
func TestRedeemRateLimitDurationMatchesHourlyPolicy(t *testing.T) {
	require.Equal(t, time.Hour, redeemRateLimitDuration)
}
```

Add `time` to the test imports.

**Step 2: Run the test to verify it fails**

Run:

```bash
cd backend
go test -tags=unit ./internal/repository -run TestRedeemRateLimitDurationMatchesHourlyPolicy -count=1
```

Expected: FAIL because the current repository duration is 24 hours.

**Step 3: Write the minimal implementation**

Change the repository constant:

```go
redeemRateLimitDuration = time.Hour
```

Do not change the threshold, counter key, error semantics, or lock TTL.

**Step 4: Run focused and related tests**

Run:

```bash
go test -tags=unit ./internal/repository -run 'TestRedeem' -count=1
go test -tags=unit ./internal/service -run 'Redeem' -count=1
gofmt -w internal/repository/redeem_cache.go internal/repository/redeem_cache_test.go
git diff --check
```

Expected: all tests and checks pass.

**Step 5: Commit**

```bash
git add backend/internal/repository/redeem_cache.go backend/internal/repository/redeem_cache_test.go
git commit -m "fix: restore hourly redeem failure limit"
```
