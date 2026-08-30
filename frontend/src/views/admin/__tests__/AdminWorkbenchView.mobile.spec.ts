import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AdminWorkbenchView.vue')
const viewSource = readFileSync(viewPath, 'utf8')
const panelPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionPanel.vue')
const panelSource = readFileSync(panelPath, 'utf8')
const calendarPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionCalendar.vue')
const calendarSource = readFileSync(calendarPath, 'utf8')
const drawerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionDayDrawer.vue')
const drawerSource = readFileSync(drawerPath, 'utf8')
const managementPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionManagement.vue')
const managementSource = readFileSync(managementPath, 'utf8')

describe('AdminWorkbenchView mobile layout', () => {
  it('keeps the workbench page and generated-code panels width-safe on small screens', () => {
    expect(viewSource).toContain('admin-workbench-page')
    expect(viewSource).toContain('admin-workbench-generated-now')
    expect(viewSource).toContain('admin-workbench-generated-results')
    expect(viewSource).toContain('grid-cols-3')
    expect(viewSource).toContain('sm:flex')
    expect(viewSource).toContain('min-h-16')
    expect(viewSource).toContain('flex-col')
    expect(viewSource).toContain('max-h-72')
    expect(viewSource).toContain('min-[360px]:flex-row')
    expect(viewSource).toContain('grid w-full grid-cols-2')
    expect(viewSource).toContain('class="grid w-full shrink-0 grid-cols-2 gap-2 sm:flex sm:w-auto"')
  })

  it('keeps the commission calendar and its detail drawer inside the page width', () => {
    expect(panelSource).toContain('class="commission-calendar-layout grid min-w-0 items-start gap-4"')
    expect(calendarSource).toContain('grid grid-cols-7')
    expect(calendarSource).toContain('aspect-square')
    expect(calendarSource).toContain('sm:hidden')
    expect(drawerSource).toContain('items-center justify-center')
    expect(drawerSource).toContain('max-h-[calc(100dvh-2rem)]')
    expect(drawerSource).toContain('min-[360px]:grid-cols-2')
    expect(managementSource).toContain('flex-col gap-3 min-[420px]:flex-row')
  })
})
