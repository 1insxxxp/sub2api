# Locked Affiliate Referral Registration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When a new user enters registration through a valid affiliate invitation link, preserve inviter attribution in a server-signed browser lock, hide the editable affiliate-code field, and apply the same attribution to email and every supported OAuth registration path. Direct registration without a valid lock keeps the current editable field.

**Architecture:** Add a public, rate-limited referral resolver and status API. A valid code is normalized and checked by the affiliate service, then stored in a versioned HMAC-signed HttpOnly cookie for 30 days. Registration and OAuth handlers treat a valid cookie as authoritative and ignore conflicting client aff_code; without a valid cookie they preserve current manual-code behavior. The frontend resolves URL codes through the server, hides the field only when the server reports a valid lock, and falls back to the legacy editable flow during a blue/green version mismatch.

**Tech Stack:** Go, Gin, HMAC-SHA256, Vue 3, TypeScript, Vitest, Vue Test Utils.

---

## Constraints and invariants

- No database migration; this is browser-scoped pre-registration state.
- Only a newly validated affiliate link can replace an existing lock.
- Invalid, unknown, malformed, expired, or tampered input never replaces a valid lock.
- A valid lock is authoritative over aff_code from JSON, query parameters, local storage, or OAuth completion payloads.
- Without a valid lock, existing manual affiliate-code behavior remains available.
- The status endpoint and frontend state never expose the locked raw code.
- Clear the lock after successful account creation or successful login.
- Preserve old-backend/new-frontend and new-backend/old-frontend compatibility.
- Preserve the existing non-fatal affiliate-binding behavior.
- Do not touch the untracked root package.json or package-lock.json.

## Task 1: Add public affiliate-code validation to the service layer

**Files:**

- Modify: backend/internal/service/affiliate_service.go
- Modify: backend/internal/service/affiliate_service_test.go
- Modify: backend/internal/service/auth_service.go
- Modify: backend/internal/service/auth_service_register_test.go

### Step 1: Write failing service tests

Add tests for AffiliateService.ResolveValidCode:

- trims whitespace and uppercases a valid code;
- rejects malformed and unknown codes with ErrAffiliateCodeInvalid;
- returns service-unavailable when the repository is missing;
- refuses to issue a lock while the affiliate feature is disabled;
- does not create, bind, or mutate an affiliate record.

Add a focused AuthService.ResolveAffiliateReferralCode delegation test.

Run:

    cd backend
    go test ./internal/service -run 'TestAffiliateServiceResolveValidCode|TestAuthServiceResolveAffiliateReferralCode' -count=1

Expected: FAIL because the methods do not exist.

### Step 2: Implement the smallest service API

In AffiliateService.ResolveValidCode:

1. Normalize with strings.ToUpper(strings.TrimSpace(rawCode)).
2. Reject empty or malformed codes.
3. Require a configured repository and enabled affiliate feature.
4. Resolve via GetAffiliateByCode.
5. Convert ErrAffiliateProfileNotFound, nil summaries, and invalid user IDs to ErrAffiliateCodeInvalid.
6. Return only the normalized code; do not bind or mutate.

Add AuthService.ResolveAffiliateReferralCode as a thin wrapper so AuthHandler does not need direct access to AffiliateService.

### Step 3: Verify and commit

Run:

    cd backend
    go test ./internal/service -run 'TestAffiliateServiceResolveValidCode|TestAuthServiceResolveAffiliateReferralCode|TestRegisterWithVerification' -count=1
    git diff --check

Commit:

    git add backend/internal/service/affiliate_service.go backend/internal/service/affiliate_service_test.go backend/internal/service/auth_service.go backend/internal/service/auth_service_register_test.go
    git commit -m "feat: validate affiliate referral links"

## Task 2: Implement the signed referral-lock cookie and public APIs

**Files:**

- Create: backend/internal/handler/auth_affiliate_referral_lock.go
- Create: backend/internal/handler/auth_affiliate_referral_lock_test.go
- Modify: backend/internal/server/routes/auth.go

### Step 1: Write failing codec and endpoint tests

Cover:

- signed lock round-trips with normalized code and a 30-day expiry;
- modified payload/signature, wrong version, and expired payload are rejected;
- resolve endpoint sets the cookie only after service validation;
- a second valid code replaces the first lock;
- invalid input leaves an existing valid cookie untouched;
- status returns only locked true/false, never the code;
- missing/invalid/expired cookie returns locked false and clears only an invalid cookie;
- production cookie has HttpOnly, Secure, SameSite=Lax, Path=/, and 30-day Max-Age;
- development cookie remains HttpOnly and SameSite=Lax without requiring HTTPS.

Run:

    cd backend
    go test ./internal/handler -run 'TestAffiliateReferralLock|TestResolveAffiliateReferral|TestAffiliateReferralStatus' -count=1

Expected: FAIL because the codec and endpoints do not exist.

### Step 2: Implement the versioned lock

Create handler-local helpers with one cookie name, affiliate_referral_lock:

- payload: version, normalized affiliate code, issued-at, expires-at;
- encoding: base64url JSON plus base64url HMAC-SHA256 signature;
- signing key: a domain-separated derivation from cfg.JWT.Secret;
- verification: exact two-part format, strict version/time checks, hmac.Equal;
- no logging of raw cookies or raw affiliate codes.

Add:

- ResolveAffiliateReferral: validate JSON and service code, set the lock only on success, return valid and locked booleans;
- GetAffiliateReferralStatus: validate cookie and return only locked;
- affiliateCodeForRequest: valid lock wins, otherwise normalize submitted code;
- clearAffiliateReferralLock: expire the cookie with matching attributes.

### Step 3: Register rate-limited routes

In backend/internal/server/routes/auth.go add:

- POST /auth/affiliate-referral/resolve, 10/minute, fail-close;
- GET /auth/affiliate-referral/status, 30/minute.

Keep both behind the existing backend-mode guard and audit middleware.

### Step 4: Verify and commit

Run:

    cd backend
    go test ./internal/handler -run 'TestAffiliateReferralLock|TestResolveAffiliateReferral|TestAffiliateReferralStatus' -count=1
    go test ./internal/server -run 'Test.*AuthRoutes' -count=1
    git diff --check

Commit:

    git add backend/internal/handler/auth_affiliate_referral_lock.go backend/internal/handler/auth_affiliate_referral_lock_test.go backend/internal/server/routes/auth.go
    git commit -m "feat: lock validated affiliate referrals"

## Task 3: Make email registration consume the lock authoritatively

**Files:**

- Modify: backend/internal/handler/auth_handler.go
- Modify: backend/internal/handler/auth_handler_test.go
- Modify: backend/internal/service/auth_service_register_test.go

### Step 1: Write failing registration tests

Cover:

- valid lock plus conflicting JSON aff_code binds the locked inviter;
- valid lock plus empty JSON still binds the locked inviter;
- tampered/expired lock falls back to the submitted manual code;
- no lock preserves the current manual-code path;
- successful registration clears the lock;
- failed registration keeps the lock for retry;
- successful ordinary login clears an old lock;
- affiliate binding failure remains non-fatal.

Run:

    cd backend
    go test ./internal/handler -run 'TestRegister.*AffiliateReferral|TestLoginClearsAffiliateReferralLock' -count=1

Expected: FAIL because Register still trusts the body code.

### Step 2: Apply the authoritative code at the HTTP boundary

In AuthHandler.Register:

1. Resolve req.AffCode through affiliateCodeForRequest.
2. Pass only the resolved value to RegisterWithVerificationAndMetadata.
3. Clear the lock only after service success.

Update AuthHandler.respondWithTokenPair or its successful call sites to clear the referral lock before sending a successful auth response. Keep package-level test helper behavior stable.

### Step 3: Verify and commit

Run:

    cd backend
    go test ./internal/handler -run 'TestRegister|TestLogin|TestAffiliateReferral' -count=1
    go test ./internal/service -run 'TestRegisterWithVerification' -count=1
    git diff --check

Commit:

    git add backend/internal/handler/auth_handler.go backend/internal/handler/auth_handler_test.go backend/internal/service/auth_service_register_test.go
    git commit -m "feat: enforce locked referrals on registration"

## Task 4: Apply the lock to every OAuth registration path

**Files:**

- Modify: backend/internal/handler/auth_email_oauth.go
- Modify: backend/internal/handler/auth_linuxdo_oauth.go
- Modify: backend/internal/handler/auth_wechat_oauth.go
- Modify: backend/internal/handler/auth_oidc_oauth.go
- Modify: backend/internal/handler/auth_dingtalk_oauth.go
- Modify: backend/internal/handler/auth_oauth_pending_flow.go
- Modify matching auth_*_oauth_test.go and auth_oauth_pending_flow_test.go files.

### Step 1: Add failing provider-matrix tests

For GitHub, Google, LinuxDO, WeChat, OIDC, and DingTalk prove:

- start with a valid lock and conflicting query/body code stores the locked code in server-controlled OAuth state;
- start without a lock keeps current manual behavior;
- completion payload cannot override a locked code;
- successful account creation clears the lock;
- failed/pending completion keeps it for retry;
- successful existing-account OAuth login clears it without binding;
- callbacks never expose the raw locked code.

Run:

    cd backend
    go test ./internal/handler -run 'Test.*OAuth.*AffiliateReferralLock|TestPendingOAuth.*AffiliateReferralLock' -count=1

Expected: FAIL on conflict and cleanup cases.

### Step 2: Resolve once at OAuth start

At each provider start, pass current query/body affiliate code through affiliateCodeForRequest before copying it into signed state or server-controlled pending claims. Do not put raw codes in redirect URLs. Preserve existing provider state signing and CSRF behavior.

### Step 3: Prevent completion-time override

In email OAuth completion and shared pending account creation:

- prefer a live valid lock;
- otherwise prefer code captured in server-controlled pending state;
- use request aff_code only when neither exists;
- clear the lock only after successful login/account creation;
- keep bind-current-user flows referral-neutral.

### Step 4: Verify and commit

Run:

    cd backend
    go test ./internal/handler -run 'Test.*OAuth|TestPendingOAuth' -count=1
    git diff --check

Commit all six production files and matching tests with:

    git commit -m "feat: enforce locked referrals across oauth"

## Task 5: Add frontend resolver/status APIs and lock state

**Files:**

- Modify: frontend/src/api/auth.ts
- Modify: frontend/src/api/__tests__/auth.spec.ts
- Modify: frontend/src/utils/oauthAffiliate.ts
- Modify: frontend/src/utils/__tests__/oauthAffiliate.spec.ts

### Step 1: Write failing API/state tests

Cover:

- resolveAffiliateReferral posts only normalized aff_code;
- getAffiliateReferralStatus returns only locked;
- successful server lock clears legacy raw codes from local/session storage;
- invalid resolution stores nothing;
- 404/405 is distinguishable for blue/green legacy fallback;
- direct/manual storage behavior remains unchanged when no server lock exists.

Run:

    cd frontend
    npx vitest run src/api/__tests__/auth.spec.ts src/utils/__tests__/oauthAffiliate.spec.ts

Expected: FAIL because the API and state helpers do not exist.

### Step 2: Implement the thin frontend contract

Add typed API functions returning only:

- AffiliateReferralLockStatus: locked boolean;
- AffiliateReferralResolveResult: valid and locked booleans.

Use the existing API client credential behavior. Do not create a client-side lock token. Clear legacy raw storage only after server confirmation.

### Step 3: Verify and commit

Run:

    cd frontend
    npx vitest run src/api/__tests__/auth.spec.ts src/utils/__tests__/oauthAffiliate.spec.ts
    npx vue-tsc --noEmit
    git diff --check

Commit:

    git commit -m "feat: add affiliate referral lock client"

## Task 6: Hide the editable field only for a validated lock

**Files:**

- Modify: frontend/src/views/auth/RegisterView.vue
- Modify: frontend/src/views/auth/__tests__/RegisterView.spec.ts
- Modify: frontend/src/i18n/locales/zh/common.ts
- Modify: frontend/src/i18n/locales/en/common.ts

### Step 1: Write failing view tests

Cover:

- valid aff/aff_code resolves, hides the input, and never stores/submits the raw code;
- last valid link causes a new resolve request and replaces attribution server-side;
- invalid link shows localized error and leaves manual input available when no prior lock exists;
- invalid link does not replace an existing valid lock;
- no link queries status so a prior browser lock stays hidden;
- direct registration with no lock shows and submits the manual field;
- resolver 404/405 falls back to current local-storage/manual behavior;
- refresh stays hidden while locked;
- loading does not flash an editable field;
- hidden input is absent from the accessibility tree and mobile layout remains usable.

Run:

    cd frontend
    npx vitest run src/views/auth/__tests__/RegisterView.spec.ts

Expected: FAIL because the field is currently rendered whenever affiliate is enabled.

### Step 2: Implement registration-page orchestration

In RegisterView.vue:

1. Add locked, resolving, and localized-error state.
2. On mount/query change, resolve aff or aff_code; otherwise fetch status.
3. Hide the field only after locked true.
4. Never copy a locked raw code into formData.aff_code.
5. Preserve current editable behavior when unlocked.
6. Render stable helper/loading space while resolving.
7. On 404/405, run legacy syncAffiliateReferralCode and keep the field editable.

Add Chinese and English messages without exposing code or inviter identity.

### Step 3: Verify and commit

Run:

    cd frontend
    npx vitest run src/views/auth/__tests__/RegisterView.spec.ts src/utils/__tests__/oauthAffiliate.spec.ts
    npx eslint src/views/auth/RegisterView.vue src/views/auth/__tests__/RegisterView.spec.ts src/api/auth.ts src/utils/oauthAffiliate.ts
    npx vue-tsc --noEmit
    git diff --check

Commit:

    git commit -m "feat: hide locked affiliate referral input"

## Task 7: Remove locked raw codes from email verification and OAuth UI payloads

**Files:**

- Modify: frontend/src/views/auth/EmailVerifyView.vue
- Modify: frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts
- Modify: frontend/src/components/auth/EmailOAuthButtons.vue
- Modify: frontend/src/components/auth/WechatOAuthSection.vue
- Modify: frontend/src/components/auth/OAuthLoginSections.vue
- Modify matching component tests.

### Step 1: Write failing handoff tests

Prove:

- email verification succeeds with empty client aff_code while the cookie supplies attribution;
- OAuth starts do not need raw code while locked;
- direct/manual registration still forwards the manual code;
- legacy stored code is used only in old-backend fallback mode;
- no component renders or logs the locked code.

Run:

    cd frontend
    npx vitest run src/views/auth/__tests__/EmailVerifyView.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/components/auth/__tests__/WechatOAuthSection.spec.ts src/components/auth/__tests__/OAuthLoginSections.spec.ts

Expected: FAIL where components still source raw code unconditionally.

### Step 2: Make cookie-backed attribution the default

Pass boolean lock state, not a code, through registration components. Suppress raw affiliate fields when locked. Retain raw payloads only for unlocked/manual or explicit old-backend fallback mode.

### Step 3: Verify and commit

Run the focused suites, vue-tsc, eslint on modified files, and git diff --check.

Commit:

    git commit -m "fix: keep locked referrals out of client payloads"

## Task 8: Full verification and rollout readiness

### Step 1: Focused backend verification

    cd backend
    go test ./internal/service -run 'TestAffiliate|TestRegisterWithVerification|Test.*OAuth' -count=1
    go test ./internal/handler -run 'TestAffiliateReferral|TestRegister|TestLogin|Test.*OAuth|TestPendingOAuth' -count=1
    go test ./internal/server -run 'Test.*AuthRoutes' -count=1

### Step 2: Focused frontend and build verification

    cd frontend
    npx vitest run src/api/__tests__/auth.spec.ts src/utils/__tests__/oauthAffiliate.spec.ts src/views/auth/__tests__/RegisterView.spec.ts src/views/auth/__tests__/EmailVerifyView.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/components/auth/__tests__/WechatOAuthSection.spec.ts src/components/auth/__tests__/OAuthLoginSections.spec.ts
    npm run lint:check
    npx vue-tsc --noEmit
    npm run build

### Step 3: Verify the compatibility matrix

1. New frontend + new backend: valid link hides the field and binds the expected inviter.
2. New frontend + old backend: resolver 404/405 leaves editable legacy flow working.
3. Old frontend + new backend: no lock accepts old aff_code; valid lock overrides it.
4. Valid A then valid B: B binds.
5. Valid A then invalid link: A remains authoritative.
6. Cleared/tampered cookie: manual field is available; server never trusts tampered content.
7. Every OAuth provider registers with locked attribution.
8. Existing-user OAuth login never binds or changes an inviter.

### Step 4: Verify repository cleanliness

    git status --short
    git diff --check
    git log --oneline --decorate -12

Confirm the root untracked package files remain untouched and no secret, cookie value, or raw affiliate code appears in logs.

### Step 5: Request review before deployment

Use requesting-code-review against the approved design and this plan. Resolve all Critical/Important findings, rerun verification, then prepare dev for push and the existing blue/green deployment procedure.
