# Custom Menu Open Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow each custom sidebar menu to open either as the existing embedded page or as a safe browser new-tab link, then configure the recharge-card menu to use the new-tab mode.

**Architecture:** Extend the existing JSON-backed `CustomMenuItem` contract with an optional `open_mode` enum. The backend normalizes legacy empty values to `embedded` and validates the enum, the settings UI edits it, and the sidebar maps `new_tab` items to external anchors while preserving existing router links for embedded and Markdown pages.

**Tech Stack:** Go/Gin backend, Vue 3 + TypeScript frontend, Vitest, Go `testing`.

---

### Task 1: Backend contract and validation

**Files:**
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/handler/admin/setting_handler_update.go`
- Create: `backend/internal/handler/admin/setting_handler_custom_menu_open_mode_test.go`

**Step 1: Write the failing tests**

Add table-driven tests in package `admin` that exercise a small `normalizeCustomMenuOpenMode` helper:

```go
func TestNormalizeCustomMenuOpenMode(t *testing.T) {
    tests := []struct {
        name     string
        item     dto.CustomMenuItem
        wantMode string
        wantErr  bool
    }{
        {name: "legacy empty mode defaults to embedded", item: dto.CustomMenuItem{URL: "https://example.com"}, wantMode: "embedded"},
        {name: "embedded is accepted", item: dto.CustomMenuItem{URL: "https://example.com", OpenMode: "embedded"}, wantMode: "embedded"},
        {name: "new tab is accepted", item: dto.CustomMenuItem{URL: "https://example.com", OpenMode: "new_tab"}, wantMode: "new_tab"},
        {name: "unknown mode is rejected", item: dto.CustomMenuItem{URL: "https://example.com", OpenMode: "popup"}, wantErr: true},
        {name: "markdown cannot open in new tab", item: dto.CustomMenuItem{URL: "md:help", OpenMode: "new_tab"}, wantErr: true},
    }
    // Copy each item, invoke normalizeCustomMenuOpenMode, and compare mode/error.
}
```

**Step 2: Run the focused test and verify RED**

Run:

```bash
go test ./backend/internal/handler/admin -run TestNormalizeCustomMenuOpenMode -count=1
```

Expected: build failure because `OpenMode` and `normalizeCustomMenuOpenMode` do not exist.

**Step 3: Add the DTO field and normalization helper**

Add to `dto.CustomMenuItem`:

```go
OpenMode string `json:"open_mode,omitempty"`
```

Add a helper in `setting_handler_update.go`:

```go
func normalizeCustomMenuOpenMode(item *dto.CustomMenuItem) error {
    item.OpenMode = strings.TrimSpace(item.OpenMode)
    if item.OpenMode == "" {
        item.OpenMode = "embedded"
    }
    if item.OpenMode != "embedded" && item.OpenMode != "new_tab" {
        return errors.New("custom menu item open mode must be 'embedded' or 'new_tab'")
    }
    if strings.HasPrefix(strings.TrimSpace(item.URL), "md:") && item.OpenMode != "embedded" {
        return errors.New("custom menu markdown pages must use embedded mode")
    }
    return nil
}
```

Call it for every submitted menu item before JSON serialization; return HTTP 400 with the helper error message.

**Step 4: Run tests and verify GREEN**

Run:

```bash
go test ./backend/internal/handler/admin -run 'TestNormalizeCustomMenuOpenMode|TestSettingHandler' -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/handler/dto/settings.go backend/internal/handler/admin/setting_handler_update.go backend/internal/handler/admin/setting_handler_custom_menu_open_mode_test.go
git commit -m "feat: validate custom menu open modes"
```

### Task 2: Frontend data contract and settings form

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/settings.ts`
- Modify: `frontend/src/i18n/locales/en/admin/settings.ts`

**Step 1: Write failing settings tests**

Add focused source-level assertions to the existing `SettingsView.spec.ts` for this large settings screen:

```ts
it('edits and defaults custom menu open mode', () => {
  expect(settingsSource).toContain('v-model="item.open_mode"')
  expect(settingsSource).toContain('open_mode: "embedded"')
  expect(settingsSource).toContain("item.url.trim().startsWith('md:')")
})
```

Also assert the payload continues to submit `form.custom_menu_items`.

**Step 2: Run focused test and verify RED**

Run:

```bash
cd frontend && npm run test:run -- src/views/admin/__tests__/SettingsView.spec.ts
```

Expected: FAIL because no open-mode control/default exists.

**Step 3: Implement the form contract and UI**

Extend `CustomMenuItem` and the local form item type:

```ts
open_mode?: 'embedded' | 'new_tab'
```

Normalize loaded items after settings assignment:

```ts
form.custom_menu_items = form.custom_menu_items.map((item) => ({
  ...item,
  open_mode: item.open_mode === 'new_tab' ? 'new_tab' : 'embedded',
}))
```

New items use `open_mode: "embedded"`. Add an “打开方式” select with `embedded` and `new_tab` options. Disable the `new_tab` option for `md:` URLs and reset a Markdown item to `embedded` before saving.

Add Chinese and English labels/help text explaining that new-tab mode does not append embedded authentication parameters.

**Step 4: Run focused test and typecheck**

Run:

```bash
cd frontend && npm run test:run -- src/views/admin/__tests__/SettingsView.spec.ts
cd frontend && npm run typecheck
```

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/views/admin/SettingsView.vue frontend/src/views/admin/__tests__/SettingsView.spec.ts frontend/src/i18n/locales/zh/admin/settings.ts frontend/src/i18n/locales/en/admin/settings.ts
git commit -m "feat: configure custom menu open mode"
```

### Task 3: Sidebar external-link behavior

**Files:**
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Create: `frontend/src/utils/custom-menu-navigation.ts`
- Create: `frontend/src/utils/__tests__/custom-menu-navigation.spec.ts`
- Modify: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`

**Step 1: Write failing navigation tests**

Test a pure navigation mapper:

```ts
expect(resolveCustomMenuNavigation({ id: 'card', url: 'https://card.example.com', open_mode: 'new_tab' }))
  .toEqual({ path: 'https://card.example.com', externalUrl: 'https://card.example.com' })

expect(resolveCustomMenuNavigation({ id: 'help', url: 'https://help.example.com' }))
  .toEqual({ path: '/custom/help' })

expect(resolveCustomMenuNavigation({ id: 'docs', url: 'md:docs', open_mode: 'new_tab' }))
  .toEqual({ path: '/custom/docs' })
```

Add AppSidebar source assertions for `target="_blank"`, `rel="noopener noreferrer"`, and the external URL branch.

**Step 2: Run focused tests and verify RED**

Run:

```bash
cd frontend && npm run test:run -- src/utils/__tests__/custom-menu-navigation.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts
```

Expected: FAIL because the mapper and external anchor branch do not exist.

**Step 3: Implement the mapper and sidebar rendering**

Create:

```ts
export function resolveCustomMenuNavigation(item: Pick<CustomMenuItem, 'id' | 'url' | 'open_mode'>) {
  if (item.open_mode === 'new_tab' && /^https?:\/\//i.test(item.url)) {
    return { path: item.url, externalUrl: item.url }
  }
  return { path: `/custom/${item.id}` }
}
```

Add `externalUrl?: string` to `NavItem` and spread the mapper result when mapping user/admin custom items. In each normal navigation loop, render:

```vue
<a
  v-if="item.externalUrl"
  :href="item.externalUrl"
  target="_blank"
  rel="noopener noreferrer"
  class="sidebar-link mb-1"
  @click="handleMenuItemClick(item.path)"
>
  <!-- same icon and label markup -->
</a>
<router-link v-else ...>
  <!-- existing markup -->
</router-link>
```

Apply the branch consistently to administrator custom items, administrator personal items, and normal-user items. Do not call `buildEmbeddedUrl` or append user/token query parameters for an external item.

**Step 4: Run focused tests, full frontend tests, and build**

Run:

```bash
cd frontend && npm run test:run -- src/utils/__tests__/custom-menu-navigation.spec.ts src/components/layout/__tests__/AppSidebar.spec.ts
cd frontend && npm run test:run
cd frontend && npm run build
```

Expected: PASS with no TypeScript errors.

**Step 5: Commit**

```bash
git add frontend/src/components/layout/AppSidebar.vue frontend/src/components/layout/__tests__/AppSidebar.spec.ts frontend/src/utils/custom-menu-navigation.ts frontend/src/utils/__tests__/custom-menu-navigation.spec.ts
git commit -m "feat: open selected custom menus in new tabs"
```

### Task 4: Cross-layer verification and local smoke test

**Files:**
- No production file changes expected.

**Step 1: Format and run backend tests**

Run:

```bash
gofmt -w backend/internal/handler/dto/settings.go backend/internal/handler/admin/setting_handler_update.go backend/internal/handler/admin/setting_handler_custom_menu_open_mode_test.go
go test ./backend/internal/handler/admin ./backend/internal/handler/dto ./backend/internal/service
```

Expected: PASS.

**Step 2: Run repository checks**

Run the project’s documented lint/build commands and `git diff --check`.

Expected: all checks pass and there is no whitespace damage.

**Step 3: Start the local stack and smoke-test both modes**

Use the repository’s existing local run command. In the browser:

1. Existing embedded menu stays inside `/custom/:id`;
2. A `new_tab` test item opens exactly its configured URL in a new tab;
3. Mobile sidebar closes after clicking the external item;
4. No `token`, `user_id`, `ui_mode`, or source query parameters are appended.

**Step 4: Commit any test-only corrections**

```bash
git add <only-files-needed-for-corrections>
git commit -m "test: cover custom menu opening modes"
```

### Task 5: Deployment configuration (only after explicit deployment authorization)

**Files:**
- Production setting `custom_menu_items` only; no source file edit.

**Step 1: Deploy the verified code through the existing deployment workflow**

Do not deploy until the user explicitly authorizes deployment.

**Step 2: Update the recharge-card menu**

Preserve the existing item and add only:

```json
"open_mode": "new_tab"
```

for menu ID `5e7074ea4b221e60` (“充值卡网（无需登录）”).

**Step 3: Verify production behavior**

Confirm the public settings API returns `open_mode: "new_tab"`, then verify a customer click opens `https://card.passionapi.com/categories/zhongzhuan` in a new tab without embedded query parameters. Complete a non-financial navigation smoke test only; do not create or pay an order.

**Step 4: Observe for regressions**

Check application logs for settings validation or frontend errors and confirm existing embedded custom pages still load.
