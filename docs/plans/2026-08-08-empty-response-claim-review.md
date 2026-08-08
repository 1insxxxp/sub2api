# 空回申请审核信息完善 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在不暴露请求/响应正文的前提下，为空回申请审批提供完整结构化证据、计费上下文、补偿来源，并完善批量审批反馈。

**Architecture:** 后端在现有空回申请列表查询中一次性关联 `usage_logs` 和 `usage_response_outcomes`，将非敏感用量字段与结构化证据映射到管理员 DTO；前端沿用“使用记录 → 空回申请”页，在现有列表上扩展信息面板与审批弹窗。批量接口继续逐条调用既有状态机和补偿事务，返回成功/失败集合，不改变补偿幂等逻辑。

**Tech Stack:** Go、PostgreSQL、Gin、Vue 3、TypeScript、Vitest、Go unit/sqlmock tests。

---

### Task 1: 提交已批准的设计文档

**Files:**
- Create: `docs/plans/2026-08-08-empty-response-claim-review-design.md`

**Step 1: Verify the design document contains the privacy boundary and batch behavior**

Run: `rg -n "隐私|批量|结构化|响应正文" docs/plans/2026-08-08-empty-response-claim-review-design.md`

Expected: all four topics are present.

**Step 2: Commit the approved design**

Run: `git add docs/plans/2026-08-08-empty-response-claim-review-design.md && git commit -m "docs: design empty response claim review"`

Expected: a commit is created without staging the user-owned root `package.json` files.

### Task 2: Add failing backend tests for review context and compensation source

**Files:**
- Modify: `backend/internal/repository/empty_response_claim_repo_test.go`
- Modify: `backend/internal/handler/admin/empty_response_claim_handler_test.go`
- Modify: `backend/internal/service/empty_response_claim_admin_test.go`

**Step 1: Write tests for usage metadata mapping**

Assert that the admin claim list maps request ID/time, token counts, cost fields, billing/request type, endpoints, duration, and response evidence without any prompt or response body field.

**Step 2: Write a test for compensation source**

Assert that a compensated claim with no reviewer is serialized as `automatic`, while a compensated claim with `ReviewedBy` is `manual`; rejected/manual-review claims are `none`.

**Step 3: Run the focused tests to verify RED**

Run: `go test -tags unit ./internal/repository ./internal/handler/admin ./internal/service -run 'EmptyResponseClaim|EmptyResponse' -count=1`

Expected: FAIL because the new fields and source mapping do not yet exist.

### Task 3: Implement backend review-context projection

**Files:**
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/empty_response_claim.go`
- Modify: `backend/internal/repository/empty_response_claim_repo.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`

**Step 1: Add a narrow usage-context struct**

Add only the non-sensitive fields needed by the reviewer; do not add request body, response text, API key value, or prompt fields.

**Step 2: Extend the list query and scanner**

Join `usage_logs` and `usage_response_outcomes`, scan nullable historical fields safely, and preserve current claims pagination/filtering.

**Step 3: Add source mapping**

Expose `automatic`, `manual`, or `none` based on compensated status and reviewer presence, while preserving `reviewed_by` and `reviewed_at` for audit display.

**Step 4: Run focused backend tests to verify GREEN**

Run: `go test -tags unit ./internal/repository ./internal/handler/admin ./internal/service -run 'EmptyResponseClaim|EmptyResponse' -count=1`

Expected: PASS.

### Task 4: Add failing frontend tests for the expanded review surface

**Files:**
- Modify: `frontend/src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts`
- Create or modify: `frontend/src/components/admin/usage/__tests__/EmptyResponseClaimReviewDialog.spec.ts`

**Step 1: Assert list evidence and usage context render**

Mount the panel with a claim containing historical usage metadata and assert the model, request time, token/cost summary, evidence flags, and missing-evidence warning are visible.

**Step 2: Assert the approval dialog renders the same context**

Assert desktop information sections, the privacy-safe wording, user reason, and the review-source label are visible; assert prompt/response fields are not rendered.

**Step 3: Assert batch feedback**

Mock a batch result with both succeeded and failed IDs and assert the UI reports each count/reason and refreshes the list without hiding failures.

**Step 4: Run focused frontend tests to verify RED**

Run: `cd frontend && npm run test:unit -- src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts src/components/admin/usage/__tests__/EmptyResponseClaimReviewDialog.spec.ts`

Expected: FAIL because the new fields and review sections are not rendered.

### Task 5: Implement desktop/mobile review UI and batch feedback

**Files:**
- Modify: `frontend/src/api/admin/usage.ts`
- Modify: `frontend/src/components/admin/usage/EmptyResponseClaimsPanel.vue`
- Modify: `frontend/src/components/admin/usage/EmptyResponseClaimReviewDialog.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`

**Step 1: Extend TypeScript DTOs**

Add nullable-safe usage context, response evidence, reviewer, and compensation-source fields matching the backend JSON.

**Step 2: Add a compact evidence summary and expanded review sections**

Use the existing slate/emerald/amber visual language, with a prominent amber banner for missing evidence. Keep the dialog wide on desktop and bottom-sheet with scrollable sections on mobile.

**Step 3: Add batch confirmation and result summary**

Show selected count and estimated total before submission. After the request, display succeeded/failed counts and failed reasons, keep failed IDs selected, and reload metrics/list.

**Step 4: Add bilingual labels**

Add translations for usage metadata, evidence labels, source labels, batch summary, and privacy notice.

**Step 5: Run focused frontend tests to verify GREEN**

Run: `cd frontend && npm run test:unit -- src/components/admin/usage/__tests__/EmptyResponseClaimsPanel.spec.ts src/components/admin/usage/__tests__/EmptyResponseClaimReviewDialog.spec.ts`

Expected: PASS.

### Task 6: Verify the whole local change

**Files:**
- No additional files.

**Step 1: Run backend relevant tests**

Run: `go test -tags unit ./internal/repository ./internal/service ./internal/handler/admin -run 'EmptyResponseClaim|EmptyResponse' -count=1`

Expected: PASS.

**Step 2: Run frontend test suite and build**

Run: `cd frontend && npm run test:unit && npm run build`

Expected: PASS with a successful production bundle.

**Step 3: Review the diff and local status**

Run: `git diff --check && git status --short`

Expected: no whitespace errors; only intended source/docs changes plus the pre-existing untracked root package files.

**Step 4: Commit the implementation on `dev`**

Run: `git add backend frontend docs/plans/2026-08-08-empty-response-claim-review.md && git commit -m "feat: improve empty response claim review"`

Expected: implementation commit is created on `dev`; no deployment or merge to `master` is performed.
