import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const stylesDir = resolve(currentDir, '../../../styles')
const tokens = readFileSync(resolve(stylesDir, 'workspace-tokens.css'), 'utf8')
const admin = readFileSync(resolve(stylesDir, 'admin-workspace.css'), 'utf8')

const materialTokens = ['surface', 'control', 'rule', 'divider', 'ink', 'muted', 'hover', 'shadow']

describe('admin workspace material baseline', () => {
  it('defines the shared material tokens for light and dark admin surfaces', () => {
    expect(tokens).toContain('.user-workspace,\n.admin-console-theme')
    expect(tokens).toContain('.dark .user-workspace,\n.dark.admin-console-theme')

    for (const token of materialTokens) {
      expect(tokens).toMatch(new RegExp(`--workspace-${token}:`))
      expect(tokens).toMatch(new RegExp(`--workspace-${token}:[^;]+;`, 'g'))
    }
  })

  it('applies the shared material to admin surfaces and controls', () => {
    expect(admin).toContain("@import './workspace-tokens.css';")
    expect(admin).toMatch(/\.admin-console-theme[^\n]*\{[\s\S]*?color: var\(--workspace-ink\)/)
    expect(admin).toContain('background: linear-gradient(125deg, var(--workspace-highlight)')
    expect(admin).toContain('border-color: var(--workspace-rule)')
    expect(admin).toContain('background: var(--workspace-control)')
    expect(admin).toContain('background: var(--workspace-hover)')
    expect(admin).toContain('border-color: var(--workspace-divider)')
    expect(admin).toContain('box-shadow: var(--workspace-shadow)')
  })

  it('keeps keyboard focus visible and disables admin motion when requested', () => {
    expect(admin).toMatch(/:is\(\.btn, \.input, \.select-trigger, \.date-picker-trigger\):focus-visible\s*\{[^}]*outline: 2px solid var\(--workspace-focus\)/)
    expect(admin).toContain('@media (prefers-reduced-motion: reduce)')
    expect(admin).toMatch(/prefers-reduced-motion: reduce\)[\s\S]*?transition: none;/)
    expect(admin).toMatch(/prefers-reduced-motion: reduce\)[\s\S]*?animation: none;/)
    expect(admin).toMatch(/prefers-reduced-motion: reduce\)[\s\S]*?\.select-dropdown-portal[\s\S]*?transition: none;/)
  })
})
