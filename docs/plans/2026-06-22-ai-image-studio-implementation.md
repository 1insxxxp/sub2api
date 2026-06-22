# AI Image Studio Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a user-facing AI image generation and editing workspace that uses the existing OpenAI-compatible image gateway, charges the user's USD wallet balance, and stores generated image files in Cloudflare R2 or local development storage.

**Architecture:** Add a thin user-facing image service around the existing image gateway and image billing logic. Persist image metadata in a new Ent table, store binary image files through an `ImageStorage` abstraction, and expose a polished Vue image workspace plus admin settings for model/storage policy.

**Tech Stack:** Go, Gin, Ent, PostgreSQL migrations, existing Sub2API services, Cloudflare R2 through S3-compatible API, Vue 3, TypeScript, Pinia, vue-i18n, Vitest.

---

**Command note:** All `go test ./...` commands in this plan are run from `D:\xp\sub2api\backend`. All `pnpm --dir frontend ...`, `make -C backend ...`, and `git ...` commands are run from `D:\xp\sub2api`.

## Task 1: Add Image Studio Domain Model And Migration

**Files:**
- Create: `backend/ent/schema/user_image.go`
- Create: `backend/migrations/155_user_images.sql`
- Modify: `backend/ent/schema/user.go`
- Modify: `backend/migrations/migrations.go`
- Test: `backend/ent/schema/user_image_schema_test.go`

**Step 1: Write the schema test**

Add a focused Ent schema test that verifies:

- `user_images` table name.
- `user_id`, `mode`, `model`, `prompt`, `aspect_ratio`, `size`, `image_url`, `storage_driver`, `storage_object_key`, `mime_type`, `bytes`, `cost`, `usage_log_id`, `source_image_count`, `expires_at`, `deleted_at`, `created_at`, and `updated_at` fields exist.
- `user_id, created_at` index exists.
- `deleted_at` index exists.

Run:

```powershell
go test ./ent/schema -run TestUserImageSchema -v
```

Expected: FAIL because `UserImage` does not exist.

**Step 2: Add the Ent schema**

Create `backend/ent/schema/user_image.go` with fields from the design. Use the existing schema style from `backend/ent/schema/user_checkin.go`:

- `field.Int64("user_id")`
- `field.String("mode").MaxLen(32).NotEmpty()`
- `field.String("model").MaxLen(128).NotEmpty()`
- `field.Text("prompt").Optional().Nillable()`
- `field.String("aspect_ratio").MaxLen(16).NotEmpty()`
- `field.String("size").MaxLen(32).NotEmpty()`
- `field.String("image_url").MaxLen(2048).NotEmpty()`
- `field.String("storage_driver").MaxLen(32).Default("local")`
- `field.String("storage_object_key").MaxLen(1024).NotEmpty()`
- `field.String("mime_type").MaxLen(128).Default("image/png")`
- `field.Int64("bytes").Default(0)`
- `field.Float("cost").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0)`
- `field.Int64("usage_log_id").Optional().Nillable()`
- `field.Int("source_image_count").Default(0)`
- `field.Time("expires_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"})`
- `field.Time("deleted_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"})`
- `field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"})`
- `field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"})`

Add an edge from `UserImage` to `User`, and add a `user_images` edge on `User`.

**Step 3: Add SQL migration**

Create `backend/migrations/155_user_images.sql`:

- Create `user_images`.
- Add foreign key to `users(id)` with cascade or restrict according to existing user-owned tables.
- Add indexes:
  - `idx_user_images_user_created_at`
  - `idx_user_images_user_deleted_at`
  - `idx_user_images_expires_at`
  - `idx_user_images_storage_object_key`

Register it in `backend/migrations/migrations.go`.

**Step 4: Generate Ent code**

Run:

```powershell
make -C backend generate
```

Expected: generated Ent code includes `UserImage`.

**Step 5: Run tests**

Run:

```powershell
go test ./ent/schema -run TestUserImageSchema -v
go test ./internal/service -run TestImageBilling -v
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add backend/ent/schema/user_image.go backend/ent/schema/user.go backend/ent backend/migrations/155_user_images.sql backend/migrations/migrations.go backend/ent/schema/user_image_schema_test.go
git commit -m "feat: add user image records schema"
```

## Task 2: Add Aspect Ratio, Price Tier, And Request Validation Helpers

**Files:**
- Create: `backend/internal/service/image_studio_types.go`
- Create: `backend/internal/service/image_studio_test.go`

**Step 1: Write failing tests**

Cover:

- `1:1 -> 1024x1024 -> 1K`
- `4:3 -> 1024x768 -> 1K`
- `3:4 -> 768x1024 -> 1K`
- `16:9 -> 1536x864 -> 2K`
- `9:16 -> 864x1536 -> 2K`
- Unknown ratio returns validation error.
- Empty model returns validation error.

Run:

```powershell
go test ./internal/service -run TestImageStudioAspectRatio -v
```

Expected: FAIL because helpers do not exist.

**Step 2: Implement helpers**

Create:

- `ImageStudioAspectRatio`
- `ImageStudioConfig`
- `ImageStudioGenerateInput`
- `ImageStudioEditInput`
- `ImageStudioImageRecord`
- `ImageStudioPricePreview`

Add:

- `SupportedImageStudioAspectRatios()`
- `ResolveImageStudioAspectRatio(ratio string) (size string, billingTier string, err error)`
- `ValidateImageStudioModel(model string, allowed []string) error`
- `NormalizeImageStudioPrompt(prompt string) (string, error)`

Reuse existing constants from `backend/internal/service/image_billing_size.go`.

**Step 3: Run tests**

Run:

```powershell
go test ./internal/service -run TestImageStudioAspectRatio -v
```

Expected: PASS.

**Step 4: Commit**

```powershell
git add backend/internal/service/image_studio_types.go backend/internal/service/image_studio_test.go
git commit -m "feat: add image studio request helpers"
```

## Task 3: Add Storage Abstraction With Local And R2 Drivers

**Files:**
- Create: `backend/internal/service/image_storage.go`
- Create: `backend/internal/service/image_storage_local.go`
- Create: `backend/internal/service/image_storage_r2.go`
- Create: `backend/internal/service/image_storage_test.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

**Step 1: Write failing storage tests**

Cover:

- Local driver writes object bytes under the configured directory.
- Local driver returns `publicBaseURL/objectKey`.
- Local delete removes the file.
- Object key generation includes user id, date partitions, and a UUID-like random suffix.
- Generated keys never allow path traversal.

Run:

```powershell
go test ./internal/service -run TestImageStorage -v
```

Expected: FAIL because storage types do not exist.

**Step 2: Implement storage interface**

Add:

```go
type ImageStorage interface {
    Put(ctx context.Context, objectKey string, contentType string, data []byte) (string, error)
    Delete(ctx context.Context, objectKey string) error
}
```

Add local storage implementation for development.

**Step 3: Add R2 implementation**

Use the AWS SDK v2 S3 client with:

- Endpoint: `https://<account_id>.r2.cloudflarestorage.com`
- Region: `auto`
- Static credentials.
- Bucket from config.

Keep R2 driver behind config. Tests should use a fake S3 client or keep R2 constructor tests limited to config validation so unit tests do not require network.

**Step 4: Run tests**

Run:

```powershell
go test ./internal/service -run TestImageStorage -v
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add backend/internal/service/image_storage*.go backend/internal/service/image_storage_test.go backend/go.mod backend/go.sum
git commit -m "feat: add image storage adapters"
```

## Task 4: Add Image Studio Settings

**Files:**
- Modify: `backend/internal/service/setting_service.go`
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `frontend/src/api/admin/settings.ts`
- Test: `backend/internal/service/setting_service_test.go`
- Test: `backend/internal/handler/admin/setting_handler_test.go`

**Step 1: Write failing service tests**

Cover defaults:

- `enabled=false`
- `allowed_models=["gpt-image-1"]` or empty list if the project prefers no default model.
- `default_model`
- `storage_driver=local`
- `retention_days=30`
- `max_images_per_user=100`
- `max_reference_image_mb=20`

Cover save/load round trip.

Run:

```powershell
go test ./internal/service -run TestImageStudioSettings -v
```

Expected: FAIL.

**Step 2: Implement setting structs and keys**

Add a JSON setting similar to Web Search config:

- `SettingKeyImageStudioConfig`
- `type ImageStudioSettings struct`
- `GetImageStudioConfig(ctx)`
- `SaveImageStudioConfig(ctx, *ImageStudioSettings)`

Include storage status fields in admin response without returning secrets.

**Step 3: Add admin handlers and routes**

Add:

- `GET /api/v1/admin/settings/image-studio`
- `PUT /api/v1/admin/settings/image-studio`
- `POST /api/v1/admin/settings/image-studio/storage/test`

The storage test should validate local path or R2 credentials by a lightweight put/delete probe when configured.

**Step 4: Add frontend admin API types**

Add TypeScript types and functions in `frontend/src/api/admin/settings.ts`.

**Step 5: Run tests**

Run:

```powershell
go test ./internal/service -run TestImageStudioSettings -v
go test ./internal/handler/admin -run TestImageStudioSettings -v
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add backend/internal/service/setting_service.go backend/internal/handler/admin/setting_handler.go backend/internal/server/routes/admin.go frontend/src/api/admin/settings.ts backend/internal/service/*test.go backend/internal/handler/admin/*test.go
git commit -m "feat: add image studio settings"
```

## Task 5: Add Image Studio Service And Repository Flow

**Files:**
- Create: `backend/internal/service/image_studio_service.go`
- Create: `backend/internal/repository/user_image_repo.go`
- Create: `backend/internal/service/image_studio_service_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/internal/repository/wire.go`

**Step 1: Write failing service tests**

Cover:

- Config disabled returns a typed forbidden error.
- Group without image generation permission returns forbidden.
- Insufficient balance returns payment/validation error.
- Successful generation stores metadata after storage succeeds.
- Storage failure does not leave a completed image record.
- List only returns current user's non-deleted records.
- Delete checks ownership and soft deletes.

Run:

```powershell
go test ./internal/service -run TestImageStudioService -v
```

Expected: FAIL.

**Step 2: Implement repository**

Add methods:

- `Create(ctx, input)`
- `ListByUser(ctx, userID, page, pageSize)`
- `GetByID(ctx, id)`
- `SoftDelete(ctx, id, userID)`
- `CountSavedByUser(ctx, userID)`
- `DeleteOldestOverLimit(ctx, userID, limit)`
- `ListExpired(ctx, now, limit)`

**Step 3: Implement service shell**

Implement config, list, and delete first. Keep generate/edit failing until Task 6 if direct gateway reuse requires a handler-level adapter.

**Step 4: Wire dependencies**

Add providers to existing Wire provider sets and regenerate if needed:

```powershell
make -C backend generate
```

**Step 5: Run tests**

Run:

```powershell
go test ./internal/service -run TestImageStudioService -v
```

Expected: PASS for config/list/delete tests and any mocked generate tests.

**Step 6: Commit**

```powershell
git add backend/internal/service/image_studio_service.go backend/internal/repository/user_image_repo.go backend/internal/service/image_studio_service_test.go backend/internal/service/wire.go backend/internal/repository/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: add image studio service"
```

## Task 6: Reuse Existing Image Gateway For Generate And Edit

**Files:**
- Modify: `backend/internal/service/openai_images.go`
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/service/image_studio_service.go`
- Test: `backend/internal/service/image_studio_gateway_test.go`
- Test: `backend/internal/handler/openai_images_failover_test.go`

**Step 1: Write integration-style service tests**

Use an `httptest.Server` as the upstream image provider. Cover:

- Generate sends model, prompt, size, `n=1`, and `response_format=b64_json`.
- Edit sends multipart prompt and reference image.
- Returned base64 image is decoded, uploaded to storage, and recorded.
- Usage/billing is recorded once.
- Upstream error does not charge or store.

Run:

```powershell
go test ./internal/service -run TestImageStudioGateway -v
```

Expected: FAIL.

**Step 2: Extract reusable gateway execution if necessary**

The existing gateway currently writes directly to Gin responses. Add the smallest reusable service method that can execute an OpenAI image request and return parsed image data and usage metadata without duplicating routing, account selection, failover, moderation, or billing.

Keep existing `/v1/images/generations` and `/v1/images/edits` behavior unchanged.

**Step 3: Implement generate and edit**

In `ImageStudioService`:

- Resolve aspect ratio to size and tier.
- Build OpenAI-compatible request.
- Force a non-streaming response format the service can persist.
- Execute through the reusable gateway path.
- Decode returned image data.
- Upload to storage.
- Charge after storage succeeds or ensure transaction rollback/refund on storage failure.
- Create image record with cost and metadata.
- Return record DTO.

**Step 4: Run gateway regression tests**

Run:

```powershell
go test ./internal/service -run "TestImageStudioGateway|TestOpenAIImages" -v
go test ./internal/handler -run "TestOpenAIImages|TestGatewayRoutesOpenAIImages" -v
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add backend/internal/service/openai_images.go backend/internal/handler/openai_images.go backend/internal/service/image_studio_service.go backend/internal/service/image_studio_gateway_test.go backend/internal/handler/openai_images_failover_test.go
git commit -m "feat: connect image studio to image gateway"
```

## Task 7: Add User Image Studio HTTP API

**Files:**
- Create: `backend/internal/handler/image_studio_handler.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/user.go`
- Test: `backend/internal/handler/image_studio_handler_test.go`

**Step 1: Write handler tests**

Cover:

- `GET /api/v1/user/images/config`
- `POST /api/v1/user/images/generate`
- `POST /api/v1/user/images/edit`
- `GET /api/v1/user/images`
- `DELETE /api/v1/user/images/:id`
- Unauthenticated requests are rejected through existing middleware route tests if available.

Run:

```powershell
go test ./internal/handler -run TestImageStudioHandler -v
```

Expected: FAIL.

**Step 2: Implement handler**

Follow `backend/internal/handler/checkin_handler.go` style:

- Get auth subject from context.
- Bind JSON or multipart form.
- Call service.
- Return `response.Success`.
- Use `response.ErrorFrom` for service errors.

**Step 3: Register routes**

Under authenticated `/user` routes:

- `images.GET("/config", h.ImageStudio.GetConfig)`
- `images.POST("/generate", h.ImageStudio.Generate)`
- `images.POST("/edit", h.ImageStudio.Edit)`
- `images.GET("", h.ImageStudio.List)`
- `images.DELETE("/:id", h.ImageStudio.Delete)`

**Step 4: Regenerate Wire**

Run:

```powershell
make -C backend generate
```

**Step 5: Run tests**

Run:

```powershell
go test ./internal/handler -run TestImageStudioHandler -v
go test ./internal/server/routes -run Test -v
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add backend/internal/handler/image_studio_handler.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/user.go backend/internal/handler/image_studio_handler_test.go backend/cmd/server/wire_gen.go
git commit -m "feat: expose image studio user api"
```

## Task 8: Add Frontend API, Types, Route, And Sidebar Entry

**Files:**
- Create: `frontend/src/api/images.ts`
- Modify: `frontend/src/api/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/api/__tests__/images.spec.ts`

**Step 1: Write API tests**

Cover URL and payload building for:

- `getImageStudioConfig`
- `generateImage`
- `editImage`
- `listImages`
- `deleteImage`

Run:

```powershell
pnpm --dir frontend run test:run -- images.spec.ts
```

Expected: FAIL.

**Step 2: Implement API module**

Create TypeScript interfaces matching backend DTOs:

- `ImageStudioConfig`
- `ImageStudioAspectRatio`
- `ImageStudioImage`
- `ImageStudioGeneratePayload`
- `ImageStudioEditPayload`

Use `FormData` for edit uploads.

**Step 3: Add route and sidebar**

Add route:

- Path: `/images`
- Name: `ImageStudio`
- Component: `@/views/user/ImageStudioView.vue`
- Auth required.
- i18n title keys.

Add user sidebar entry with a theme-consistent icon.

**Step 4: Add i18n copy**

Add `imageStudio.*` keys in Chinese and English.

**Step 5: Run tests**

Run:

```powershell
pnpm --dir frontend run test:run -- images.spec.ts
pnpm --dir frontend run typecheck
```

Expected: PASS.

**Step 6: Commit**

```powershell
git add frontend/src/api/images.ts frontend/src/api/index.ts frontend/src/router/index.ts frontend/src/components/layout/AppSidebar.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/api/__tests__/images.spec.ts
git commit -m "feat: add image studio frontend route"
```

## Task 9: Build The User Image Studio View

**Files:**
- Create: `frontend/src/views/user/ImageStudioView.vue`
- Modify: `frontend/src/components/common/ImageUpload.vue` if paste/drop support cannot be composed outside it.
- Test: `frontend/src/views/user/__tests__/ImageStudioView.spec.ts`

**Step 1: Write view tests**

Cover:

- Shows disabled state when config disabled.
- Renders model selector and ratio selector from config.
- Shows estimated cost when model and ratio are selected.
- Generate button disabled without prompt.
- Successful generate appends image to gallery.
- Edit mode accepts a file.
- Delete removes image from list after confirmation.

Run:

```powershell
pnpm --dir frontend run test:run -- ImageStudioView.spec.ts
```

Expected: FAIL.

**Step 2: Implement layout**

Use current theme patterns:

- No card-inside-card nesting.
- Responsive two-column desktop layout and single-column mobile layout.
- Main workspace: prompt, mode toggle, model, ratio, upload.
- Right/result area: current result and recent images.
- Clear loading skeleton/progress state.

**Step 3: Implement interactions**

- Load config and first history page on mount.
- Generate uses JSON API.
- Edit uses multipart API.
- Reference upload supports file input, drag/drop, and paste.
- On success update gallery and optionally refresh balance from auth store/profile.
- Copy URL uses clipboard.
- Download uses anchor download.

**Step 4: Run tests**

Run:

```powershell
pnpm --dir frontend run test:run -- ImageStudioView.spec.ts
pnpm --dir frontend run typecheck
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add frontend/src/views/user/ImageStudioView.vue frontend/src/components/common/ImageUpload.vue frontend/src/views/user/__tests__/ImageStudioView.spec.ts
git commit -m "feat: build image studio workspace"
```

## Task 10: Add Admin UI For Image Studio Settings

**Files:**
- Modify: `frontend/src/views/admin/SettingsView.vue` or the current settings subcomponent that owns feature settings.
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/views/admin/__tests__/SettingsView.spec.ts` or a new focused settings component test.

**Step 1: Write UI tests**

Cover:

- Loads image studio settings.
- Toggles global enable.
- Edits allowed models and default model.
- Edits storage driver and R2 public base URL.
- Saves retention days, max images per user, and max reference image size.
- Test storage button shows success/failure.

Run:

```powershell
pnpm --dir frontend run test:run -- SettingsView.spec.ts
```

Expected: FAIL.

**Step 2: Implement settings panel**

Add a compact "AI Image Studio" settings section:

- Feature switch.
- Model list textarea or tag input.
- Default model select/input.
- Storage driver segmented control: local/R2.
- R2 public URL field and status badge.
- Retention and quota numeric inputs.
- Storage test button.

Do not expose secrets in public frontend state. Secret env vars should be configured on the server.

**Step 3: Run tests**

Run:

```powershell
pnpm --dir frontend run test:run -- SettingsView.spec.ts
pnpm --dir frontend run typecheck
```

Expected: PASS.

**Step 4: Commit**

```powershell
git add frontend/src/views/admin/SettingsView.vue frontend/src/api/admin/settings.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts frontend/src/views/admin/__tests__/SettingsView.spec.ts
git commit -m "feat: add image studio admin settings"
```

## Task 11: Add Cleanup Job Or Command

**Files:**
- Create: `backend/internal/service/image_studio_cleanup.go`
- Create: `backend/internal/service/image_studio_cleanup_test.go`
- Modify: scheduler/bootstrap file that starts recurring cleanup jobs, if one exists.

**Step 1: Write cleanup tests**

Cover:

- Expired image records are soft deleted.
- Storage delete is attempted for expired records.
- Per-user over-limit cleanup deletes oldest records.
- Storage delete failure is logged but does not stop DB cleanup of other records.

Run:

```powershell
go test ./internal/service -run TestImageStudioCleanup -v
```

Expected: FAIL.

**Step 2: Implement cleanup service**

Add a small service that can be called manually or by scheduler:

- `CleanupExpired(ctx, now, limit)`
- `CleanupUserOverLimit(ctx, userID, maxImages)`

**Step 3: Wire scheduler carefully**

If there is an existing periodic cleanup pattern, register this cleanup with a conservative interval. If not, leave it callable from service/admin first and document R2 lifecycle as backup.

**Step 4: Run tests**

Run:

```powershell
go test ./internal/service -run TestImageStudioCleanup -v
```

Expected: PASS.

**Step 5: Commit**

```powershell
git add backend/internal/service/image_studio_cleanup.go backend/internal/service/image_studio_cleanup_test.go
git commit -m "feat: add image studio cleanup"
```

## Task 12: End-To-End Verification

**Files:**
- Modify only if tests reveal issues.

**Step 1: Run backend focused tests**

```powershell
go test ./internal/service -run "TestImageStudio|TestImageStorage|TestImageBilling" -v
go test ./internal/handler -run "TestImageStudio|TestOpenAIImages" -v
go test ./internal/server/routes -run Test -v
```

Expected: PASS.

**Step 2: Run backend unit suite**

```powershell
make -C backend test-unit
```

Expected: PASS.

**Step 3: Run frontend checks**

```powershell
pnpm --dir frontend run test:run
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

Expected: PASS.

**Step 4: Run local manual flow**

Use existing local dev stack:

- Backend: `http://127.0.0.1:18080`
- Frontend Vite: `http://127.0.0.1:5173`

Verify:

- Admin enables Image Studio and configures allowed model.
- User opens `/images`.
- Text-to-image succeeds.
- Edit mode succeeds with an uploaded or pasted reference image.
- Image preview loads from local storage URL in development.
- Balance and usage record change after success.
- Delete removes image from gallery/history.
- Chinese/English and light/dark modes look correct.

**Step 5: Final commit if needed**

If verification fixes were needed:

```powershell
git add <changed-files>
git commit -m "fix: stabilize image studio verification"
```

## Deployment Notes

Before production deployment:

- Configure Cloudflare R2 bucket.
- Bind a public custom domain such as `assets.passionapi.com`.
- Set backend env vars:
  - `IMAGE_STORAGE_DRIVER=r2`
  - `R2_ACCOUNT_ID`
  - `R2_ACCESS_KEY_ID`
  - `R2_SECRET_ACCESS_KEY`
  - `R2_BUCKET`
  - `R2_PUBLIC_BASE_URL`
- Keep feature disabled until storage test passes in admin settings.
- Deploy with the existing no-downtime production process only after local verification passes.
