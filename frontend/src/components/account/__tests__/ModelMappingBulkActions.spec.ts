import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'),
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
    if (!params) return key
    return `${key}:${JSON.stringify(params)}`
  } })
}))


vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showInfo: vi.fn() }) }))

import ModelMappingBulkActions from '../ModelMappingBulkActions.vue'

describe('ModelMappingBulkActions', () => {
  it('generates prefixed request mappings while preserving existing entries', async () => {
    const wrapper = mount(ModelMappingBulkActions, {
      props: {
        modelValue: [{ from: 'manual', to: 'upstream-manual' }],
        allowedModels: ['claude-opus-4-6', 'claude-sonnet-4-6'],
      },
    })
    await wrapper.get('[data-testid="model-mapping-prefix"]').setValue('按次/')
    await wrapper.get('[data-testid="generate-mappings"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[
      { from: 'manual', to: 'upstream-manual' },
      { from: '按次/claude-opus-4-6', to: 'claude-opus-4-6' },
      { from: '按次/claude-sonnet-4-6', to: 'claude-sonnet-4-6' },
    ]])
  })

  it('adds a prefix to request names and leaves upstream names unchanged', async () => {
    const wrapper = mount(ModelMappingBulkActions, {
      props: {
        modelValue: [
          { from: 'claude-opus-4-6', to: 'claude-opus-4-6' },
          { from: '按次/claude-sonnet-4-6', to: 'claude-sonnet-4-6' },
        ],
        allowedModels: [],
      },
    })
    await wrapper.get('[data-testid="model-mapping-prefix"]').setValue('按次/')
    await wrapper.get('[data-testid="prepend-prefix"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[
      { from: '按次/claude-opus-4-6', to: 'claude-opus-4-6' },
      { from: '按次/claude-sonnet-4-6', to: 'claude-sonnet-4-6' },
    ]])
  })

  it('disables generation without whitelist models and rejects empty or wildcard prefixes', async () => {
    const wrapper = mount(ModelMappingBulkActions, {
      props: { modelValue: [], allowedModels: [] },
    })
    expect(wrapper.get('[data-testid="generate-mappings"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="model-mapping-prefix"]').setValue('foo*')
    expect(wrapper.get('[data-testid="generate-mappings"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="prepend-prefix"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.accounts.wildcardOnlyAtEnd')
  })

  it('cleans a shared upstream prefix and suffix while preserving request names', async () => {
    const wrapper = mount(ModelMappingBulkActions, {
      props: {
        modelValue: [{ from: 'old-name', to: 'a/gemini-preview' }],
        allowedModels: []
      }
    })
    await wrapper.get('[data-testid="model-mapping-prefix"]').setValue('测试/')
    await wrapper.get('[data-testid="remove-upstream-prefix"]').setValue('a/')
    await wrapper.get('[data-testid="remove-upstream-suffix"]').setValue('-preview')
    expect(wrapper.get('[data-testid="cleanup-preview"]').text()).toContain('1')
    await wrapper.get('[data-testid="cleanup-mapping-targets"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([[
      { from: '测试/gemini', to: 'a/gemini-preview' }
    ]])
  })
})
