import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../CustomGroupsView.vue'), 'utf8')

describe('CustomGroupsView layout', () => {
  it('renders inside the shared application shell', () => {
    expect(source).toContain('<AppLayout>')
    expect(source).toContain("import AppLayout from '@/components/layout/AppLayout.vue'")
    expect(source).toContain('<CustomGroupsManager')
    expect(source).toContain("import CustomGroupsManager from '@/components/custom-groups/CustomGroupsManager.vue'")
  })
})
