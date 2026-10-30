# Gemini Upstream Prefix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Safely forward tagged Gemini upstream names and import the selected metered models into the requested group.

**Architecture:** Keep shared/native public validators strict. Validate tagged final AI Studio model names separately and escape exactly one path segment; allow mapped native aliases only for AI Studio branches.

**Tech Stack:** Go, net/url, httptest, PostgreSQL read-only reconciliation, Admin API, GitHub Actions, Docker and Nginx.

### Task 1: Reproduce and repair

1. Run the existing Gemini URL tests as baseline.
2. Add URL wire-format, tagged generation/stream/countTokens, invalid prefix, strict public/shared guard and native-alias regression tests in `backend/internal/service/gemini_upstream_url_test.go` and `gemini_native_alias_test.go`.
3. Run targeted tests; confirm valid tagged cases fail for the current validator.
4. Add the private scoped validator and URL encoding in `gemini_upstream_url.go`; adjust only AI Studio mapped-alias validation in `gemini_messages_compat_service.go`.
5. Run targeted tests and same-account input-count recovery regression, then the appropriate service/handler tests and independent security/code review. Address findings before committing.

### Task 2: Build and release

1. Verify clean integration and unchanged `VERSION`; commit/push the exact changes to the feature branch and dev without overwriting concurrent work.
2. Wait for required CI and deployment-image build, download the exact SHA artifact and verify archive checksum and binary/image identity.
3. Adapt and test the existing release helper for current 0.2.19 baseline with no new migrations. Discover actual containers/routes/ports; preserve config, mounts, network, source blocks and protected services.
4. Prepare new instances, run readiness and isolated prefix/billing acceptance, switch Nginx smoothly, verify both HTTPS entries and stop old app workers only after draining. Retain rollback containers.

### Task 3: Import and verify

1. Re-read group 121 and source account 6010, preserving the original account and group settings.
2. Create an independent Gemini API-key account with seven exact ordinary-name to `[量]` mappings, limited concurrency, no automatic probe/rate synchronization. Keep it off production traffic until direct account tests pass.
3. Test every mapped model and actual gateway forwarding using dedicated isolated resources where needed. Bind only the intended group after verification; do not borrow customer keys or wallets.
4. Read back mapping, status and group binding; clean isolated test resources, reconcile charges and report actual verified deployment/account identifiers and the upstream tool limitation.
