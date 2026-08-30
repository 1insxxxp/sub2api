import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const createSource = readFileSync(resolve(currentDir, '../UserCreateModal.vue'), 'utf8')
const editSource = readFileSync(resolve(currentDir, '../UserEditModal.vue'), 'utf8')

describe('admin user role options', () => {
  it('offers the secondary admin role in create and edit forms', () => {
    expect(createSource).toContain('<option value="sub_admin">{{ t(\'admin.users.roles.sub_admin\') }}</option>')
    expect(editSource).toContain("{ value: 'sub_admin', label: t('admin.users.roles.sub_admin') }")
  })
})
