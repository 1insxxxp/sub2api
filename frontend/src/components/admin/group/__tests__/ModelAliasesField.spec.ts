import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ModelAliasesField from '../ModelAliasesField.vue'

describe('group alias editor', () => {
  it('edits the model alias and upstream mapping without group continuation controls', async () => {
    const wrapper = mount(ModelAliasesField, {
      props: { modelValue: [{ alias: 'long-public', upstream_model: 'provider' }], idPrefix: 'test-alias' },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: {} })] },
    })
    expect(wrapper.findAll('input')).toHaveLength(2)
    expect(wrapper.get('#test-alias-0-alias').element).toHaveProperty('value', 'long-public')
    expect(wrapper.get('#test-alias-0-upstream').element).toHaveProperty('value', 'provider')
    expect(wrapper.text()).not.toContain('antiTruncation')

    await wrapper.get('button[aria-label="admin.groups.form.removeModelAlias"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[[]]])
  })
})
