import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AdminWorkbenchView.vue')
const viewSource = readFileSync(viewPath, 'utf8')
const transferPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/redeem/BalanceTransferPanel.vue')
const transferSource = readFileSync(transferPath, 'utf8')
const panelPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionPanel.vue')
const panelSource = readFileSync(panelPath, 'utf8')
const calendarPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionCalendar.vue')
const calendarSource = readFileSync(calendarPath, 'utf8')
const trendChartPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionTrendChart.vue')
const trendChartSource = readFileSync(trendChartPath, 'utf8')
const drawerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionDayDrawer.vue')
const drawerSource = readFileSync(drawerPath, 'utf8')
const managementPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/admin/workbench/SubAdminCommissionManagement.vue')
const managementSource = readFileSync(managementPath, 'utf8')

describe('AdminWorkbenchView mobile layout', () => {
  it('keeps the workbench page and generated-code panels width-safe on small screens', () => {
    expect(viewSource).toContain('admin-workbench-page')
    expect(transferSource).toContain('admin-workbench-generated-now')
    expect(transferSource).toContain('admin-workbench-generated-results')
    expect(viewSource).toContain('grid-cols-3')
    expect(viewSource).toContain('sm:flex')
    expect(viewSource).not.toContain('min-h-16')
    expect(viewSource).toContain('admin-workbench-console')
    expect(viewSource).toContain('workbench-navigation')
    expect(viewSource).toContain('min-h-11')
    expect(transferSource).toContain('--workspace-surface')
    expect(viewSource).toContain('prefers-reduced-motion: reduce')
    expect(transferSource).toContain('flex-col')
    expect(transferSource).toContain('max-h-72')
    expect(transferSource).toContain('min-[360px]:flex-row')
    expect(transferSource).toContain('grid w-full grid-cols-2')
    expect(transferSource).toContain('workbench-history-actions')
    expect(transferSource).toMatch(/\.workbench-history-actions \.btn\s*\{[^}]*width: 2\.75rem;/)
    expect(transferSource).not.toContain('class="grid w-full shrink-0 grid-cols-2 gap-2 sm:flex sm:w-auto"')
  })

  it('keeps the commission calendar and its detail drawer inside the page width', () => {
    expect(panelSource).toContain('class="commission-calendar-layout grid min-w-0 items-start gap-4"')
    expect(calendarSource).toContain('grid grid-cols-7')
    expect(calendarSource).toContain('aspect-square')
    expect(calendarSource).toContain('sm:hidden')
    expect(drawerSource).toContain('items-center justify-center')
    expect(drawerSource).toContain('max-h-[calc(100dvh-2rem)]')
    expect(drawerSource).toContain('min-[360px]:grid-cols-2')
    expect(managementSource).toContain('workbench-commission-controls')
    expect(managementSource).toContain('minmax(0, 1fr) auto')
  })

  it('mounts the daily earnings chart with width-safe responsive sizing', () => {
    expect(calendarSource).toContain("import SubAdminCommissionTrendChart from './SubAdminCommissionTrendChart.vue'")
    expect(calendarSource).toContain('<SubAdminCommissionTrendChart')
    expect(calendarSource).toContain(':days="days"')
    expect(calendarSource).toContain(':loading="loading"')
    expect(trendChartSource).toContain('data-test="commission-daily-chart"')
    expect(trendChartSource).toContain('class="commission-daily-chart min-w-0 w-full h-48 overflow-hidden sm:h-64"')
    expect(trendChartSource).toContain('class="relative block h-full w-full min-w-0"')
    expect(trendChartSource).toContain('max-w-full')
    expect(trendChartSource).toContain('.commission-daily-chart :deep(canvas)')
    expect(trendChartSource).toContain('width: 100% !important')
  })
})
