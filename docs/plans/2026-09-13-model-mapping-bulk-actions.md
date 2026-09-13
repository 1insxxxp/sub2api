# Model Mapping Bulk Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add one-click actions to generate request-name mappings from the selected model whitelist and prepend a shared prefix to existing request names in account creation and editing forms.

**Architecture:** Keep persistence unchanged: the forms continue serializing `credentials.model_mapping`. Add pure, reusable helpers in `useModelWhitelist.ts` for deduplicated mapping generation and idempotent prefix application. Add a small toolbar to the existing mapping panels in `CreateAccountModal.vue` and `EditAccountModal.vue`; generated mappings remain editable before save. The prefix applies to the left/request model name, while the right/upstream model name is preserved.

**Tech Stack:** Vue 3 Composition API, TypeScript, vue-i18n, Vitest, Tailwind utility classes.

---

### Task 1: Add pure mapping helpers and tests

**Files:**
- Modify: `frontend/src/composables/useModelWhitelist.ts`
- Test: `frontend/src/composables/__tests__/useModelWhitelist.spec.ts`

**Step 1: Write the failing tests**

Add tests for:
- generating `prefix + whitelistModel -> whitelistModel` entries;
- retaining an existing mapping with the same `from` value;
- ignoring blank whitelist entries and duplicate whitelist entries;
- applying a prefix only to non-empty `from` values, skipping rows already prefixed, and preserving `to`;
- returning unchanged rows when the prefix is blank.

**Step 2: Run the focused test file**

Run: `cd frontend && pnpm vitest run src/composables/__tests__/useModelWhitelist.spec.ts`
Expected: FAIL because the helpers do not exist.

**Step 3: Implement the minimal helpers**

Export typed helpers that return new arrays without mutating callers:
- `generateModelMappingsFromWhitelist(allowedModels, prefix, existingMappings)` trims values, builds `prefix + model -> model`, skips blank/duplicate sources, and never overwrites an existing source.
- `prependModelMappingPrefix(mappings, prefix)` trims the prefix, leaves blank prefixes and blank rows unchanged, prepends only to non-empty request names, skips rows whose request name already starts with the prefix, and preserves targets.

**Step 4: Run the focused tests**

Run: `cd frontend && pnpm vitest run src/composables/__tests__/useModelWhitelist.spec.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/composables/useModelWhitelist.ts frontend/src/composables/__tests__/useModelWhitelist.spec.ts
git commit -m "feat: add bulk model mapping helpers"
```

### Task 2: Add bilingual UI strings

**Files:**
- Modify: `frontend/src/i18n/locales/zh/admin/accounts.ts`
- Modify: `frontend/src/i18n/locales/en/admin/accounts.ts`

**Step 1: Add strings**

Add labels and feedback messages for generating mappings from the whitelist, entering a shared prefix, applying the prefix, empty whitelist/prefix validation, and counts of added/updated mappings. Keep keys identical in both locale files.

**Step 2: Run locale tests**

Run: `cd frontend && pnpm vitest run src/i18n/__tests__/localeParity.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts`
Expected: PASS.

**Step 3: Commit**

```bash
git add frontend/src/i18n/locales/zh/admin/accounts.ts frontend/src/i18n/locales/en/admin/accounts.ts
git commit -m "feat: add model mapping bulk action translations"
```

### Task 3: Integrate actions into account forms

**Files:**
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/components/account/CreateAccountModal.vue`

**Step 1: Add toolbar UI to each ordinary mapping panel**

Place a shared-looking toolbar above the mapping rows in the existing general mapping sections (including platform-specific ordinary sections that use `modelMappings`). Include a prefix input, a button to generate mappings from `allowedModels`, and a button to prepend the prefix. Disable generation when the whitelist is empty and keep the controls accessible on narrow screens.

**Step 2: Wire the helpers into modal state**

Import the pure helpers. Add handlers that update `modelMappings` with fresh arrays, preserve existing rows, switch Create form state to a mode that persists both whitelist and generated mappings when necessary, and show localized feedback counts. Keep all save payloads on the existing `model_mapping` path.

**Step 3: Verify ordinary and platform-specific mapping panels**

Confirm the handlers are used by every `modelMappings` mapping panel in both modals and do not affect Antigravity’s separate `antigravityModelMappings` flow or OpenAI compact mappings.

**Step 4: Run component/type checks**

Run: `cd frontend && pnpm vitest run src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/CreateAccountModal.spec.ts`
Run: `cd frontend && pnpm typecheck`
Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/components/account/EditAccountModal.vue frontend/src/components/account/CreateAccountModal.vue
git commit -m "feat: add account model mapping bulk actions"
```

### Task 4: Full frontend verification

**Files:**
- Verify only; no new files.

**Step 1: Run focused helper and component tests**

Run: `cd frontend && pnpm vitest run src/composables/__tests__/useModelWhitelist.spec.ts src/components/account/__tests__/ModelWhitelistSelector.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/CreateAccountModal.spec.ts`

**Step 2: Run lint and build checks**

Run: `cd frontend && pnpm lint:check`
Run: `cd frontend && pnpm build`
Expected: all commands pass.

**Step 3: Inspect the final diff**

Run: `git diff --check HEAD~3..HEAD` and review that only the planned frontend files and design plan changed; retain the pre-existing `AGENTS.md` modification.
