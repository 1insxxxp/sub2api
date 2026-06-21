# Registration Abuse Control Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add persistent registration source tracking and a settings-backed auth IP blacklist without requiring invitation-code registration.

**Architecture:** The auth handler performs request-context extraction and blacklist enforcement. The auth service carries source metadata through registration/login flows. The user repository persists the metadata via Ent fields and a SQL migration.

**Tech Stack:** Go, Gin, Ent, PostgreSQL SQL migrations, existing setting repository.

---

### Task 1: Add Settings Model and Tests

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/setting_service.go`
- Test: `backend/internal/service/auth_ip_blacklist_test.go`

**Steps:**
1. Write failing unit tests for disabled defaults, exact IP matching, CIDR matching, invalid rule normalization, and enabled blacklist blocking.
2. Add `SettingKeyAuthIPBlacklistEnabled` and `SettingKeyAuthIPBlacklistRules`.
3. Add `AuthIPBlacklistSettings`, `GetAuthIPBlacklistSettings`, `SetAuthIPBlacklistSettings`, and `IsAuthIPBlocked`.
4. Run the focused service tests.

### Task 2: Persist User Source Metadata

**Files:**
- Modify: `backend/ent/schema/user.go`
- Create: `backend/migrations/154_add_user_auth_source_metadata.sql`
- Modify generated Ent files with `go generate ./ent`
- Modify: `backend/internal/service/user.go`
- Modify: `backend/internal/repository/user_repo.go`
- Test: `backend/internal/repository/user_repo_auth_metadata_test.go`

**Steps:**
1. Write a repository test that creates a user with registration metadata and reads it back.
2. Add nullable/default metadata fields to the Ent user schema and migration.
3. Update service model and repository create/update/read mapping.
4. Run the focused repository tests.

### Task 3: Enforce Blacklist in Auth Flows

**Files:**
- Modify: `backend/internal/handler/auth_handler.go`
- Modify: `backend/internal/service/auth_service.go`
- Test: `backend/internal/handler/auth_handler_ip_blacklist_test.go`
- Test: `backend/internal/service/auth_service_auth_metadata_test.go`

**Steps:**
1. Write failing tests for blocked verification/register/login and for registration/login metadata writes.
2. Add request IP/UA extraction and `ensureAuthIPAllowed`.
3. Pass source metadata into registration.
4. Update successful login metadata after authentication.
5. Run the focused handler and auth service tests.

### Task 4: Add Admin API

**Files:**
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Test: `backend/internal/handler/admin/setting_handler_auth_ip_blacklist_test.go`

**Steps:**
1. Write failing handler tests for GET/PUT blacklist settings.
2. Add request/response DTOs.
3. Register admin routes under settings.
4. Run the focused admin handler tests.

### Task 5: Verify, Commit, Deploy

**Steps:**
1. Run `go test` for touched backend packages.
2. Run the backend build command used by the repo.
3. Commit and push.
4. Deploy with the existing non-disruptive container rollout and verify `/health`.
