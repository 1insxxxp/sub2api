import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CategoryDefaultGroupDialog from '../CategoryDefaultGroupDialog.vue'
import { categoryDefaultGroupsAPI } from '@/api/admin/categoryDefaultGroups'

vi.mock('@/api/admin/categoryDefaultGroups', () => ({ categoryDefaultGroupsAPI: { list: vi.fn(), candidates: vi.fn(), preview: vi.fn(), apply: vi.fn(), setStatus: vi.fn() } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const route = { public_model: 'claude-example', source_group_id: 11, source_model: 'claude-example' }
const config = { group_id: 90, category: 'anthropic' as const, enabled: true, revision: 3, source_group_ids: [11], models: [route], models_count: 1 }
const preview = { revision: 3, preview_hash: 'reviewed-hash', added: [route], modified: [], removed: [], unchanged: [], conflicts: [], models_count: 1 }
const candidates = [{ group: { id: 11, name: 'Primary Claude', platform: 'anthropic', subscription_type: 'standard' }, models: ['claude-example'], models_count: 1 }]
const mountDialog = () => mount(CategoryDefaultGroupDialog, { props: { show: true }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }, Icon: true } } })

describe('CategoryDefaultGroupDialog', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(categoryDefaultGroupsAPI.list).mockResolvedValue([config])
    vi.mocked(categoryDefaultGroupsAPI.candidates).mockResolvedValue(candidates as never)
    vi.mocked(categoryDefaultGroupsAPI.preview).mockResolvedValue(preview)
    vi.mocked(categoryDefaultGroupsAPI.apply).mockResolvedValue({ ...config, revision: 4 })
    vi.mocked(categoryDefaultGroupsAPI.setStatus).mockResolvedValue({ ...config, enabled: false })
  })

  it('offers six categories and publishes only after preview and explicit confirmation', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="category-default-category"]').map(button => button.attributes('data-category'))).toEqual(['anthropic', 'openai', 'gemini', 'grok', 'domestic', 'other'])
    expect(wrapper.get('[data-testid="category-default-publish"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="category-default-preview"]').trigger('click')
    await flushPromises()
    expect(categoryDefaultGroupsAPI.preview).toHaveBeenCalledWith('anthropic', [11])
    expect(categoryDefaultGroupsAPI.apply).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="category-default-publish"]').trigger('click')
    await flushPromises()
    expect(categoryDefaultGroupsAPI.apply).toHaveBeenCalledWith('anthropic', [11], expect.objectContaining({ revision: 3, preview_hash: 'reviewed-hash' }))
    expect(wrapper.emitted('saved')?.[0]).toEqual([{ ...config, revision: 4 }])
    wrapper.unmount()
  })

  it('shows every conflicting model and its source groups and blocks publication', async () => {
    vi.mocked(categoryDefaultGroupsAPI.preview).mockResolvedValue({ ...preview, conflicts: [{ public_model: 'duplicate-model', source_group_ids: [11, 22], source_group_names: ['Primary Claude', 'Other Claude'] }] })
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="category-default-preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="category-default-conflicts"]').text()).toContain('duplicate-model')
    expect(wrapper.text()).toContain('Other Claude')
    expect(wrapper.get('[data-testid="category-default-publish"]').attributes('disabled')).toBeDefined()
    expect(categoryDefaultGroupsAPI.apply).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('invalidates a preview when source selection changes', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="category-default-preview"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="category-default-source"]').setValue(false)
    expect(wrapper.find('[data-testid="category-default-preview-result"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="category-default-publish"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('requires a fresh preview after a publication conflict', async () => {
    vi.mocked(categoryDefaultGroupsAPI.apply).mockRejectedValue({ response: { status: 409, data: { message: 'Configuration changed' } } })
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="category-default-preview"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="category-default-publish"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Configuration changed')
    expect(wrapper.get('[data-testid="category-default-publish"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('disables sources without a finite model catalog and changes status only on click', async () => {
    vi.mocked(categoryDefaultGroupsAPI.candidates).mockResolvedValue([...candidates, { group: { id: 22, name: 'Wildcard' }, models: [], models_count: 0, invalid_reason: 'Explicit models required' }] as never)
    const wrapper = mountDialog()
    await flushPromises()
    expect(wrapper.get('[data-source-id="22"]').attributes('disabled')).toBeDefined()
    expect(categoryDefaultGroupsAPI.setStatus).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="category-default-status"]').trigger('click')
    await flushPromises()
    expect(categoryDefaultGroupsAPI.setStatus).toHaveBeenCalledWith('anthropic', false)
    wrapper.unmount()
  })

  it('lets administrators remove missing or newly invalid sources from an existing configuration', async () => {
    vi.mocked(categoryDefaultGroupsAPI.list).mockResolvedValue([{ ...config, source_group_ids: [11, 22] }])
    vi.mocked(categoryDefaultGroupsAPI.candidates).mockResolvedValue([{ ...candidates[0], invalid_reason: 'Explicit models required' }] as never)
    const wrapper = mountDialog()
    await flushPromises()
    expect(wrapper.get('[data-source-id="11"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-source-id="22"]').setValue(false)
    await wrapper.get('[data-source-id="11"]').setValue(false)
    expect(wrapper.get('[data-testid="category-default-preview"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('ignores a delayed candidate response from the previously selected category', async () => {
    let resolveFirst!: (value: never) => void
    vi.mocked(categoryDefaultGroupsAPI.candidates).mockReturnValueOnce(new Promise(resolve => { resolveFirst = resolve }))
    const wrapper = mountDialog()
    await wrapper.get('[data-category="gemini"]').trigger('click')
    await flushPromises()
    resolveFirst([{ group: { id: 33, name: 'Stale Claude' }, models: [], models_count: 0 }] as never)
    await flushPromises()
    expect(wrapper.text()).not.toContain('Stale Claude')
    expect(wrapper.get('[data-category="gemini"]').attributes('aria-pressed')).toBe('true')
    wrapper.unmount()
  })
})
