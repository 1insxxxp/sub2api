import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OpsErrorDetailsModal from '../OpsErrorDetailsModal.vue'

vi.mock('vue-i18n', async (importOriginal) => { const actual = await importOriginal<typeof import('vue-i18n')>(); return { ...actual, useI18n: () => ({ t: (key: string) => key }) } })
vi.mock('@/api/admin/ops', async (importOriginal) => { const actual = await importOriginal<typeof import('@/api/admin/ops')>('@/api/admin/ops'); return { ...actual, opsAPI: { ...actual.opsAPI, listUpstreamErrors: vi.fn().mockResolvedValue({ items: [], total: 0 }), listRequestErrors: vi.fn().mockResolvedValue({ items: [], total: 0 }) } } })
const BaseDialog = { props: ['show'], template: '<div v-if="show"><slot /></div>' }
const Table = { template: '<div />' }
function mountModal(errorType: 'request' | 'upstream') { return mount(OpsErrorDetailsModal, { props: { show: true, timeRange: '1h', errorType }, global: { stubs: { BaseDialog, OpsErrorLogTable: Table, Select: { template: '<div />' } } } }) }
describe('OpsErrorDetailsModal summary entry', () => {
  it('shows and emits summary for upstream errors', async () => { const w = mountModal('upstream'); const b = w.findAll('button').find(x => x.text().includes('summaryButton')); expect(b).toBeTruthy(); await b!.trigger('click'); expect(w.emitted('openSummary')).toHaveLength(1) })
  it('does not show summary entry for request errors', () => { const w = mountModal('request'); expect(w.findAll('button').some(x => x.text().includes('summaryButton'))).toBe(false) })
})
