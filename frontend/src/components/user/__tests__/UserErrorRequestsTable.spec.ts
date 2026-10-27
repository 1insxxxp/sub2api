import { expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UserErrorRequestsTable from '../UserErrorRequestsTable.vue'
import type { UserErrorRequest } from '@/types'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

it('shows an interrupted HTTP 200 separately from a successful stream', () => {
  const wrapper = mount(UserErrorRequestsTable, {
    props: {
      rows: [{ id: 1, status_code: 200, client_canceled: true, category: 'client_cancel' } as UserErrorRequest],
      total: 1, page: 1, pageSize: 20, loading: false,
    },
    global: { stubs: {
      IpGeoBatchToolbar: true,
      Pagination: true,
      UserErrorDetailModal: true,
      DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-status" :row="row" /><slot name="cell-category" :row="row" /></div></div>' },
    } },
  })
  expect(wrapper.text()).toContain('200')
  expect(wrapper.get('[data-testid="error-client-canceled"]').attributes('title')).toBe('usage.errors.clientCanceledHint')
  expect(wrapper.text()).toContain('usage.errors.categories.client_cancel')
  wrapper.unmount()
})
