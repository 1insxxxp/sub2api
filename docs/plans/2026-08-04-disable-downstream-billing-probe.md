# Disable Downstream Billing Probe Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Hide billing multipliers from downstream API-key holders while preserving all normal gateway, billing, and dashboard behavior.

**Architecture:** Add a boolean gateway setting named `billing_probe_enabled`, defaulting to false through Go's zero value. Keep the route registered, but have the handler return the existing JSON 404 response before resolving any multiplier unless the setting is explicitly enabled.

**Tech Stack:** Go, Gin, Viper/mapstructure YAML configuration, Testify.

---

### Task 1: Specify Disabled and Enabled Handler Behavior

**Files:**
- Modify: `backend/internal/handler/gateway_key_billing_test.go`

**Step 1: Write the failing disabled-by-default test**

Add a test that constructs an authenticated grouped API key, invokes a handler with a standard-mode configuration whose `Gateway.BillingProbeEnabled` is false, and asserts HTTP 404, no multiplier fields, and zero user-rate repository lookups.

**Step 2: Preserve enabled behavior explicitly**

Update existing successful handler tests to construct the handler with `Gateway.BillingProbeEnabled: true`. This makes the compatibility expectation explicit.

**Step 3: Run the focused handler test**

Run:

```bash
cd backend && go test ./internal/handler -run 'KeyBilling|BuildKeyBilling' -count=1
```

Expected: the new test fails because the configuration field and guard do not exist yet.

### Task 2: Add the Configuration and Handler Guard

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/handler/gateway_key_billing.go`
- Modify: `deploy/config.example.yaml`

**Step 1: Add the gateway setting**

Add this field to `GatewayConfig`:

```go
// BillingProbeEnabled allows API-key holders to query their effective billing multiplier.
// Disabled by default to avoid exposing rate information to downstream services.
BillingProbeEnabled bool `mapstructure:"billing_probe_enabled"`
```

**Step 2: Guard the endpoint**

After retrieving the API key but before mode/group/rate processing, return:

```go
if h.cfg == nil || !h.cfg.Gateway.BillingProbeEnabled {
    h.errorResponse(c, http.StatusNotFound, "not_found_error", "Billing information is not supported")
    return
}
```

**Step 3: Document the setting**

Add `billing_probe_enabled: false` under `gateway:` in `deploy/config.example.yaml`, noting that enabling it exposes effective multiplier data to any valid API key holder.

**Step 4: Run the focused handler tests**

Run:

```bash
cd backend && go test ./internal/handler -run 'KeyBilling|BuildKeyBilling' -count=1
```

Expected: PASS.

### Task 3: Verify Route-Level Privacy

**Files:**
- Modify: `backend/internal/server/routes/gateway_key_billing_test.go`

**Step 1: Parameterize the route test helper**

Allow the helper to set `cfg.Gateway.BillingProbeEnabled` independently of run mode.

**Step 2: Add the default-disabled route assertion**

For a valid bearer key in standard mode with the flag disabled, assert HTTP 404, the safe JSON error shape, absence of all multiplier field names, and zero rate lookups.

**Step 3: Keep the existing success contract behind opt-in**

Set the flag true in the current standard-mode success test and retain its HTTP 200 and effective multiplier assertions.

**Step 4: Run route tests**

Run:

```bash
cd backend && go test ./internal/server/routes -run 'KeyBilling' -count=1
```

Expected: PASS.

### Task 4: Format and Verify the Complete Change

**Files:**
- Verify all files modified above.

**Step 1: Format Go files**

Run:

```bash
cd backend && gofmt -w internal/config/config.go internal/handler/gateway_key_billing.go internal/handler/gateway_key_billing_test.go internal/server/routes/gateway_key_billing_test.go
```

**Step 2: Run all focused tests together**

Run:

```bash
cd backend && go test ./internal/handler ./internal/server/routes -run 'KeyBilling|BuildKeyBilling' -count=1
```

Expected: PASS.

**Step 3: Run configuration tests**

Run:

```bash
cd backend && go test ./internal/config -count=1
```

Expected: PASS.

**Step 4: Review the diff**

Confirm the diff only changes the probe feature setting, endpoint guard, documentation, and tests. Confirm `package.json` and `package-lock.json` remain untouched.

**Step 5: Commit the implementation**

```bash
git add backend/internal/config/config.go backend/internal/handler/gateway_key_billing.go backend/internal/handler/gateway_key_billing_test.go backend/internal/server/routes/gateway_key_billing_test.go deploy/config.example.yaml
git commit -m "feat: disable downstream billing probe by default"
```
