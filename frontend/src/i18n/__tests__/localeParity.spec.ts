import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

function collectLeafMessages(
  messages: LocaleMessages,
  prefix = '',
  leaves = new Map<string, unknown>(),
): Map<string, unknown> {
  for (const [key, value] of Object.entries(messages)) {
    const path = prefix ? `${prefix}.${key}` : key

    if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      collectLeafMessages(value as LocaleMessages, path, leaves)
    } else {
      leaves.set(path, value)
    }
  }

  return leaves
}

const zhMessages = collectLeafMessages(zh)
const enMessages = collectLeafMessages(en)

const requiredEmptyResponseKeys = [
  'usage.emptyResponse.title',
  'usage.emptyResponse.eyebrow',
  'usage.emptyResponse.model',
  'usage.emptyResponse.manualReview',
  'usage.emptyResponse.autoReview',
  'usage.emptyResponse.outputTokens',
  'usage.emptyResponse.originalCharge',
  'usage.emptyResponse.expectedRefund',
  'usage.emptyResponse.rules',
  'usage.emptyResponse.reason',
  'usage.emptyResponse.reasonPlaceholder',
  'usage.emptyResponse.submitting',
  'usage.emptyResponse.submit',
  'usage.emptyResponse.refunded',
  'usage.emptyResponse.netCharge',
  'usage.emptyResponse.action',
  'usage.emptyResponse.actionHint',
  'usage.emptyResponse.submitSuccess',
  'usage.emptyResponse.submitFailed',
  'usage.emptyResponse.status.evaluating',
  'usage.emptyResponse.status.manual_review',
  'usage.emptyResponse.status.approved',
  'usage.emptyResponse.status.rejected',
  'usage.emptyResponse.status.compensated',
  'usage.emptyResponse.reasonCode.pure_empty',
  'usage.emptyResponse.reasonCode.low_output',
  'usage.emptyResponse.reasonCode.upstream_http_5xx',
  'usage.emptyResponse.reasonCode.upstream_timeout',
  'usage.emptyResponse.reasonCode.upstream_interrupted',
  'usage.emptyResponse.reasonCode.client_cancelled',
  'usage.emptyResponse.reasonCode.effective_output',
  'usage.emptyResponse.reasonCode.not_charged',
  'usage.emptyResponse.reasonCode.already_compensated',
  'usage.emptyResponse.reasonCode.group_disabled',
  'usage.emptyResponse.reasonCode.claim_window_expired',
  'usage.emptyResponse.reasonCode.missing_evidence',
  'usage.emptyResponse.reasonCode.conflicting_evidence',
  'usage.emptyResponse.reasonCode.daily_limit_manual_review',
  'usage.emptyResponse.reasonCode.already_claimed',
  'admin.usage.emptyResponseClaims.tab',
  'admin.usage.emptyResponseClaims.rate',
  'admin.usage.emptyResponseClaims.warning',
  'admin.usage.emptyResponseClaims.claims',
  'admin.usage.emptyResponseClaims.refunded',
  'admin.usage.emptyResponseClaims.pending',
  'admin.usage.emptyResponseClaims.allStatuses',
  'admin.usage.emptyResponseClaims.selectPage',
  'admin.usage.emptyResponseClaims.clearPageSelection',
  'admin.usage.emptyResponseClaims.selectedCount',
  'admin.usage.emptyResponseClaims.batchApprove',
  'admin.usage.emptyResponseClaims.batchReject',
  'admin.usage.emptyResponseClaims.batchReviewItems',
  'admin.usage.emptyResponseClaims.expandDetails',
  'admin.usage.emptyResponseClaims.collapseDetails',
  'admin.usage.emptyResponseClaims.approve',
  'admin.usage.emptyResponseClaims.reject',
  'admin.usage.emptyResponseClaims.identity',
  'admin.usage.emptyResponseClaims.evidence',
  'admin.usage.emptyResponseClaims.refund',
  'admin.usage.emptyResponseClaims.statusLabel',
  'admin.usage.emptyResponseClaims.empty',
  'admin.usage.emptyResponseClaims.loadFailed',
  'admin.usage.emptyResponseClaims.reviewSuccess',
  'admin.usage.emptyResponseClaims.reviewFailed',
  'admin.usage.emptyResponseClaims.rejectionNote',
  'admin.usage.emptyResponseClaims.reviewNote',
  'admin.usage.emptyResponseClaims.lowOutputNotice',
  'admin.usage.emptyResponseClaims.rankings.group',
  'admin.usage.emptyResponseClaims.rankings.account',
  'admin.usage.emptyResponseClaims.rankings.model',
  'admin.usage.emptyResponseClaims.evidenceSummary',
  'admin.usage.emptyResponseClaims.complete',
  'admin.usage.emptyResponseClaims.interrupted',
  'admin.usage.emptyResponseClaims.status.evaluating',
  'admin.usage.emptyResponseClaims.status.manual_review',
  'admin.usage.emptyResponseClaims.status.approved',
  'admin.usage.emptyResponseClaims.status.rejected',
  'admin.usage.emptyResponseClaims.status.compensated',
]

describe('locale parity', () => {
  it('keeps the complete Chinese and English message trees in sync', () => {
    const zhOnly = [...zhMessages.keys()].filter((key) => !enMessages.has(key))
    const enOnly = [...enMessages.keys()].filter((key) => !zhMessages.has(key))

    expect({ zhOnly, enOnly }).toEqual({ zhOnly: [], enOnly: [] })
  })

  it.each([
    ['zh', zhMessages],
    ['en', enMessages],
  ])('contains only non-empty leaf messages in %s', (_locale, messages) => {
    const invalidMessages = [...messages].filter(
      ([, value]) => typeof value !== 'string' || value.trim() === '',
    )

    expect(invalidMessages).toEqual([])
  })

  it.each([
    ['zh', zhMessages],
    ['en', enMessages],
  ])('contains every empty-response compensation message in %s', (_locale, messages) => {
    const missing = requiredEmptyResponseKeys.filter((key) => !messages.has(key))
    expect(missing).toEqual([])
  })
})
