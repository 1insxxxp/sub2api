<template>
  <div class="support-panel min-w-0 space-y-6">
      <form class="support-lookup" :aria-busy="busy || checking" @submit.prevent="submit">
        <div class="min-w-0 flex-1">
          <label for="support-account" class="input-label">{{ t('userSupport.account') }}</label>
          <input
            id="support-account" v-model="account" data-test="support-account" type="text"
            class="input" autocomplete="off" autocapitalize="off" spellcheck="false" maxlength="254"
            :placeholder="t('userSupport.accountPlaceholder')" :disabled="busy || checking || denied"
            aria-describedby="support-trial-notice"
          />
        </div>
        <button type="submit" class="btn btn-primary min-h-11 shrink-0 gap-2" :disabled="busy || checking || denied || !account.trim()">
          <Icon :name="busy ? 'refresh' : 'search'" size="sm" :class="{ 'motion-safe:animate-spin': busy }" />
          {{ busy ? t('userSupport.querying') : t('userSupport.submit') }}
        </button>
        <p id="support-trial-notice" class="support-notice text-xs leading-5 text-gray-500 dark:text-dark-400">
          {{ t('userSupport.trialNotice') }}
        </p>
      </form>

      <p v-if="errorKey" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ t(errorKey) }}</p>
      <p v-else-if="checking" role="status" class="text-sm text-gray-500 dark:text-dark-400">{{ t('userSupport.checkingAccess') }}</p>

      <div v-if="result" data-test="support-result" class="space-y-7" aria-live="polite">
        <section class="support-summary" :aria-label="t('userSupport.summary')">
          <div class="min-w-0">
            <p class="support-label">{{ t('userSupport.account') }}</p>
            <p class="mt-2 break-all text-sm font-medium text-gray-900 dark:text-white">{{ result.user.email }}</p>
            <p class="mt-1 text-xs tabular-nums text-gray-500 dark:text-dark-400">#{{ result.user.id }}</p>
          </div>
          <div>
            <p class="support-label">{{ t('userSupport.balance') }}</p>
            <p class="mt-1.5 text-2xl font-semibold tabular-nums tracking-tight text-gray-900 dark:text-white">{{ formatAmount(result.user.balance) }}</p>
          </div>
          <div>
            <p class="support-label">{{ t('userSupport.trial.title') }}</p>
            <p class="mt-2 inline-flex items-center gap-1.5 text-sm font-medium" :class="result.trial.status === 'granted' ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-700 dark:text-dark-200'">
              <Icon :name="result.trial.status === 'granted' ? 'checkCircle' : 'check'" size="sm" />
              {{ t(result.trial.status === 'granted' ? 'userSupport.trial.granted' : 'userSupport.trial.alreadyReceived') }}
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t(result.trial.legacy ? 'userSupport.trial.historical' : 'userSupport.trial.once') }}</p>
          </div>
        </section>

        <section data-test="support-usage" class="min-w-0" :aria-labelledby="'support-usage-title'">
          <div class="support-section-heading">
            <h2 id="support-usage-title" class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('userSupport.recentUsage') }}</h2>
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('userSupport.newestFirst') }}</span>
          </div>
          <p v-if="result.usage.status !== 'ok'" class="support-empty">{{ t('userSupport.recordsUnavailable') }}</p>
          <p v-else-if="usage.length === 0" class="support-empty">{{ t('userSupport.noUsage') }}</p>
          <div v-else class="support-table-wrap">
            <table class="support-table">
              <thead><tr><th>{{ t('userSupport.columns.time') }}</th><th>{{ t('userSupport.columns.model') }}</th><th>{{ t('userSupport.columns.tokens') }}</th><th>{{ t('userSupport.columns.charge') }}</th><th>{{ t('userSupport.columns.duration') }}</th><th>{{ t('userSupport.columns.status') }}</th></tr></thead>
              <tbody>
                <tr v-for="item in usage" :key="item.id" data-test="support-usage-row">
                  <td :data-label="t('userSupport.columns.time')" class="support-time">{{ formatTime(item.created_at) }}</td>
                  <td :data-label="t('userSupport.columns.model')"><span class="break-words font-medium">{{ displayModel(item.model) }}</span><span class="mt-1 block text-xs text-gray-400 dark:text-dark-500">{{ t(item.stream ? 'userSupport.stream' : 'userSupport.sync') }}</span></td>
                  <td :data-label="t('userSupport.columns.tokens')" class="tabular-nums"><span class="text-emerald-600 dark:text-emerald-400">↓ {{ formatCount(item.input_tokens) }}</span><span class="ml-2 text-violet-600 dark:text-violet-400">↑ {{ formatCount(item.output_tokens) }}</span><span class="mt-1 block text-xs text-gray-400 dark:text-dark-500">{{ t('userSupport.cache') }} {{ formatCount(item.cache_read_tokens) }}</span></td>
                  <td :data-label="t('userSupport.columns.charge')" class="tabular-nums">{{ formatAmount(item.charged_amount) }}</td>
                  <td :data-label="t('userSupport.columns.duration')" class="tabular-nums">{{ formatDuration(item.duration_ms) }}<span class="mt-1 block text-xs text-gray-400 dark:text-dark-500">{{ t('userSupport.firstToken') }} {{ formatDuration(item.first_token_ms) }}</span></td>
                  <td :data-label="t('userSupport.columns.status')"><span class="support-status" :class="statusClass(item.status)">{{ t(`userSupport.status.${safeStatus(item.status)}`) }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section data-test="support-errors" class="min-w-0" aria-labelledby="support-errors-title">
          <div class="support-section-heading">
            <h2 id="support-errors-title" class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('userSupport.recentErrors') }}</h2>
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('userSupport.newestFirst') }}</span>
          </div>
          <p v-if="result.errors.status !== 'ok'" class="support-empty">{{ t('userSupport.recordsUnavailable') }}</p>
          <p v-else-if="errors.length === 0" class="support-empty">{{ t('userSupport.noErrors') }}</p>
          <div v-else class="support-table-wrap">
            <table class="support-table support-error-table">
              <thead><tr><th>{{ t('userSupport.columns.time') }}</th><th>{{ t('userSupport.columns.model') }}</th><th>{{ t('userSupport.columns.error') }}</th></tr></thead>
              <tbody>
                <tr v-for="item in errors" :key="item.id" data-test="support-error-row">
                  <td :data-label="t('userSupport.columns.time')" class="support-time">{{ formatTime(item.created_at) }}</td>
                  <td :data-label="t('userSupport.columns.model')" class="font-medium break-words">{{ displayModel(item.model) }}</td>
                  <td :data-label="t('userSupport.columns.error')" class="text-amber-700 dark:text-amber-400">{{ t(`userSupport.categories.${safeCategory(item.category)}`) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
      <div v-else-if="!checking && !errorKey" class="support-initial">
        <Icon name="search" size="xl" class="text-gray-300 dark:text-dark-600" />
        <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('userSupport.initial') }}</p>
      </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { useUserSupportAccess } from '@/composables/useUserSupportAccess'
import { lookup, type UserSupportResult } from '@/api/admin/userSupport'

const { t, locale } = useI18n()
const emit = defineEmits<{ 'access-denied': [] }>()
const auth = useAuthStore()
const access = useUserSupportAccess(auth)
const account = ref('')
const checking = ref(true)
const denied = ref(false)
const busy = ref(false)
const errorKey = ref('')
const result = ref<UserSupportResult | null>(null)
const usage = computed(() => result.value?.usage.items.slice(0, 10) ?? [])
const errors = computed(() => result.value?.errors.items.slice(0, 10) ?? [])
let version = 0
let accessVersion = 0

function clearResult() { version++; result.value = null; errorKey.value = '' }
async function checkAccess() {
  const requestVersion = ++accessVersion
  checking.value = true
  denied.value = true
  errorKey.value = ''
  const allowed = await access.refresh()
  if (requestVersion !== accessVersion) return
  checking.value = false
  denied.value = !allowed
  errorKey.value = allowed ? '' : 'userSupport.errors.forbidden'
  if (!allowed) emit('access-denied')
}
watch(account, clearResult)
watch(() => `${auth.user?.id}:${auth.user?.role}:${auth.token ?? ''}`, () => {
  clearResult()
  void checkAccess()
})
watch(access.canAccess, enabled => {
  if (!enabled && !checking.value && !denied.value) {
    clearResult(); denied.value = true; errorKey.value = 'userSupport.errors.forbidden'
    emit('access-denied')
  }
})

onMounted(checkAccess)
onBeforeUnmount(() => { version++; accessVersion++ })

async function submit() {
  if (busy.value || checking.value || denied.value) return
  clearResult()
  const value = account.value.trim()
  if (!value || value.length > 254) { errorKey.value = 'userSupport.errors.invalidAccount'; return }
  const requestVersion = version
  busy.value = true
  try {
    const response = await lookup(value)
    if (version === requestVersion) result.value = response
  } catch (error: unknown) {
    if (version !== requestVersion) return
    const info = error as { status?: number; code?: string }
    if (info?.status === 401 || info?.status === 403 || info?.code === 'FORBIDDEN') {
      access.revoke(); denied.value = true; errorKey.value = 'userSupport.errors.forbidden'
      emit('access-denied')
    } else if (info?.status === 404 || info?.code === 'USER_NOT_FOUND') {
      errorKey.value = 'userSupport.errors.userNotFound'
    } else if (info?.status === 400 || info?.code === 'INVALID_ACCOUNT') {
      errorKey.value = 'userSupport.errors.invalidAccount'
    } else {
      errorKey.value = 'userSupport.errors.unavailable'
    }
  } finally { busy.value = false }
}

function formatTime(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value === 'zh' ? 'zh-CN' : 'en-US', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date)
}
function formatCount(value: number | null) { return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString() : '—' }
function formatAmount(value: number) { return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 }) : '—' }
function formatDuration(value: number | null) { return typeof value === 'number' && Number.isFinite(value) ? `${(value / 1000).toFixed(2)}s` : '—' }
function displayModel(model: string) { return model && model !== 'unknown' ? model : t('userSupport.unknownModel') }
const statuses = new Set(['completed', 'interrupted', 'failed', 'empty', 'unknown'])
const categories = new Set(['auth', 'quota', 'rate_limit', 'invalid_request', 'timeout', 'service_unavailable', 'other'])
function safeStatus(value: string) { return statuses.has(value) ? value : 'unknown' }
function safeCategory(value: string) { return categories.has(value) ? value : 'other' }
function statusClass(value: string) {
  if (value === 'completed') return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
  if (value === 'failed') return 'bg-red-500/10 text-red-700 dark:text-red-400'
  return 'bg-gray-500/10 text-gray-600 dark:text-dark-300'
}
</script>

<style scoped>
.support-lookup { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; max-width: 850px; }
.support-lookup > div { min-width: min(100%, 260px); }
.support-notice { flex-basis: 100%; margin-top: -4px; }
.support-summary { display: grid; grid-template-columns: minmax(0, 1.4fr) 1fr 1.2fr; gap: 24px; padding: 22px; border: 1px solid var(--workspace-rule); border-radius: 8px; background: var(--workspace-surface); box-shadow: inset 0 1px 0 var(--workspace-highlight), 0 3px 12px rgb(15 23 42 / 0.025); }
.support-label { font-size: 12px; color: var(--workspace-muted); }
.support-section-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; padding-bottom: 12px; }
.support-table-wrap { min-width: 0; border-block: 1px solid var(--workspace-rule); }
.support-table { width: 100%; border-collapse: collapse; text-align: left; font-size: 12px; color: var(--workspace-ink); }
.support-table th { padding: 11px 12px; font-size: 11px; font-weight: 500; color: var(--workspace-muted); background: var(--workspace-hover); }
.support-table td { padding: 14px 12px; vertical-align: middle; border-top: 1px solid var(--workspace-rule); }
.support-table th:first-child, .support-table td:first-child { padding-left: 0; }
.support-time { white-space: nowrap; font-variant-numeric: tabular-nums; color: var(--workspace-muted); }
.support-status { display: inline-flex; padding: 3px 7px; border-radius: 4px; font-size: 11px; white-space: nowrap; }
.support-empty { padding: 28px 0; border-top: 1px solid var(--workspace-rule); font-size: 13px; color: var(--workspace-muted); }
.support-initial { display: flex; min-height: 230px; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
.support-error-table th:first-child { width: 25%; }
.support-error-table th:nth-child(2) { width: 35%; }
@media (max-width: 767px) {
  .support-lookup > div { flex-basis: 100%; }
  .support-lookup > button { width: 100%; }
  .support-summary { grid-template-columns: 1fr 1fr; gap: 20px 14px; padding: 18px; }
  .support-summary > div:first-child { grid-column: 1 / -1; }
  .support-table, .support-table tbody { display: block; }
  .support-table thead { display: none; }
  .support-table tr { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 16px; padding: 16px 0; border-bottom: 1px solid var(--workspace-rule); }
  .support-table tr:last-child { border-bottom: 0; }
  .support-table td, .support-table td:first-child { display: block; min-width: 0; border: 0; padding: 0; }
  .support-table td::before { content: attr(data-label); display: block; margin-bottom: 5px; font-size: 10px; font-weight: 400; color: var(--workspace-muted); }
  .support-table td:nth-child(2) { overflow-wrap: anywhere; }
  .support-error-table tr { grid-template-columns: 1fr 1fr; }
  .support-error-table td:last-child { grid-column: 1 / -1; }
}
</style>
