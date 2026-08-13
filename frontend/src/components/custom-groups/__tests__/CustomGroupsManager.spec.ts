import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const apiMocks = vi.hoisted(() => ({
  list: vi.fn(),
  candidates: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/api/customGroups', () => ({
  customGroupsAPI: apiMocks,
}))

import CustomGroupsManager from '../CustomGroupsManager.vue'
import { useAppStore } from '@/stores/app'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../CustomGroupsManager.vue'), 'utf8')

const candidates = [
  { id: 11, name: 'Claude 满血', platform: 'anthropic', models: ['claude-sonnet-4-6', 'claude-opus-4-6'] },
  { id: 12, name: 'Gemini 反重力', platform: 'gemini', models: ['gemini-3.1-pro-preview'] },
]

const groups = [{
  id: 21,
  user_id: 7,
  name: '酒馆统一模型',
  status: 'active',
  models: [{
    id: 31,
    custom_group_id: 21,
    public_model: 'claude-sonnet-4-6',
    source_group_id: 11,
    source_model: 'claude-sonnet-4-6',
  }],
  created_at: '2026-08-08T00:00:00Z',
  updated_at: '2026-08-08T00:00:00Z',
}]

const groupsWithStaleSource = [{
  ...groups[0],
  models: [
    {
      ...groups[0].models[0],
      source_available: true,
      source_group: { id: 11, name: 'Claude 满血', status: 'active', platform: 'anthropic' },
    },
    {
      id: 32,
      custom_group_id: 21,
      public_model: 'claude-opus-backup',
      source_group_id: 98,
      source_model: 'claude-opus-4-6',
      source_available: false,
      source_issue: 'source_group_unavailable',
    },
  ],
}]

const mountManager = async () => {
  const wrapper = mount(CustomGroupsManager)
  await flushPromises()
  return wrapper
}

const deleteButton = (wrapper: Awaited<ReturnType<typeof mountManager>>) => {
  const button = wrapper.findAll('button').find(candidate => candidate.text() === '删除')
  if (!button) throw new Error('delete button not found')
  return button
}

describe('CustomGroupsManager', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    apiMocks.list.mockResolvedValue(groups)
    apiMocks.candidates.mockResolvedValue(candidates)
  })

  it('uses single-layer list and form modes for responsive modal use', () => {
    expect(source).toContain("mode === 'list'")
    expect(source).toContain("mode === 'form'")
    expect(source).toContain('data-test="custom-groups-back"')
    expect(source).toContain('min-h-0 flex-1 overflow-y-auto')
    expect(source).not.toContain('<BaseDialog')
  })

  it('shows editable call names with real model and source metadata in a mobile stack', () => {
    expect(source).toContain('调用名称')
    expect(source).toContain('真实模型')
    expect(source).toContain('来源分组')
    expect(source).toContain('sourceMappingKey')
    expect(source).toContain('grid-cols-1')
  })

  it('keeps every source group collapsed by default and expands one group at a time', async () => {
    const wrapper = await mountManager()

    await wrapper.get('[data-test="custom-groups-create"]').trigger('click')

    const claudeToggle = wrapper.get('[data-test="custom-group-source-toggle-11"]')
    expect(claudeToggle.attributes('aria-expanded')).toBe('false')
    expect(claudeToggle.text()).toContain('2 个模型')
    expect(wrapper.find('[data-test="custom-group-source-models-11"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="custom-group-source-models-12"]').exists()).toBe(false)

    await claudeToggle.trigger('click')

    expect(claudeToggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[data-test="custom-group-source-models-11"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="custom-group-source-models-12"]').exists()).toBe(false)
  })

  it('expands and collapses all source groups from one mobile-friendly control', async () => {
    const wrapper = await mountManager()

    await wrapper.get('[data-test="custom-groups-create"]').trigger('click')
    const expandAll = wrapper.get('[data-test="custom-group-sources-toggle-all"]')
    expect(expandAll.text()).toContain('全部展开')

    await expandAll.trigger('click')

    expect(wrapper.find('[data-test="custom-group-source-models-11"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="custom-group-source-models-12"]').exists()).toBe(true)
    expect(expandAll.text()).toContain('全部折叠')

    await expandAll.trigger('click')

    expect(wrapper.find('[data-test="custom-group-source-models-11"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="custom-group-source-models-12"]').exists()).toBe(false)
  })

  it('shows selected counts while keeping edit mode collapsed', async () => {
    const wrapper = await mountManager()

    await wrapper.get('[data-test="custom-groups-edit-21"]').trigger('click')

    const claudeToggle = wrapper.get('[data-test="custom-group-source-toggle-11"]')
    expect(claudeToggle.attributes('aria-expanded')).toBe('false')
    expect(claudeToggle.text()).toContain('已选 1')
    expect(wrapper.find('[data-test="custom-group-source-models-11"]').exists()).toBe(false)
  })

  it('marks stale source mappings in the group list', async () => {
    apiMocks.list.mockResolvedValue(groupsWithStaleSource)

    const wrapper = await mountManager()

    expect(wrapper.get('[data-test="custom-group-stale-summary-21"]').text()).toContain('1 个来源失效')
    const staleModel = wrapper.get('[data-test="custom-group-stale-model-32"]')
    expect(staleModel.text()).toContain('claude-opus-backup')
    expect(staleModel.classes()).toContain('text-red-700')
  })

  it('shows stale mappings in edit mode and requires removal before saving', async () => {
    apiMocks.list.mockResolvedValue(groupsWithStaleSource)
    const wrapper = await mountManager()

    await wrapper.get('[data-test="custom-groups-edit-21"]').trigger('click')

    const warning = wrapper.get('[data-test="custom-group-stale-sources"]')
    expect(warning.text()).toContain('存在失效来源线路')
    expect(warning.text()).toContain('来源分组已下架或停用')
    expect(warning.text()).toContain('claude-opus-backup')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="custom-group-remove-stale-32"]').trigger('click')

    expect(wrapper.find('[data-test="custom-group-stale-sources"]').exists()).toBe(false)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('treats omitted source availability from an old backend as valid', async () => {
    const wrapper = await mountManager()

    expect(wrapper.find('[data-test="custom-group-stale-summary-21"]').exists()).toBe(false)
    await wrapper.get('[data-test="custom-groups-edit-21"]').trigger('click')
    expect(wrapper.find('[data-test="custom-group-stale-sources"]').exists()).toBe(false)
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('distinguishes a source permission loss from a removed source group', async () => {
    const deniedModel = {
      ...groupsWithStaleSource[0].models[1],
      source_issue: 'source_group_not_allowed',
      source_group: { id: 98, name: '专属 Claude', status: 'active', platform: 'anthropic' },
    }
    apiMocks.list.mockResolvedValue([{
      ...groupsWithStaleSource[0],
      models: [groupsWithStaleSource[0].models[0], deniedModel],
    }])
    const wrapper = await mountManager()

    await wrapper.get('[data-test="custom-groups-edit-21"]').trigger('click')

    expect(wrapper.get('[data-test="custom-group-stale-sources"]').text()).toContain('已无权使用该来源分组')
    expect(wrapper.get('[data-test="custom-group-stale-sources"]').text()).toContain('专属 Claude')
  })

  it('confirms the affected API keys before forcing a bound custom group deletion', async () => {
    const confirm = vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)
      .mockReturnValueOnce(true)
    apiMocks.delete
      .mockRejectedValueOnce({
        status: 409,
        code: 409,
        reason: 'CUSTOM_GROUP_IN_USE',
        metadata: { bound_api_key_count: '2' },
        message: 'custom group is bound to one or more API keys',
      })
      .mockResolvedValueOnce({ deleted: true, unbound_api_key_count: 2 })
    const wrapper = await mountManager()

    await deleteButton(wrapper).trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledTimes(2)
    expect(confirm.mock.calls[1]?.[0]).toContain('2 个 API 密钥')
    expect(confirm.mock.calls[1]?.[0]).toContain('恢复使用原分组')
    expect(apiMocks.delete).toHaveBeenNthCalledWith(1, 21)
    expect(apiMocks.delete).toHaveBeenNthCalledWith(2, 21, true)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('does not force deletion when the affected-key confirmation is cancelled', async () => {
    vi.spyOn(window, 'confirm')
      .mockReturnValueOnce(true)
      .mockReturnValueOnce(false)
    apiMocks.delete.mockRejectedValueOnce({
      status: 409,
      code: 409,
      reason: 'CUSTOM_GROUP_IN_USE',
      metadata: { bound_api_key_count: '3' },
    })
    const wrapper = await mountManager()

    await deleteButton(wrapper).trigger('click')
    await flushPromises()

    expect(apiMocks.delete).toHaveBeenCalledTimes(1)
    expect(apiMocks.delete).toHaveBeenCalledWith(21)
    expect(wrapper.emitted('changed')).toBeUndefined()
  })

  it('shows a forced deletion failure instead of reporting success', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    apiMocks.delete
      .mockRejectedValueOnce({
        status: 409,
        code: 409,
        reason: 'CUSTOM_GROUP_IN_USE',
        metadata: { bound_api_key_count: '1' },
      })
      .mockRejectedValueOnce({ message: '解绑失败，请重试' })
    const wrapper = await mountManager()

    await deleteButton(wrapper).trigger('click')
    await flushPromises()

    const app = useAppStore()
    expect(app.toasts.some(toast => toast.type === 'error' && toast.message === '解绑失败，请重试')).toBe(true)
    expect(wrapper.emitted('changed')).toBeUndefined()
  })
})
