# Unreleased Features Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development for every behavior change.

**Goal:** Integrate four completed but unreleased Sub2API behaviors into the current `dev` branch without reverting newer production work.

**Architecture:** Port tests and implementation by behavior rather than merging the stale branch. Reuse current service/repository interfaces, route wiring, Vue components, and gateway accounting paths.

**Tech Stack:** Go, Gin, Ent/PostgreSQL repositories, Vue 3, TypeScript, Vitest.

---

### Task 1: Self-service account deletion

**Files:**
- Modify: `backend/internal/handler/user_handler.go`
- Modify: `backend/internal/service/user_service.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/cmd/server/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Modify: `frontend/src/api/user.ts`
- Create: `frontend/src/components/user/profile/ProfileDangerZoneCard.vue`
- Modify: `frontend/src/views/user/ProfileView.vue`
- Test: corresponding handler, service, API, component, and profile tests

1. Port focused tests and run them to verify the endpoint/component is absent.
2. Implement password confirmation, admin rejection, transactional key/user deletion, cache invalidation, session revocation, and the profile UI.
3. Run focused backend/frontend tests and commit.

### Task 2: Deleted-identity registration tombstones

**Files:**
- Modify: `backend/internal/repository/user_repo.go`
- Modify: `backend/internal/service/registration_email_alias.go`
- Modify: `backend/internal/service/auth_service.go`
- Modify: verified-email OAuth registration services
- Test: repository, password registration, verification-code, and OAuth registration tests

1. Port tests proving deleted exact emails and aliases are rejected.
2. Add the include-deleted repository capability and use it only in guarded registration paths.
3. Run focused service/repository tests and commit.

### Task 3: Read-only model lists with exhausted balance

**Files:**
- Modify: `backend/internal/server/middleware/api_key_auth.go`
- Modify: `backend/internal/server/middleware/api_key_auth_google.go`
- Test: `backend/internal/server/middleware/api_key_auth_test.go`
- Test: `backend/internal/server/middleware/api_key_auth_google_test.go`

1. Add failing tests for exhausted-balance model-list requests.
2. Add an exact GET model-list classifier and bypass only balance/subscription quota checks.
3. Run all API-key middleware tests and commit.

### Task 4: Native Gemini Image Studio

**Files:**
- Modify: `backend/internal/service/image_studio_gateway.go`
- Modify: `backend/internal/service/image_studio_service.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Test: Image Studio gateway/service tests

1. Port request conversion, dispatch, response parsing, and usage-recording tests and verify they fail.
2. Implement the Gemini gateway adapter and current dependency wiring.
3. Run focused Image Studio and Gemini gateway tests and commit.

### Task 5: Full verification

1. Run `go test` for all touched backend packages.
2. Run frontend focused tests and both type-check commands.
3. Run the repository critical test target.
4. Review the final diff against `dev`, confirm only intended files changed, and report any residual risk.
