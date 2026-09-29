# NovelAI Native Image Adapter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a native NovelAI image-generation endpoint so clients such as 智绘姬 can use Passion API keys while preserving existing routing, failover, permissions, billing, and OpenAI image APIs.

**Architecture:** Add a protocol-aware native image adapter behind `/ai/generate-image` and `/v1/ai/generate-image`. The adapter reuses the existing image scheduling lifecycle and selects either the existing OpenAI forwarder or a new NovelAI native forwarder based on `credentials.image_protocol` (default `openai`). Native requests retain the complete `parameters` object and unknown fields; successful ZIP/PNG responses are returned in NovelAI-compatible form.

**Tech Stack:** Go, Gin, existing gateway services and middleware, Go `encoding/json`/`archive/zip`, Vue 3 + TypeScript admin account forms, testify.

---

### Task 1: Define native request/response contracts and parser

**Files:**
- Create: `backend/internal/service/novelai_images.go`
- Test: `backend/internal/service/novelai_images_test.go`

**Step 1: Write the failing tests**

Add table-driven tests for:
- valid `action=generate` requests preserving `input`, `model`, full `parameters`, and unknown top-level fields;
- missing `input`/`model`, unsupported action, malformed JSON, and invalid `parameters` type;
- defaulting model only when the route/group supplies one, never silently switching to a text model;
- extracting `n_samples` and dimensions for usage accounting without dropping the original body.

**Step 2: Run the focused tests to verify they fail**

Run:
```bash
go test ./backend/internal/service -run 'TestNovelAIRequest' -count=1
```
Expected: FAIL because the parser and request type do not exist.

**Step 3: Implement the minimal parser**

Create a typed request containing the original JSON, normalized model/input/action, parameter metadata, and a redacted logging summary. Validate only the native image contract and keep unknown fields in `json.RawMessage` so newer NovelAI parameters pass through unchanged.

**Step 4: Run the focused tests**

Run the same command and expect PASS.

**Step 5: Commit**

```bash
git add backend/internal/service/novelai_images.go backend/internal/service/novelai_images_test.go
git commit -m "feat: parse native NovelAI image requests"
```

### Task 2: Add protocol-aware account forwarding

**Files:**
- Modify: `backend/internal/service/account.go`
- Modify: `backend/internal/service/openai_images.go`
- Create: `backend/internal/service/novelai_images_forward.go`
- Test: `backend/internal/service/novelai_images_forward_test.go`

**Step 1: Write failing forwarding tests**

Cover:
- `image_protocol` absent/`openai` keeps the current OpenAI URL/body;
- `image_protocol=novelai` builds `<base>/ai/generate-image`, uses the upstream account bearer key, preserves the original native body, and does not forward the customer API key;
- base URLs ending in `/v1`, `/ai`, or `/ai/generate-image` normalize without duplicate path segments;
- upstream ZIP, PNG, JSON success, and 4xx/5xx responses are captured without leaking credentials.

**Step 2: Run focused tests and verify RED**

```bash
go test ./backend/internal/service -run 'TestNovelAIForward|TestImageProtocol' -count=1
```
Expected: FAIL for missing protocol helpers/forwarder.

**Step 3: Implement protocol helpers and native forwarder**

Add a defaulted `ImageProtocol` accessor on `Account`. Implement URL normalization, authentication header construction, request body forwarding, response classification, and bounded response reads using the same HTTP profile, timeout, proxy, header override, and upstream error sanitization conventions as OpenAI images.

**Step 4: Run focused tests and then existing image tests**

```bash
go test ./backend/internal/service -run 'TestNovelAIForward|TestImageProtocol' -count=1
go test ./backend/internal/service -run 'TestOpenAIImages' -count=1
```
Expected: PASS with no existing OpenAI regression.

**Step 5: Commit**

```bash
git add backend/internal/service/account.go backend/internal/service/openai_images.go backend/internal/service/novelai_images_forward.go backend/internal/service/novelai_images_forward_test.go
git commit -m "feat: add NovelAI native upstream forwarding"
```

### Task 3: Reuse image scheduling and usage lifecycle for native requests

**Files:**
- Modify: `backend/internal/handler/openai_images.go`
- Create: `backend/internal/handler/novelai_images.go`
- Test: `backend/internal/handler/novelai_images_test.go`

**Step 1: Write failing handler tests**

Use existing test fixtures to verify:
- API-key authentication and group `AllowImageGeneration` are enforced;
- `nai-diffusion-*` models are accepted by image routing and remain bound to the requested model during account switches;
- a failed NovelAI account is excluded and the next same-model account is attempted;
- successful usage records contain the native request hash, client model, upstream model, image count, and response size;
- upstream errors return the existing structured gateway error without writing a second response.

**Step 2: Run focused tests to verify RED**

```bash
go test ./backend/internal/handler -run 'TestNovelAIImages' -count=1
```
Expected: FAIL because the route and native handler are not registered.

**Step 3: Implement a shared image dispatch boundary**

Extract only the account-selection, slot-acquisition, failover, scheduling-result, and usage-recording portion of `OpenAIGatewayHandler.Images` into a private helper that accepts a protocol-specific forward callback. Keep existing OpenAI behavior byte-for-byte where possible. Add `NovelAIImages` to parse the native request, set the native endpoint context, invoke the shared lifecycle with the native forwarder, and return the normalized native response.

Extend image model predicates and allowlist checks to recognize `nai-diffusion-` models without making them available to text endpoints.

**Step 4: Run handler and route tests**

```bash
go test ./backend/internal/handler -run 'TestNovelAIImages|TestGatewayRoutesOpenAIImagesPathsAreRegistered' -count=1
go test ./backend/internal/handler ./backend/internal/service -count=1
```
Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/openai_images.go backend/internal/handler/novelai_images.go backend/internal/handler/novelai_images_test.go backend/internal/service
git commit -m "feat: route native NovelAI images through shared scheduling"
```

### Task 4: Register public routes and response compatibility

**Files:**
- Modify: `backend/internal/server/routes/gateway.go`
- Modify: `backend/internal/server/routes/gateway_test.go`
- Modify: `backend/internal/handler/endpoint.go`
- Test: `backend/internal/handler/endpoint_test.go`

**Step 1: Write failing route tests**

Assert `POST /ai/generate-image` and `POST /v1/ai/generate-image` reach the native handler, are included in endpoint normalization/usage classification, and do not change `/v1/images/generations` behavior.

**Step 2: Run tests to verify RED**

```bash
go test ./backend/internal/server/routes ./backend/internal/handler -run 'TestGatewayRoutes|TestEndpoint' -count=1
```
Expected: FAIL because the new paths are not registered.

**Step 3: Register and classify the routes**

Add both aliases using the same body limit and API-key middleware as image generation. Add inbound endpoint normalization and native response content-type handling. Return the upstream native body for ZIP/PNG and structured JSON for errors.

**Step 4: Run route tests and the complete backend suite**

```bash
go test ./backend/internal/server/routes ./backend/internal/handler -run 'TestGatewayRoutes|TestEndpoint' -count=1
go test ./backend/... -count=1
```
Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/server/routes/gateway.go backend/internal/server/routes/gateway_test.go backend/internal/handler/endpoint.go backend/internal/handler/endpoint_test.go
git commit -m "feat: expose NovelAI native image routes"
```

### Task 5: Expose account protocol configuration in the admin UI

**Files:**
- Modify: `frontend/src/components/account/CreateAccountModal.vue`
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/components/account/BulkEditAccountModal.vue` (only if the shared account editor supports protocol fields)
- Test: existing frontend account component tests or add `frontend/src/components/account/__tests__/novelaiProtocol.test.ts`

**Step 1: Write failing UI/state tests**

Verify API-key image-capable accounts can select `OpenAI 兼容` or `NovelAI 原生`, the value is stored as `credentials.image_protocol`, and existing accounts with no value render as OpenAI.

**Step 2: Run the focused frontend test to verify RED**

```bash
cd frontend npm run test -- --run novelaiProtocol
```
Expected: FAIL because the selector and serializer are absent.

**Step 3: Implement the selector and serializer**

Add the protocol selector near the API base URL/image capability controls, gate it to API-key accounts that can serve image models, persist the value in create/edit/bulk-edit flows, and explain that NovelAI clients should use the native endpoint. Do not display or expose upstream secrets.

**Step 4: Run frontend checks**

```bash
cd frontend
npm run test -- --run novelaiProtocol
npm run type-check
npm run build
```
Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/account/CreateAccountModal.vue frontend/src/components/account/EditAccountModal.vue frontend/src/components/account/BulkEditAccountModal.vue frontend/src/components/account/__tests__
git commit -m "feat: configure NovelAI account protocol"
```

### Task 6: End-to-end regression and documentation

**Files:**
- Modify: `docs/plans/2026-09-29-novelai-native-adapter-design.md` (implementation notes only)
- Create: `docs/api/novelai-compatible.md` if the repository’s API documentation convention supports a dedicated page
- Test: existing backend and frontend suites

**Step 1: Add contract examples**

Document the client configuration: use a Passion API key, base URL `https://api.passionapi.com`, native path handled by the client, and lowercase model IDs `nai-diffusion-4-5-full` / `nai-diffusion-5-full`. Document that upstream keys are never given to customers.

**Step 2: Run all verification**

```bash
go test ./backend/... -count=1
cd frontend && npm run type-check && npm run build
```

**Step 3: Inspect the diff and commit documentation**

```bash
git diff origin/dev...HEAD --stat
git status --short

git add docs/api docs/plans/2026-09-29-novelai-native-adapter-design.md
git commit -m "docs: document NovelAI client configuration"
```

**Step 4: Final verification**

Run the focused native tests and full suites again after the final commit. Report the exact commit, tests, and any upstream integration limitation.

