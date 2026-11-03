import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import AccountAntiTruncationEditor from '../AccountAntiTruncationEditor.vue'

const mountEditor = (value: unknown = null) => mount(AccountAntiTruncationEditor, {
  props: { modelValue: value, models: ['gemini-4-argon'] } as any,
  global: { plugins: [createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: {} })] },
})

describe('AccountAntiTruncationEditor', () => {
  it('does not silently enable account rules when opened', () => {
    const wrapper = mountEditor()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.find('[data-testid="anti-truncation-add-rule"]').exists()).toBe(false)
  })

  it('explicitly takes ownership with an empty policy', async () => {
    const wrapper = mountEditor()
    await wrapper.get('[data-testid="anti-truncation-account-enabled"]').setValue(true)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ version: 1, rules: [] }])
  })

  it('resets the local off switch when a new account policy is loaded', async () => {
    const wrapper = mountEditor({ version: 1, rules: [{ model: 'old', enabled: true }] })
    await wrapper.get('[data-testid="anti-truncation-account-enabled"]').setValue(false)
    await wrapper.setProps({ modelValue: { version: 1, rules: [{ model: 'next', enabled: true }] } })
    expect(wrapper.find('[data-testid="anti-truncation-model-0"]').exists()).toBe(true)
  })

  it('adds a per-model rule and keeps zero output budget', async () => {
    const wrapper = mountEditor({ version: 1, rules: [] })
    await wrapper.get('[data-testid="anti-truncation-add-rule"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ version: 1, rules: [{ model: '', enabled: true, max_rounds: 1, max_output_tokens: 0, max_duration_seconds: 180 }] }])
  })

  it('removing the last rule disables all models without restoring legacy inheritance', async () => {
    const wrapper = mountEditor({ version: 1, rules: [{ model: 'alias', enabled: true }] })
    await wrapper.get('[data-testid="anti-truncation-remove-rule-0"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ version: 1, rules: [] }])
  })

  it('turning the account switch off clears rules explicitly rather than re-enabling legacy policies', async () => {
    const wrapper = mountEditor({ version: 1, rules: [{ model: 'alias', enabled: true }] })
    await wrapper.get('[data-testid="anti-truncation-account-enabled"]').setValue(false)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([{ version: 1, rules: [] }])
  })
})
