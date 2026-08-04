import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../BatchImageGuideView.vue'), 'utf8')

describe('BatchImageGuideView mobile controls', () => {
  it('uses responsive Select controls in the create dialog', () => {
    expect(source).toContain(':options="createApiKeyOptions"')
    expect(source).toContain(':options="createModelOptions"')
    expect(source).toContain(':options="responseMimeTypeOptions"')
    expect(source).toContain(':options="outputCountSelectOptions"')
    expect(source).not.toContain('<select v-model.number="form.apiKeyId"')
    expect(source).not.toContain('<select v-model="form.model"')
  })
})
