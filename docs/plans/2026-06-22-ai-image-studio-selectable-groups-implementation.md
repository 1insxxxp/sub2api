# AI Image Studio Selectable Groups Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let users choose an image-enabled group and model in AI Image Studio, then generate or edit images through the selected group's account pool and image pricing while billing the user's USD wallet balance.

**Architecture:** Add a backend options API that resolves image-capable groups, per-group models, and price previews for the current user. Update Image Studio generation/edit execution to accept a selected `group_id`, validate user access and model availability, and schedule accounts from that group instead of implicitly using the user's first active API key. Update the Vue image page to render group/model selectors and submit the selected group with requests.

**Tech Stack:** Go, Gin, Ent repositories, PostgreSQL, existing Sub2API group/account/channel services, existing OpenAI image gateway, Vue 3, TypeScript, Pinia, vue-i18n, Vitest.

---

**Command note:** All Go commands are run from `D:\xp\sub2api\backend` unless noted. All frontend and git commands are run from `D:\xp\sub2api`.

## Task 1: Add Image Studio Options Types And Resolver Tests

**Files:**
- Modify: `backend/internal/service/image_studio_types.go`
- Modify: `backend/internal/service/image_studio_service.go`
- Create: `backend/internal/service/image_studio_options_test.go`

**Step 1: Write failing tests**

Add tests for:

- Active image-enabled group with schedulable account appears.
- Group with `allow_image_generation=false` is excluded.
- Group with no active schedulable account is excluded.
- Group models are resolved and deduplicated.
- Price preview includes supported aspect ratios.

Run:

```powershell
go test ./internal/service -run TestImageStudioOptions -v
```

Expected: FAIL because options resolver does not exist.

**Step 2: Add DTOs**

Add service DTOs:

```go
type ImageStudioOptions struct {
    Enabled        bool                     `json:"enabled"`
    DefaultGroupID *int64                   `json:"default_group_id,omitempty"`
    DefaultModel   string                   `json:"default_model"`
    Groups         []ImageStudioGroupOption `json:"groups"`
}

type ImageStudioGroupOption struct {
    ID          int64                         `json:"id"`
    Name        string                        `json:"name"`
    Description string                        `json:"description,omitempty"`
    Platform    string                        `json:"platform"`
    Models      []ImageStudioModelOption      `json:"models"`
    Prices      []ImageStudioPricePreviewItem `json:"prices"`
}

type ImageStudioModelOption struct {
    Model        string   `json:"model"`
    Label        string   `json:"label"`
    MappedModel  string   `json:"mapped_model,omitempty"`
    Capabilities []string `json:"capabilities"`
}

type ImageStudioPricePreviewItem struct {
    Ratio         string  `json:"ratio"`
    Size          string  `json:"size"`
    BillingTier   string  `json:"billing_tier"`
    EstimatedCost float64 `json:"estimated_cost"`
}
```

**Step 3: Add resolver dependencies**

Extend `ImageStudioService` with repository/service interfaces needed to list candidate groups and schedulable accounts. Prefer existing repositories already wired into services. If no single repository method exists, add a narrow method such as:

```go
ListImageStudioAvailableGroups(ctx context.Context, userID int64) ([]ImageStudioGroupCandidate, error)
```

Keep this resolver read-only.

**Step 4: Implement minimal resolver**

Add:

```go
func (s *ImageStudioService) GetOptions(ctx context.Context, userID int64) (*ImageStudioOptions, error)
```

Use existing Image Studio global settings for enabled/default model. Return empty `groups` when disabled or no groups are available.

**Step 5: Run tests**

Run:

```powershell
go test ./internal/service -run TestImageStudioOptions -v
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add backend/internal/service/image_studio_types.go backend/internal/service/image_studio_service.go backend/internal/service/image_studio_options_test.go
git commit -m "feat: resolve image studio group options"
```

## Task 2: Add User Options API Handler

**Files:**
- Modify: `backend/internal/handler/image_studio_handler.go`
- Modify: `backend/internal/handler/image_studio_handler_test.go`
- Modify: `backend/internal/server/routes/user.go`

**Step 1: Write failing handler test**

Add a test for:

- `GET /api/v1/user/images/options` uses authenticated user ID.
- Response contains `enabled`, `groups`, and default fields.

Run:

```powershell
go test ./internal/handler -run TestImageStudioHandlerGetOptions -v
```

Expected: FAIL because handler method and route do not exist.

**Step 2: Add service interface method**

Extend `ImageStudioService` interface in handler:

```go
GetOptions(ctx context.Context, userID int64) (*service.ImageStudioOptions, error)
```

**Step 3: Add handler method**

Add:

```go
func (h *ImageStudioHandler) GetOptions(c *gin.Context)
```

Read user ID using the same helper pattern as generate/list/delete.

**Step 4: Add route**

Under `/api/v1/user/images` add:

```go
images.GET("/options", h.ImageStudio.GetOptions)
```

Place it before `images.GET("")`.

**Step 5: Run tests**

Run:

```powershell
go test ./internal/handler -run TestImageStudioHandlerGetOptions -v
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add backend/internal/handler/image_studio_handler.go backend/internal/handler/image_studio_handler_test.go backend/internal/server/routes/user.go
git commit -m "feat: expose image studio options api"
```

## Task 3: Require Selected Group In Generate And Edit Inputs

**Files:**
- Modify: `backend/internal/service/image_studio_types.go`
- Modify: `backend/internal/service/image_studio_service.go`
- Modify: `backend/internal/service/image_studio_service_test.go`
- Modify: `backend/internal/handler/image_studio_handler.go`
- Modify: `backend/internal/handler/image_studio_handler_test.go`

**Step 1: Write failing service tests**

Cover:

- Generate with valid `group_id` stores that group in gateway input.
- Generate without `group_id` auto-selects when exactly one group is available.
- Generate without `group_id` fails when multiple groups are available.
- Generate rejects unavailable group.
- Generate rejects model not allowed by selected group.
- Edit follows the same validation.

Run:

```powershell
go test ./internal/service -run 'TestImageStudio(Service|SelectableGroup)' -v
```

Expected: FAIL.

**Step 2: Extend inputs**

Add to `ImageStudioGenerateInput` and `ImageStudioEditInput`:

```go
GroupID *int64 `json:"group_id,omitempty"`
```

For multipart edit handler, parse `group_id` from form field.

**Step 3: Add group/model selection helper**

Add a helper that resolves:

- selected group
- selected model
- selected price preview

Return typed errors:

- `IMAGE_STUDIO_GROUP_REQUIRED`
- `IMAGE_STUDIO_GROUP_NOT_AVAILABLE`
- `IMAGE_STUDIO_MODEL_NOT_AVAILABLE`

**Step 4: Update preparation**

`prepareGenerateInput` and `prepareEditInput` should call the helper before building gateway payload.

**Step 5: Run tests**

Run:

```powershell
go test ./internal/service -run 'TestImageStudio(Service|SelectableGroup)' -v
go test ./internal/handler -run TestImageStudioHandler -v
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add backend/internal/service/image_studio_types.go backend/internal/service/image_studio_service.go backend/internal/service/image_studio_service_test.go backend/internal/handler/image_studio_handler.go backend/internal/handler/image_studio_handler_test.go
git commit -m "feat: validate selected image studio group"
```

## Task 4: Schedule Images From Selected Group Instead Of First User API Key

**Files:**
- Modify: `backend/internal/service/image_studio_gateway.go`
- Modify: `backend/internal/service/image_studio_gateway_test.go`
- Modify: `backend/internal/service/api_key_service.go`
- Modify: `backend/internal/service/image_studio_api_key_test.go`

**Step 1: Write failing gateway tests**

Cover:

- Gateway executor receives selected group ID.
- Account selection uses selected group ID.
- User with no API key can still generate if group is available.
- If a user's API key exists for the selected group, it can be used for attribution.
- Billing still records the real user and selected group.

Run:

```powershell
go test ./internal/service -run 'TestImageStudioGatewayExecutor|TestAPIKeyServiceGetDefaultImageStudioAPIKey' -v
```

Expected: FAIL for no-key selected group behavior.

**Step 2: Extend gateway input**

Add selected group context to gateway execution input:

```go
GroupID int64
Group   *Group
User    *User
```

**Step 3: Add attribution API key lookup**

Replace mandatory `GetDefaultImageStudioAPIKey(userID)` with a softer method:

```go
GetImageStudioAttributionAPIKey(ctx context.Context, userID int64, groupID int64) (*APIKey, error)
```

This should return:

- first active key bound to group, if found
- nil, nil when none exists
- real error only on repository failure

**Step 4: Build compatible usage context**

If no API key exists, create an in-memory APIKey-like object with:

- `UserID`
- `User`
- `GroupID`
- `Group`
- `Status=active`

Do not persist it and do not expose a key string.

**Step 5: Update billing eligibility**

Confirm `CheckBillingEligibility` works with synthetic context. If it requires a non-empty key string, adjust it to accept Image Studio internal context explicitly.

**Step 6: Run tests**

Run:

```powershell
go test ./internal/service -run 'TestImageStudioGatewayExecutor|TestAPIKeyServiceGetDefaultImageStudioAPIKey|TestOpenAIGatewayServiceRecordUsage_Image' -v
```

Expected: PASS.

**Step 7: Commit**

```powershell
git add backend/internal/service/image_studio_gateway.go backend/internal/service/image_studio_gateway_test.go backend/internal/service/api_key_service.go backend/internal/service/image_studio_api_key_test.go
git commit -m "feat: schedule image studio by selected group"
```

## Task 5: Update Frontend API Types

**Files:**
- Modify: `frontend/src/api/images.ts`
- Modify: `frontend/src/api/__tests__/images.spec.ts`

**Step 1: Write failing API tests**

Cover:

- `getOptions()` calls `/user/images/options`.
- `generate()` sends `group_id`.
- `edit()` appends `group_id` to multipart form.

Run:

```powershell
pnpm --dir frontend run test:run -- images.spec.ts
```

Expected: FAIL.

**Step 2: Add TypeScript types**

Add:

```ts
export interface ImageStudioOptions
export interface ImageStudioGroupOption
export interface ImageStudioModelOption
export interface ImageStudioPricePreviewItem
```

Extend generate/edit payloads with:

```ts
group_id?: number
```

**Step 3: Add `getOptions`**

Add to API module and exported object.

**Step 4: Run tests**

Run:

```powershell
pnpm --dir frontend run test:run -- images.spec.ts
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add frontend/src/api/images.ts frontend/src/api/__tests__/images.spec.ts
git commit -m "feat: add image studio group options api"
```

## Task 6: Add Group And Model Selectors To User Image Page

**Files:**
- Modify: `frontend/src/views/user/ImageStudioView.vue`
- Modify: `frontend/src/views/user/__tests__/ImageStudioView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: Write failing component tests**

Cover:

- Groups render in selector.
- First available group auto-selects.
- Model selector renders selected group's models.
- Changing group updates model options.
- Cost preview updates when group or ratio changes.
- Generate payload includes `group_id`.
- Edit payload includes `group_id`.
- No groups shows empty unavailable state.

Run:

```powershell
pnpm --dir frontend run test:run -- ImageStudioView.spec.ts
```

Expected: FAIL.

**Step 2: Load options**

On mount, load:

- config
- options
- history

Keep config for enabled/storage/upload limits. Use options for groups/models/prices.

**Step 3: Add selectors**

In the creation panel:

- Add group selector.
- Add model selector scoped to selected group.
- Keep aspect ratio selector.
- Show selected price preview next to billing tier.

Use existing theme controls and avoid nested card styling.

**Step 4: Add empty states**

Show:

- Studio disabled.
- No image group available.
- Group has no models.

**Step 5: Submit selected group**

Include `group_id` in both generate and edit payloads.

**Step 6: Add i18n**

Add Chinese and English keys for:

- group selector
- no group state
- model unavailable
- estimated cost
- selected route

**Step 7: Run tests**

Run:

```powershell
pnpm --dir frontend run test:run -- ImageStudioView.spec.ts
pnpm --dir frontend run typecheck
```

Expected: PASS.

**Step 8: Commit**

```powershell
git add frontend/src/views/user/ImageStudioView.vue frontend/src/views/user/__tests__/ImageStudioView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: let users select image group and model"
```

## Task 7: Add Admin Readiness Copy And Optional Group Summary

**Files:**
- Modify: `frontend/src/views/admin/settings/ImageStudioSettingsPanel.vue`
- Modify: `frontend/src/views/admin/settings/__tests__/ImageStudioSettingsPanel.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Step 1: Write failing component test**

Cover that the settings panel explains:

- Image groups are configured in group management.
- Only image-enabled groups with schedulable accounts appear to users.

Run:

```powershell
pnpm --dir frontend run test:run -- ImageStudioSettingsPanel.spec.ts
```

Expected: FAIL until copy is added.

**Step 2: Add copy**

Add a concise info section to the Image Studio settings panel. Do not add a full duplicate group manager in this task.

**Step 3: Run tests**

Run:

```powershell
pnpm --dir frontend run test:run -- ImageStudioSettingsPanel.spec.ts
```

Expected: PASS.

**Step 4: Commit**

```powershell
git add frontend/src/views/admin/settings/ImageStudioSettingsPanel.vue frontend/src/views/admin/settings/__tests__/ImageStudioSettingsPanel.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "docs: clarify image studio group setup in admin"
```

## Task 8: Full Verification

**Files:**
- No direct file edits.

**Step 1: Backend focused tests**

Run:

```powershell
go test ./internal/service -run 'TestImageStudio|TestAPIKeyServiceGetImageStudio|TestOpenAIGatewayServiceRecordUsage_Image' -v
go test ./internal/handler -run TestImageStudioHandler -v
```

Expected: PASS.

**Step 2: Frontend focused tests**

Run:

```powershell
pnpm --dir frontend run test:run -- images.spec.ts ImageStudioView.spec.ts ImageStudioSettingsPanel.spec.ts
pnpm --dir frontend run typecheck
```

Expected: PASS.

**Step 3: Build**

Run:

```powershell
pnpm --dir frontend run build
```

Expected: PASS with only existing Vite/Browserslist warnings.

**Step 4: Local Docker verification**

Run:

```powershell
docker compose -f deploy/docker-compose.dev.yml up -d --build sub2api
```

Verify:

- `http://127.0.0.1:18080/api/v1/user/images/options` returns groups after login.
- `/images` shows group selector.
- Switching groups changes models and price preview.
- Generate is blocked with a clear message when no image group is available.

**Step 5: Final commit**

```powershell
git status --short
git log --oneline -5
```

Expected: only intended changes are present.
