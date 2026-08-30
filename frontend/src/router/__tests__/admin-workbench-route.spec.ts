import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8'
)
const metaSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../meta.d.ts'),
  'utf8'
)

describe('admin workbench route', () => {
  it('declares a narrow workbench permission separate from full admin access', () => {
    expect(metaSource).toContain('requiresAdminWorkbench?: boolean')
    expect(routerSource).toContain("path: '/admin/workbench'")
    expect(routerSource).toContain("component: () => import('@/views/admin/AdminWorkbenchView.vue')")
    expect(routerSource).toContain('requiresAdminWorkbench: true')
  })
})
