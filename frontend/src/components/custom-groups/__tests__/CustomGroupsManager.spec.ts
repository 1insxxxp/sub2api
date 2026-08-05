import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../CustomGroupsManager.vue'), 'utf8')

describe('CustomGroupsManager', () => {
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
})
