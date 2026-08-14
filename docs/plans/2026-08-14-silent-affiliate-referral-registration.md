# Silent Affiliate Referral Registration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove every visible referral hint from the registration page while an affiliate link is resolving or a valid server referral lock exists.

**Architecture:** Keep the existing `affiliateReferralResolving` and `affiliateReferralLocked` state because registration and OAuth need it, but render no referral DOM for those states. Preserve the existing manual input and invalid-link error only when resolution is complete and no valid lock exists.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils.

---

### Task 1: Make locked referral handling visually silent

**Files:**

- Modify: `frontend/src/views/auth/RegisterView.vue:138-180`
- Test: `frontend/src/views/auth/__tests__/RegisterView.spec.ts`

**Step 1: Write the failing tests**

Update the resolving-state and locked-state tests to assert:

```ts
expect(wrapper.find('[data-testid="affiliate-referral-resolving"]').exists()).toBe(false)
expect(wrapper.find('[data-testid="affiliate-referral-locked"]').exists()).toBe(false)
expect(wrapper.find('[data-testid="affiliate-code-input"]').exists()).toBe(false)
expect(wrapper.text()).not.toContain('auth.affiliateReferralLocked')
expect(wrapper.text()).not.toContain('auth.affiliateReferralResolving')
```

Keep the existing direct-access and invalid-without-lock assertions that require the manual input and validation error.

**Step 2: Run the focused test and verify RED**

Run:

```bash
cd frontend
pnpm exec vitest run src/views/auth/__tests__/RegisterView.spec.ts
```

Expected: FAIL because the resolving and locked status boxes are still rendered.

**Step 3: Implement the minimal template change**

Remove the resolving and locked status blocks. Render the manual affiliate-code form only when:

```vue
v-if="affiliateEnabled && !affiliateReferralResolving && !affiliateReferralLocked"
```

Do not modify referral state, submission, resolver, OAuth or backend logic.

**Step 4: Verify GREEN and regression coverage**

Run:

```bash
cd frontend
pnpm exec vitest run src/views/auth/__tests__/RegisterView.spec.ts
pnpm run lint:check
pnpm exec vue-tsc --noEmit
```

Expected: all commands exit 0.

Then reload `http://localhost:3001/register?aff=<valid-code>` and confirm the invitation area is absent with no console errors.

**Step 5: Commit**

```bash
git add frontend/src/views/auth/RegisterView.vue frontend/src/views/auth/__tests__/RegisterView.spec.ts
git commit -m "fix: hide locked referral status"
```
