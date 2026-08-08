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

const mountManager = async () => {
  const wrapper = mount(CustomGroupsManager)
  await flushPromises()
  return wrapper
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
})
