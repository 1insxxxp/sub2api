import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AdminWorkbenchView.vue')
const viewSource = readFileSync(viewPath, 'utf8')
const panelPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionPanel.vue')
const panelSource = readFileSync(panelPath, 'utf8')

describe('AdminWorkbenchView mobile layout', () => {
  it('keeps the workbench page and generated-code panels width-safe on small screens', () => {
    expect(viewSource).toContain('admin-workbench-page')
    expect(viewSource).toContain('admin-workbench-generated-now')
    expect(viewSource).toContain('admin-workbench-generated-results')
    expect(viewSource).toContain('class="flex w-full shrink-0 items-center justify-end gap-2 sm:w-auto"')
  })

  it('keeps the commission calendar and its detail drawer inside the page width', () => {
    expect(panelSource).toContain('class="commission-calendar-layout grid min-w-0 items-start gap-4"')
  })
})
