<template>
  <section class="admin-form-section" data-test="support-access-control">
    <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userSupport.permission.title') }}</h3>
    <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('userSupport.permission.description') }}</p>
    <div class="mt-3 grid gap-2 sm:grid-cols-3">
      <label v-for="item in permissionOptions" :key="item.key" class="flex min-h-11 items-center gap-2 rounded-lg border border-gray-200 bg-white/70 p-3 text-xs dark:border-dark-700 dark:bg-dark-900/40">
        <input v-model="permissions[item.key]" :data-test="`workbench-access-${item.key}`" type="checkbox" :disabled="loading || saving || !loaded" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
        <span>{{ t(item.label) }}</span>
      </label>
    </div>
    <div class="mt-2 flex flex-wrap items-center justify-end gap-3">
      <span v-if="message" :role="failed ? 'alert' : 'status'" class="text-xs" :class="failed ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'">{{ t(message) }}</span>
      <button v-if="!loaded && !loading" type="button" class="btn btn-secondary min-h-10 text-xs" @click="load">{{ t('userSupport.permission.retry') }}</button>
      <button v-if="loaded" data-test="support-access-save" type="button" class="btn btn-secondary min-h-10 text-xs" :disabled="loading || saving || !changed" @click="save">
        {{ t(saving ? 'userSupport.permission.saving' : 'userSupport.permission.save') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUserWorkbenchAccess, setUserWorkbenchAccess, type WorkbenchPermissions } from '@/api/admin/userSupport'

const props = defineProps<{ userId: number }>()
const { t } = useI18n()
const emptyPermissions = (): WorkbenchPermissions => ({ commission: false, affiliate_leaderboard: false, user_support: false })
const permissions = ref(emptyPermissions())
const saved = ref(emptyPermissions())
const permissionOptions = [
  { key: 'commission', label: 'userSupport.permission.commission' },
  { key: 'affiliate_leaderboard', label: 'userSupport.permission.affiliateLeaderboard' },
  { key: 'user_support', label: 'userSupport.permission.userSupport' },
] as const
const changed = computed(() => permissionOptions.some(item => permissions.value[item.key] !== saved.value[item.key]))
function safePermissions(value: WorkbenchPermissions): WorkbenchPermissions {
  return { commission: value.commission === true, affiliate_leaderboard: value.affiliate_leaderboard === true, user_support: value.user_support === true }
}
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const message = ref('')
const failed = ref(false)
let version = 0

async function load() {
  const request = ++version
  loaded.value = false; permissions.value = emptyPermissions(); message.value = ''; loading.value = true
  try {
    const result = await getUserWorkbenchAccess(props.userId)
    if (request !== version) return
    permissions.value = safePermissions(result); saved.value = { ...permissions.value }
    loaded.value = true
  } catch {
    if (request !== version) return
    failed.value = true; message.value = 'userSupport.permission.loadFailed'
  } finally { if (request === version) loading.value = false }
}
async function save() {
  if (!loaded.value || saving.value || loading.value) return
  const request = version
  const userID = props.userId
  saving.value = true; message.value = ''
  try {
    const result = await setUserWorkbenchAccess(userID, { ...permissions.value })
    if (request !== version) return
    permissions.value = safePermissions(result); saved.value = { ...permissions.value }
    failed.value = false; message.value = 'userSupport.permission.saved'
  } catch {
    if (request !== version) return
    failed.value = true; message.value = 'userSupport.permission.saveFailed'
  } finally { if (request === version) saving.value = false }
}
watch(() => props.userId, () => { saving.value = false; void load() }, { immediate: true })
onBeforeUnmount(() => { version++ })
</script>
