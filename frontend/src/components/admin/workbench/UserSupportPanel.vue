<template>
  <div class="support-panel min-w-0">
      <form ref="lookupRoot" class="support-lookup" :aria-busy="busy || checking" @submit.prevent="submit">
        <div class="support-account-field min-w-0 flex-1">
          <label for="support-account" class="input-label">{{ t('userSupport.account') }}</label>
          <input
            id="support-account" v-model="account" data-test="support-account" type="text"
            class="input" autocomplete="off" autocapitalize="off" spellcheck="false" maxlength="254"
            :placeholder="t('userSupport.accountPlaceholder')" :disabled="checking || denied"
            role="combobox" aria-autocomplete="list" :aria-controls="suggestionsOpen ? 'support-suggestions' : undefined"
            :aria-expanded="suggestionsOpen" :aria-activedescendant="activeSuggestion >= 0 && suggestionsOpen ? `support-suggestion-${activeSuggestion}` : undefined"
            aria-describedby="support-trial-notice" @keydown="onAccountKeydown" @focus="scheduleSearch" @blur="dismissSuggestions"
          />
          <div v-if="suggestionsOpen" class="support-suggestions">
            <ul id="support-suggestions" role="listbox" :aria-label="t('userSupport.suggestions')">
              <li v-for="(item, index) in suggestions" :id="`support-suggestion-${index}`" :key="item.id" role="option"
                class="support-suggestion" :aria-selected="activeSuggestion === index" @pointerdown.prevent @click="selectSuggestion(item)">
                <span class="min-w-0 [overflow-wrap:anywhere]">{{ item.email }}</span><span class="support-label shrink-0 tabular-nums">#{{ item.id }}</span>
              </li>
            </ul>
            <p v-if="!suggestions.length" role="status" class="support-search-status">{{ t(searching ? 'userSupport.searching' : 'userSupport.noMatches') }}</p>
          </div>
          <p v-if="searchError" role="status" class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t(searchError) }}</p>
        </div>
        <button type="submit" class="btn btn-primary min-h-11 shrink-0 gap-2" :disabled="busy || checking || denied || !account.trim()" @pointerdown.prevent>
          <Icon :name="busy ? 'refresh' : 'search'" size="sm" :class="{ 'motion-safe:animate-spin': busy }" />
          {{ busy ? t('userSupport.querying') : t('userSupport.submit') }}
        </button>
        <p id="support-trial-notice" class="support-notice text-xs leading-5 text-gray-500 dark:text-dark-400">
          {{ t('userSupport.trialNotice') }}
        </p>
      </form>

      <p v-if="errorKey" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ t(errorKey) }}</p>
      <p v-else-if="checking" role="status" class="text-sm text-gray-500 dark:text-dark-400">{{ t('userSupport.checkingAccess') }}</p>

      <div v-if="result" data-test="support-result" class="support-results" aria-live="polite">
        <section class="support-summary" :aria-label="t('userSupport.summary')">
          <div class="support-summary-overview">
            <div class="support-summary-identity">
              <div class="support-summary-heading"><p class="support-label">{{ t('userSupport.account') }}</p><span class="support-user-id">#{{ result.user.id }}</span></div>
              <p class="support-summary-account">{{ result.user.email }}</p>
            </div>
            <div class="support-summary-balance">
              <p class="support-label">{{ t('userSupport.balance') }}</p>
              <p class="support-balance">{{ formatAmount(result.user.balance) }}</p>
            </div>
          </div>
          <div class="support-summary-trial" :data-state="result.trial.status">
            <p class="support-trial-label">
              <Icon name="gift" size="sm" aria-hidden="true" />
              <span>{{ t('userSupport.trial.label') }}</span>
              <strong>{{ formatCount(result.trial.amount) }}</strong>
            </p>
            <div class="support-trial-controls">
              <div class="support-trial-outcome" role="group" :aria-label="trialDescription" :title="trialDescription">
                <span class="support-trial-state" :class="{ 'is-granted': result.trial.status === 'granted' }">
                  <Icon v-if="result.trial.status !== 'eligible'" name="check" size="xs" aria-hidden="true" />
                  <span>{{ t(result.trial.status === 'eligible' ? 'userSupport.trial.eligibleShort' : result.trial.status === 'granted' ? 'userSupport.trial.grantedShort' : 'userSupport.trial.receivedShort') }}</span>
                </span>
                <span v-if="result.trial.legacy" class="support-trial-source">{{ t('userSupport.trial.historicalShort') }}</span>
              </div>
              <button v-if="result.trial.status === 'eligible'" type="button" data-test="support-grant" class="btn btn-primary support-grant"
                :disabled="granting || busy || checking || denied" :aria-busy="granting" aria-describedby="support-trial-notice" @click="sendTrial">
                <Icon v-if="granting" name="refresh" size="sm" class="motion-safe:animate-spin" />{{ t(granting ? 'userSupport.trial.sending' : 'userSupport.trial.send') }}
              </button>
            </div>
          </div>
        </section>

        <nav ref="recordTabs" role="tablist" class="support-record-tabs" :aria-label="t('userSupport.records')">
          <button v-for="tab in recordTabNames" :id="`support-tab-${tab}`" :key="tab" type="button" role="tab" :data-test="`support-tab-${tab}`"
            :aria-controls="`support-${tab}-panel`" :aria-selected="activeTab === tab" :tabindex="activeTab === tab ? 0 : -1"
            @click="activeTab = tab" @keydown="onRecordTabKeydown($event, tab)">
            {{ t(tab === 'usage' ? 'userSupport.usageTab' : 'userSupport.errorsTab') }}
            <span class="support-tab-count" aria-hidden="true">{{ tab === 'usage' ? usage.length : errors.length }}</span>
          </button>
        </nav>
        <section v-show="activeTab === 'usage'" id="support-usage-panel" data-test="support-usage" class="min-w-0 support-record-panel" role="tabpanel" aria-labelledby="support-tab-usage" tabindex="0">
          <div class="support-section-heading">
            <h2 id="support-usage-title">{{ t('userSupport.recentUsage') }}</h2>
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('userSupport.newestFirst') }}</span>
          </div>
          <p v-if="result.usage.status !== 'ok'" class="support-empty">{{ t('userSupport.recordsUnavailable') }}</p>
          <p v-else-if="usage.length === 0" class="support-empty">{{ t('userSupport.noUsage') }}</p>
          <ol v-else class="support-record-list support-usage-list">
            <li v-for="item in usage" :key="item.id" data-test="support-usage-row" class="support-record-card">
              <header class="support-record-header">
                <div class="support-record-title"><h3>{{ displayModel(item.model) }}</h3><span class="support-protocol">{{ t(item.stream ? 'userSupport.stream' : 'userSupport.sync') }}</span></div>
                <span class="support-status" :class="statusClass(item.status)">{{ t(`userSupport.status.${safeStatus(item.status)}`) }}</span>
              </header>
              <div class="support-record-meta">
                <time class="support-time" :datetime="item.created_at">{{ formatTime(item.created_at, true) }}</time>
                <span class="support-record-group"><span class="support-meta-label">{{ t('userSupport.columns.group') }}</span>{{ item.group_name || '—' }}</span>
              </div>
              <dl class="support-token-line">
                <div data-field="tokens"><dt>{{ t('userSupport.columns.tokens') }}</dt><dd><span>↓ {{ formatCount(item.input_tokens) }}</span><span>↑ {{ formatCount(item.output_tokens) }}</span></dd></div>
                <div data-field="cache"><dt>{{ t('userSupport.cache') }}</dt><dd>{{ formatCount(item.cache_read_tokens) }}</dd></div>
              </dl>
              <dl class="support-record-footer">
                <div data-field="charge"><dt>{{ t('userSupport.columns.charge') }}</dt><dd>{{ formatAmount(item.charged_amount) }}</dd></div>
                <div data-field="duration"><dt>{{ t('userSupport.columns.duration') }}</dt><dd>{{ formatDuration(item.duration_ms) }}</dd></div>
                <div data-field="first-token"><dt>{{ t('userSupport.firstToken') }}</dt><dd>{{ formatDuration(item.first_token_ms) }}</dd></div>
              </dl>
            </li>
          </ol>
        </section>

        <section v-show="activeTab === 'errors'" id="support-errors-panel" data-test="support-errors" class="min-w-0 support-record-panel" role="tabpanel" aria-labelledby="support-tab-errors" tabindex="0">
          <div class="support-section-heading">
            <h2>{{ t('userSupport.recentErrors') }}</h2>
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('userSupport.newestFirst') }}</span>
          </div>
          <p v-if="result.errors.status !== 'ok'" class="support-empty">{{ t('userSupport.recordsUnavailable') }}</p>
          <p v-else-if="errors.length === 0" class="support-empty">{{ t('userSupport.noErrors') }}</p>
          <ol v-else class="support-record-list support-error-list">
            <li v-for="item in errors" :key="item.id" data-test="support-error-row" class="support-record-card support-error-record">
              <header class="support-record-header">
                <h3>{{ displayModel(item.model) }}</h3>
                <span data-test="support-error-status" class="support-error-status" :title="t('userSupport.httpStatusHint')">{{ errorStatus(item.status_code, item.category) }}</span>
              </header>
              <div class="support-record-meta">
                <time class="support-time" :datetime="item.created_at">{{ formatTime(item.created_at) }}</time>
                <span class="support-record-group"><span class="support-meta-label">{{ t('userSupport.columns.group') }}</span>{{ item.group_name || '—' }}</span>
                <span class="support-error-kind">{{ t(`userSupport.errorKinds.${safeErrorKind(item.error_kind)}`) }}</span>
              </div>
              <p data-test="support-error-original" class="support-error-original"><span class="support-label">{{ t('userSupport.columns.originalError') }}：</span>{{ originalError(item.original_message) }}</p>
              <p v-if="item.chineseExplanation" data-test="support-error-chinese" class="support-error-chinese" lang="zh-CN"><span class="support-label">{{ t('userSupport.columns.chineseExplanation') }}：</span>{{ item.chineseExplanation }}</p>
            </li>
          </ol>
        </section>
      </div>
      <div v-else-if="!checking && !errorKey" class="support-initial">
        <Icon name="search" size="xl" class="text-gray-300 dark:text-dark-600" />
        <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('userSupport.initial') }}</p>
      </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { translateSupportErrorToChinese } from './supportErrorTranslation'
import { useAuthStore } from '@/stores/auth'
import { useUserSupportAccess } from '@/composables/useUserSupportAccess'
import { grant, lookup, search, type SupportSearchItem, type UserSupportResult } from '@/api/admin/userSupport'

const { t } = useI18n()
const emit = defineEmits<{ 'access-denied': [] }>()
const auth = useAuthStore()
const access = useUserSupportAccess(auth)
const account = ref('')
const selectedAccount = ref<SupportSearchItem | null>(null)
const checking = ref(true)
const denied = ref(false)
const busy = ref(false)
const pendingGrants = ref(new Set<string>())
const settledGrants = new Map<string, number>()
let grantSequence = 0
const errorKey = ref('')
const result = ref<UserSupportResult | null>(null)
const trialDescription = computed(() => {
  const trial = result.value?.trial
  if (!trial) return ''
  const status = trial.status === 'eligible' ? 'eligible' : trial.status === 'granted' ? 'granted' : 'alreadyReceived'
  return `${t(`userSupport.trial.${status}`)} · ${t(trial.legacy ? 'userSupport.trial.historical' : 'userSupport.trial.once')}`
})
const usage = computed(() => result.value?.usage.items.slice(0, 10) ?? [])
const errors = computed(() => result.value?.errors.items.slice(0, 10).map(item => ({ ...item, chineseExplanation: chineseError(item.original_message) })) ?? [])
const lookupRoot = ref<HTMLElement | null>(null)
const recordTabs = ref<HTMLElement | null>(null)
const recordTabNames = ['usage', 'errors'] as const
const activeTab = ref<(typeof recordTabNames)[number]>('usage')
const suggestions = ref<SupportSearchItem[]>([])
const suggestionsOpen = ref(false)
const activeSuggestion = ref(-1)
const searching = ref(false)
const searchError = ref('')
let version = 0
let accessVersion = 0
let searchVersion = 0
let searchTimer: ReturnType<typeof setTimeout> | undefined
const sessionIdentity = () => `${auth.user?.id}:${auth.user?.role}:${auth.token ?? ''}`
const granting = computed(() => result.value !== null && pendingGrants.value.has(`${sessionIdentity()}:${result.value.user.id}`))

function clearResult() {
  version++; result.value = null; errorKey.value = ''; busy.value = false; activeTab.value = 'usage'
}
function dismissSuggestions() {
  searchVersion++
  if (searchTimer !== undefined) clearTimeout(searchTimer)
  searchTimer = undefined
  suggestions.value = []; suggestionsOpen.value = false; activeSuggestion.value = -1; searching.value = false; searchError.value = ''
}
function revokeAccess(revoke = true) {
  clearResult(); dismissSuggestions()
  denied.value = true; checking.value = false; errorKey.value = 'userSupport.errors.forbidden'
  if (revoke) access.revoke()
  emit('access-denied')
}
async function checkAccess() {
  const requestVersion = ++accessVersion
  checking.value = true; denied.value = true; errorKey.value = ''
  const allowed = await access.refresh()
  if (requestVersion !== accessVersion) return
  checking.value = false; denied.value = !allowed
  if (!allowed) revokeAccess(false)
}
function forbidden(error: unknown) {
  const info = error as { status?: number; code?: string }
  return info?.status === 401 || info?.status === 403 || info?.code === 'FORBIDDEN'
}
function current(requestVersion: number, actor: string) {
  return requestVersion === version && actor === sessionIdentity() && !checking.value && !denied.value
}
function scheduleSearch() {
  dismissSuggestions()
  const query = account.value.trim()
  if (checking.value || denied.value || query.length > 254 || !(/^[0-9]+$/.test(query) || query.length >= 2)) return
  const requestVersion = searchVersion
  searchTimer = setTimeout(() => { searchTimer = undefined; void fetchSuggestions(query, requestVersion) }, 250)
}
async function fetchSuggestions(query: string, requestVersion: number) {
  const actor = sessionIdentity()
  searching.value = true; suggestionsOpen.value = true
  try {
    const response = await search(query)
    if (requestVersion !== searchVersion || actor !== sessionIdentity() || checking.value || denied.value) return
    suggestions.value = response.items.filter(item => Number.isSafeInteger(item.id) && item.id > 0 && typeof item.email === 'string').slice(0, 8)
  } catch (error) {
    if (requestVersion !== searchVersion || actor !== sessionIdentity()) return
    if (forbidden(error)) revokeAccess()
    else { suggestionsOpen.value = false; searchError.value = 'userSupport.errors.searchUnavailable' }
  } finally {
    if (requestVersion === searchVersion) searching.value = false
  }
}
function selectSuggestion(item: SupportSearchItem) {
  if (checking.value || denied.value) return
  clearResult()
  account.value = item.email
  // Set this after the synchronous input watcher clears the previous selection.
  selectedAccount.value = { id: item.id, email: item.email }
  dismissSuggestions()
  void submit()
}
function onAccountKeydown(event: KeyboardEvent) {
  if (event.isComposing) return
  if (event.key === 'Escape' || event.key === 'Tab') { dismissSuggestions(); return }
  if (event.key === 'Enter' && suggestionsOpen.value && activeSuggestion.value >= 0) {
    const item = suggestions.value[activeSuggestion.value]
    if (item) { event.preventDefault(); selectSuggestion(item) }
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!suggestions.value.length) { scheduleSearch(); return }
    suggestionsOpen.value = true
    activeSuggestion.value = activeSuggestion.value < 0
      ? event.key === 'ArrowDown' ? 0 : suggestions.value.length - 1
      : (activeSuggestion.value + (event.key === 'ArrowDown' ? 1 : -1) + suggestions.value.length) % suggestions.value.length
    void nextTick(() => {
      const option = lookupRoot.value?.querySelector<HTMLElement>(`#support-suggestion-${activeSuggestion.value}`)
      const scroller = lookupRoot.value?.querySelector<HTMLElement>('.support-suggestions')
      if (!option || !scroller) return
      if (option.offsetTop < scroller.scrollTop) scroller.scrollTop = option.offsetTop
      else if (option.offsetTop + option.offsetHeight > scroller.scrollTop + scroller.clientHeight) {
        scroller.scrollTop = option.offsetTop + option.offsetHeight - scroller.clientHeight
      }
    })
  }
}
function onOutside(event: Event) {
  if (event.target instanceof Node && !lookupRoot.value?.contains(event.target)) dismissSuggestions()
}
watch(account, () => { selectedAccount.value = null; clearResult(); scheduleSearch() }, { flush: 'sync' })
watch(sessionIdentity, () => { selectedAccount.value = null; clearResult(); dismissSuggestions(); settledGrants.clear(); void checkAccess() }, { flush: 'sync' })
watch(access.canAccess, enabled => {
  if (!enabled && !checking.value && !denied.value) revokeAccess(false)
}, { flush: 'sync' })
onMounted(() => { document.addEventListener('pointerdown', onOutside); void checkAccess() })
onBeforeUnmount(() => { version++; accessVersion++; denied.value = true; dismissSuggestions(); document.removeEventListener('pointerdown', onOutside) })

async function submit() {
  if (busy.value || checking.value || denied.value) return
  clearResult(); dismissSuggestions()
  const label = account.value.trim()
  const value = selectedAccount.value?.email.trim() === label ? String(selectedAccount.value.id) : label
  if (!value || value.length > 254) { errorKey.value = 'userSupport.errors.invalidAccount'; return }
  const requestVersion = version; const actor = sessionIdentity()
  const startedGrantSequence = grantSequence
  busy.value = true
  try {
    let response = await lookup(value)
    if (!current(requestVersion, actor)) return
    // A lookup started before a successful grant may carry a pre-grant
    // snapshot. Fetch again after the receipt instead of reviving eligibility.
    if ((settledGrants.get(`${actor}:${response.user.id}`) ?? 0) > startedGrantSequence) {
      const userID = response.user.id
      response = await lookup(String(userID))
      if (response.user.id !== userID) throw new Error('Mismatched support lookup')
    }
    if (current(requestVersion, actor)) result.value = response
  } catch (error: unknown) {
    if (!current(requestVersion, actor)) return
    const info = error as { status?: number; code?: string }
    if (forbidden(error)) revokeAccess()
    else if (info?.status === 404 || info?.code === 'USER_NOT_FOUND') errorKey.value = 'userSupport.errors.userNotFound'
    else if (info?.status === 400 || info?.code === 'INVALID_ACCOUNT') errorKey.value = 'userSupport.errors.invalidAccount'
    else errorKey.value = 'userSupport.errors.unavailable'
  } finally { if (version === requestVersion) busy.value = false }
}
async function sendTrial() {
  const customer = result.value
  if (!customer || customer.trial.status !== 'eligible' || granting.value || busy.value || checking.value || denied.value) return
  const requestVersion = version; const actor = sessionIdentity(); const userID = customer.user.id
  const pendingKey = `${actor}:${userID}`
  pendingGrants.value.add(pendingKey); errorKey.value = ''
  try {
    const response = await grant(userID)
    if (actor !== sessionIdentity() || checking.value || denied.value) return
    if (response.user.id !== userID) {
      if (current(requestVersion, actor)) errorKey.value = 'userSupport.errors.grantUnavailable'
      return
    }
    settledGrants.set(pendingKey, ++grantSequence)
    if (result.value?.user.id !== userID) return
    const refreshVersion = version
    result.value = { ...result.value, user: response.user, trial: response.trial }
    if (refreshVersion !== requestVersion) {
      try {
        const snapshot = await lookup(String(userID))
        if (!current(refreshVersion, actor) || result.value?.user.id !== userID) return
        if (snapshot.user.id !== userID) throw new Error('Mismatched support lookup')
        result.value = snapshot
      } catch (error) {
        if (!current(refreshVersion, actor)) return
        if (forbidden(error)) revokeAccess()
        else errorKey.value = 'userSupport.errors.refreshAfterGrant'
      }
    }
  } catch (error) {
    if (!current(requestVersion, actor)) return
    if (forbidden(error)) revokeAccess()
    else errorKey.value = 'userSupport.errors.grantUnavailable'
  } finally { pendingGrants.value.delete(pendingKey) }
}
function onRecordTabKeydown(event: KeyboardEvent, tab: (typeof recordTabNames)[number]) {
  const index = recordTabNames.indexOf(tab)
  let selected: (typeof recordTabNames)[number]
  if (event.key === 'ArrowRight') selected = recordTabNames[(index + 1) % recordTabNames.length]
  else if (event.key === 'ArrowLeft') selected = recordTabNames[(index + recordTabNames.length - 1) % recordTabNames.length]
  else if (event.key === 'Home') selected = 'usage'
  else if (event.key === 'End') selected = 'errors'
  else return
  event.preventDefault(); activeTab.value = selected
  void nextTick(() => recordTabs.value?.querySelector<HTMLButtonElement>(`#support-tab-${selected}`)?.focus())
}
function formatTime(value: string, seconds = false) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  const parts = Object.fromEntries(new Intl.DateTimeFormat('en-GB', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: seconds ? '2-digit' : undefined, hourCycle: 'h23' }).formatToParts(date).map(part => [part.type, part.value]))
  return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}${seconds ? `:${parts.second}` : ''}`
}
function formatCount(value: number | null) { return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString() : '—' }
function formatAmount(value: number) { return typeof value === 'number' && Number.isFinite(value) ? value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 }) : '—' }
function formatDuration(value: number | null) { return typeof value === 'number' && Number.isFinite(value) ? `${(value / 1000).toFixed(2)}s` : '—' }
function displayModel(model: string) { return model && model !== 'unknown' ? model : t('userSupport.unknownModel') }
function errorStatus(value: unknown, category: string) {
  const http = typeof value === 'number' && Number.isInteger(value) && value >= 100 && value <= 599 ? `HTTP ${value}｜` : ''
  return http + t(`userSupport.categories.${safeCategory(category)}`)
}
function originalError(value: unknown) { return typeof value === 'string' && value.trim() ? value : t('userSupport.noOriginalError') }
function chineseError(value: unknown) {
  if (typeof value !== 'string' || !/[a-z]{2}/i.test(value) || /[\u3400-\u9fff]/.test(value)) return ''
  return translateSupportErrorToChinese(value) ?? t('userSupport.noChineseTranslation')
}
const statuses = new Set(['completed', 'interrupted', 'failed', 'empty', 'unknown'])
const categories = new Set(['auth', 'quota', 'rate_limit', 'invalid_request', 'timeout', 'service_unavailable', 'other'])
const errorKinds = new Set(['request', 'auth', 'account_auth', 'routing', 'upstream', 'network', 'internal', 'unknown'])
function safeStatus(value: string) { return statuses.has(value) ? value : 'unknown' }
function safeCategory(value: string) { return categories.has(value) ? value : 'other' }
function safeErrorKind(value: unknown) { return typeof value === 'string' && errorKinds.has(value) ? value : 'unknown' }
function statusClass(value: string) {
  if (value === 'completed') return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
  if (value === 'failed') return 'bg-red-500/10 text-red-700 dark:text-red-400'
  return 'bg-gray-500/10 text-gray-600 dark:text-dark-300'
}
</script>

<style scoped>
.support-panel { container: support / inline-size; display: grid; gap: 16px; color: var(--workspace-ink); }
.support-results { display: grid; gap: 12px; min-width: 0; }
.support-lookup { display: flex; flex-wrap: wrap; gap: 8px 10px; align-items: flex-end; max-width: 850px; }
.support-lookup > div { min-width: min(100%, 260px); }
.support-account-field { position: relative; }
.support-panel .input-label { margin-bottom: 6px; font-size: 12px; }
.support-suggestions { position: absolute; z-index: 20; top: calc(100% + 5px); inset-inline: 0; max-height: min(300px, 50dvh); overflow-y: auto; overscroll-behavior: contain; border: 1px solid var(--workspace-rule); border-radius: 6px; background: linear-gradient(130deg, var(--workspace-highlight), transparent 65%), var(--workspace-surface); box-shadow: inset 0 1px 0 var(--workspace-highlight), 0 6px 20px rgb(15 23 42 / 10%); backdrop-filter: blur(12px); }
.support-suggestion { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 44px; padding: 10px 12px; cursor: pointer; font-size: 13px; color: var(--workspace-ink); }
.support-suggestion:hover, .support-suggestion[aria-selected="true"] { background: var(--workspace-hover); }
.support-search-status { padding: 12px; font-size: 12px; color: var(--workspace-muted); }
.support-notice { flex-basis: 100%; margin-top: 0; font-size: 11px; line-height: 1.5; }
.support-summary, .support-record-card {
  --support-card-rule: rgb(148 163 184 / 13%);
  min-width: 0;
  border: 1px solid rgb(148 163 184 / 24%);
  border-radius: 8px;
  background-color: rgb(255 255 255 / 96%);
  background-image: linear-gradient(125deg, rgb(255 255 255 / 82%) 15%, transparent 65%, rgb(148 163 184 / 3%));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 4px rgb(15 23 42 / 4%);
}
.dark .support-summary, .dark .support-record-card {
  --support-card-rule: rgb(255 255 255 / 7%);
  border-color: rgb(255 255 255 / 11%);
  background-color: #27292e;
  background-image: linear-gradient(125deg, rgb(255 255 255 / 4%), transparent 65%, rgb(148 163 184 / 3%));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%), 0 2px 5px rgb(0 0 0 / 12%);
}
.support-summary { --support-summary-inset: 14px; display: grid; padding: 0; }
.support-summary-overview { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px 24px; min-width: 0; padding: 12px var(--support-summary-inset); }
.support-summary-identity { min-width: 0; }
.support-summary-heading { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.support-summary-account { margin-top: 4px; font-size: 14px; line-height: 1.4; font-weight: 600; overflow-wrap: anywhere; }
.support-user-id { padding: 1px 5px; border-radius: 4px; background: var(--workspace-hover); color: var(--workspace-muted); font-size: 10px; line-height: 1.4; font-weight: 500; font-variant-numeric: tabular-nums; }
.support-summary-balance { min-width: 0; text-align: right; }
.support-balance { margin-top: 4px; font-size: 20px; font-weight: 600; line-height: 1.2; letter-spacing: -0.025em; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.support-label { font-size: 11px; line-height: 1.4; color: var(--workspace-muted); }
.support-summary-trial { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px 12px; min-width: 0; padding: 8px var(--support-summary-inset); border-top: 1px solid var(--support-card-rule); border-radius: 0 0 7px 7px; background: var(--workspace-hover); }
.support-trial-label { display: inline-flex; align-items: center; gap: 7px; min-width: max-content; color: var(--workspace-muted); font-size: 11px; line-height: 1.5; }
.support-trial-label strong { margin-left: 1px; color: var(--workspace-ink); font-size: 14px; line-height: 1.4; font-weight: 600; font-variant-numeric: tabular-nums; }
.support-trial-controls { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px 12px; min-width: 0; max-width: 100%; margin-inline-start: auto; }
.support-trial-outcome { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.support-trial-state { display: inline-flex; align-items: center; gap: 4px; padding: 2px 7px; border: 1px solid var(--workspace-rule); border-radius: 4px; background: var(--workspace-surface); color: var(--workspace-muted); font-size: 11px; line-height: 1.5; white-space: nowrap; }
.support-trial-state.is-granted { border-color: rgb(16 185 129 / 18%); background: rgb(16 185 129 / 6%); color: #047857; }
.dark .support-trial-state.is-granted { color: #6ee7b7; }
.support-trial-source { color: var(--workspace-muted); font-size: 10px; line-height: 1.5; white-space: nowrap; }
.support-panel .support-grant { display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; gap: 5px; min-height: 36px; padding: 7px 12px; font-size: 12px; white-space: nowrap; }
.support-record-tabs { display: flex; align-items: center; gap: 4px; padding-bottom: 6px; border-bottom: 1px solid var(--workspace-rule); }
.support-record-tabs button { display: inline-flex; align-items: center; gap: 7px; min-height: 34px; padding: 6px 10px; border: 1px solid transparent; border-radius: 6px; font-size: 12px; font-weight: 500; color: var(--workspace-muted); transition: color 120ms ease, background-color 120ms ease; }
.support-record-tabs button[aria-selected="true"] { border-color: var(--workspace-rule); background: var(--workspace-surface); color: rgb(var(--brand-rgb)); box-shadow: inset 0 1px 0 var(--workspace-highlight), 0 1px 2px rgb(15 23 42 / 3%); }
.dark .support-record-tabs button[aria-selected="true"] { color: #93b8ef; }
.support-tab-count { font-size: 10px; font-weight: 400; opacity: 0.7; font-variant-numeric: tabular-nums; }
.support-record-tabs button:focus-visible, .support-record-panel:focus-visible { outline: 2px solid var(--workspace-focus); outline-offset: 3px; border-radius: 6px; }
.support-record-panel { min-width: 0; }
.support-section-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 4px 12px; margin-bottom: 8px; color: var(--workspace-muted); }
.support-section-heading h2, .support-section-heading > span { font-size: 11px; line-height: 1.5; font-weight: 400; }
.support-record-list { display: grid; align-items: start; gap: 8px; min-width: 0; margin: 0; padding: 0; list-style: none; }
.support-record-list > :only-child { grid-column: 1 / -1; }
.support-record-card { padding: 11px 12px; }
.support-record-header { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 4px 10px; }
.support-record-title { display: flex; flex: 1; align-items: baseline; flex-wrap: wrap; gap: 4px 7px; min-width: 0; }
.support-record-header h3 { min-width: 0; font-size: 14px; line-height: 1.4; font-weight: 600; overflow-wrap: anywhere; }
.support-protocol { color: var(--workspace-muted); font-size: 10px; line-height: 1.4; }
.support-status { display: inline-flex; align-items: center; gap: 5px; padding: 0; border-radius: 0; background-color: transparent; font-size: 10px; line-height: 1.5; }
.support-status::before { content: ''; width: 4px; height: 4px; flex-shrink: 0; border-radius: 50%; background: currentColor; opacity: 0.8; }
.support-record-meta { display: flex; flex-wrap: wrap; align-items: baseline; gap: 3px 10px; margin-top: 5px; color: var(--workspace-muted); font-size: 11px; line-height: 1.5; }
.support-time { font-variant-numeric: tabular-nums; }
.support-record-group { display: inline; min-width: 0; overflow-wrap: anywhere; }
.support-meta-label { margin-right: 5px; color: var(--workspace-muted); font-size: 10px; font-weight: 400; }
.support-token-line, .support-record-footer { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 16px; font-size: 12px; line-height: 1.5; }
.support-token-line { margin-top: 7px; }
.support-token-line > div, .support-record-footer > div { display: inline-flex; flex-wrap: wrap; align-items: baseline; gap: 3px 6px; min-width: 0; }
.support-token-line dt, .support-record-footer dt { color: var(--workspace-muted); font-size: 11px; }
.support-token-line dd, .support-record-footer dd { color: var(--workspace-ink); font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.support-token-line [data-field="tokens"] dd { display: inline-flex; flex-wrap: wrap; gap: 3px 10px; font-weight: 500; }
.support-record-footer { margin-top: 7px; padding-top: 6px; border-top: 1px solid var(--support-card-rule); }
.support-error-status { max-width: 100%; color: #9a4b10; font-size: 11px; line-height: 1.5; overflow-wrap: anywhere; }
.dark .support-error-status { color: #e6b786; }
.support-error-kind { color: var(--workspace-muted); font-size: 10px; }
.support-error-original { margin-top: 7px; padding-top: 7px; border-top: 1px solid var(--support-card-rule); color: var(--workspace-ink); font-size: 12px; line-height: 1.55; white-space: pre-wrap; overflow-wrap: anywhere; }
.support-error-original .support-label, .support-error-chinese .support-label { font-size: 10px; }
.support-error-chinese { margin-top: 4px; color: var(--workspace-ink); font-size: 12px; line-height: 1.55; overflow-wrap: anywhere; }
.support-empty { padding: 20px 0; font-size: 12px; color: var(--workspace-muted); }
.support-initial { display: flex; min-height: 150px; flex-direction: column; align-items: center; justify-content: center; text-align: center; }
@container support (min-width: 760px) { .support-record-list { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@container support (max-width: 320px) {
  .support-summary-overview { grid-template-columns: minmax(0, 1fr); gap: 10px; }
  .support-summary-balance { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; text-align: left; }
  .support-balance { margin-top: 0; }
}
@container support (max-width: 420px) {
  .support-summary-trial[data-state="eligible"] { display: grid; grid-template-columns: minmax(0, 1fr) auto; row-gap: 4px; }
  .support-summary-trial[data-state="eligible"] .support-trial-label { grid-column: 1; grid-row: 1; }
  .support-summary-trial[data-state="eligible"] .support-trial-controls { display: contents; }
  .support-summary-trial[data-state="eligible"] .support-trial-outcome { grid-column: 1; grid-row: 2; }
  .support-summary-trial[data-state="eligible"] .support-grant { grid-column: 2; grid-row: 1 / 3; }
}
@media (max-width: 767px) {
  .support-panel .support-lookup > div { flex-basis: 100%; min-width: 0; }
  .support-panel .support-lookup .input { min-height: 44px; font-size: 16px; }
  .support-panel .support-lookup > button { width: 100%; min-height: 44px; white-space: normal; }
  .support-summary { --support-summary-inset: 12px; }
  .support-panel .support-grant { min-height: 44px; padding: 6px 9px; }
  .support-record-tabs button { min-height: 44px; }
}
@media (max-width: 359px) {
  .support-record-meta { column-gap: 8px; }
  .support-token-line, .support-record-footer { column-gap: 12px; }
}
@media (prefers-reduced-motion: reduce) { .support-record-tabs button { transition: none; } }
</style>
