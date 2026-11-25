<template>
  <section class="admin-surface checkins-table-card rounded-2xl">
    <h3 class="admin-panel-header">
      <button type="button" data-test="risk-open" :aria-expanded="open" class="flex w-full items-center gap-2 text-left text-base font-semibold" @click="toggle">
        <Icon name="shield" size="sm" />{{ tr('title') }}
        <Icon :name="open ? 'chevronUp' : 'chevronDown'" size="sm" class="ml-auto" />
      </button>
    </h3>
    <div v-if="open" class="space-y-4 p-4 sm:p-5">
      <div class="flex flex-wrap gap-2" role="group" :aria-label="tr('title')">
        <button v-for="item in tabs" :key="item" type="button" :data-test="'risk-' + item"
                :aria-pressed="tab === item" class="btn btn-sm" :class="tab === item ? 'btn-primary' : 'btn-secondary'"
                @click="selectTab(item)">{{ tr(item) }}</button>
      </div>
      <CheckinRiskSettings v-if="tab === 'settings'" />
      <template v-else>
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="applyFilters">
          <label class="min-w-40 flex-1">
            <span class="input-label">{{ tr('email') }}</span>
            <input v-model.trim="draft.search" type="search" class="input" />
          </label>
          <label v-if="tab !== 'whitelist'">
            <span class="input-label">{{ tr('status') }}</span>
            <select v-model="draft.status" class="input">
              <option value="">{{ tr('all') }}</option>
              <option v-for="s in availableStatuses" :key="s" :value="s">{{ tr('statuses.' + s) }}</option>
            </select>
          </label>
          <label v-if="tab === 'reviews'">
            <span class="input-label">{{ tr('checkin_date') }}</span>
            <input v-model="draft.date" class="input" type="date" />
          </label>
          <template v-if="tab === 'associations'">
            <label>
              <span class="input-label">{{ tr('signal') }}</span>
              <select v-model="draft.reason" class="input">
                <option value="">{{ tr('all') }}</option>
                <option v-for="signal in riskReasons" :key="signal" :value="signal">{{ tr('signals.' + signal) }}</option>
              </select>
            </label>
            <label>
              <span class="input-label">{{ tr('start') }}</span>
              <input v-model="draft.start_date" class="input" type="date" :max="draft.end_date || undefined" />
            </label>
            <label>
              <span class="input-label">{{ tr('end') }}</span>
              <input v-model="draft.end_date" class="input" type="date" :min="draft.start_date || undefined" />
            </label>
          </template>
          <button type="submit" class="btn btn-secondary">{{ t('common.search') }}</button>
          <button v-if="tab === 'whitelist'" type="button" class="btn btn-primary" @click="editWhitelist(true)">{{ tr('addWhitelist') }}</button>
        </form>
        <div v-if="loadError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ loadError }}</div>
        <DataTable v-else :columns="columns" :data="items" :loading="loading" :row-key="rowKey"
                   :mobile-layout="{ title: tab === 'whitelist' ? 'email' : 'user_email', subtitle: tab === 'associations' ? 'related_email' : 'owner_email', status: 'status', summary: ['signals', 'checkin_date', 'reason'] }">
          <template #cell-status="{ value }">{{ tr('statuses.' + value) }}</template>
          <template #cell-created_at="{ value }">{{ formatDateTime(value) || value }}</template>
          <template #cell-signals="{ row }">
            <strong v-if="row.high_risk" class="block text-amber-700 dark:text-amber-300">{{ tr('highRisk') }}</strong>
            <span>{{ signalNames(row.reasons) }}</span>
            <details v-if="row.evidence?.length" class="mt-1 text-xs">
              <summary class="cursor-pointer">{{ tr('evidence') }}</summary>
              <dl class="space-y-1 pt-1">
                <div v-for="entry in row.evidence" :key="entry.reason">
                  <dt>{{ tr('signals.' + entry.reason) }}</dt>
                  <dd>{{ formatDateTime(entry.first_seen_at) }} – {{ formatDateTime(entry.last_seen_at) }}</dd>
                </div>
                <div v-if="row.user_agent || row.related_user_agent">
                  <dt>{{ tr('userAgent') }}</dt>
                  <dd v-if="row.user_agent" class="break-words">{{ row.user_email }} · {{ row.user_agent }}</dd>
                  <dd v-if="row.related_user_agent" class="break-words">{{ row.related_email }} · {{ row.related_user_agent }}</dd>
                </div>
              </dl>
            </details>
          </template>
          <template #cell-inviter_email="{ value }">{{ value || '—' }}</template>
          <template #cell-activity="{ row }">
            <div>{{ tr('accountReward', { email: row.user_email, count: row.checkin_count, amount: formatUsd(row.reward_amount) }) }}</div>
            <div>{{ tr('accountReward', { email: row.related_email, count: row.related_checkin_count, amount: formatUsd(row.related_reward_amount) }) }}</div>
          </template>
          <template #cell-first_seen_at="{ value }">{{ formatDateTime(value) || value }}</template>
          <template #cell-last_seen_at="{ value }">{{ formatDateTime(value) || value }}</template>
          <template #cell-actions="{ row }">
            <div v-if="tab === 'reviews' && row.status === 'pending'" class="flex gap-2">
              <button type="button" class="btn btn-sm btn-secondary" data-test="approve-review" @click="editReview(row, 'approve')">{{ tr('approve') }}</button>
              <button type="button" class="btn btn-sm btn-secondary" @click="editReview(row, 'reject')">{{ tr('reject') }}</button>
            </div>
            <button v-else-if="tab === 'reviews' && row.status === 'rejected' && row.checkin_date === beijingToday()" type="button" class="btn btn-sm btn-secondary" data-test="appeal-review" @click="editReview(row, 'appeal')">{{ tr('appeal') }}</button>
            <button v-else-if="tab === 'whitelist'" type="button" class="btn btn-sm btn-secondary" @click="editWhitelist(false, row.email)">{{ tr('removeWhitelist') }}</button>
          </template>
        </DataTable>
        <Pagination v-if="total && !loadError" :page="page" :page-size="pageSize" :page-size-options="[10, 20, 50, 100]" :total="total" @update:page="changePage" @update:page-size="changePageSize" />
      </template>
    </div>
    <BaseDialog :show="!!action" :title="action ? tr(action.label) : ''" width="narrow" @close="closeAction">
      <form v-if="action" data-test="risk-action-form" class="space-y-4" @submit.prevent="submitAction">
        <label class="block">
          <span class="input-label">{{ tr('email') }}</span>
          <input v-model.trim="action.email" required type="email" :readonly="action.label !== 'addWhitelist'" class="input" />
        </label>
        <label class="block">
          <span class="input-label">{{ tr('reason') }}</span>
          <textarea v-model.trim="reason" data-test="risk-action-reason" required maxlength="250" rows="3" class="input" />
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="closeAction">{{ t('common.cancel') }}</button>
          <button data-test="confirm-risk-action" type="submit" class="btn btn-primary" :disabled="saving || !reason || !action.email">{{ tr(action.label) }}</button>
        </div>
      </form>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { checkinRisk, riskReasons, type RiskAssociation, type RiskReview, type RiskWhitelistEntry, type ReviewStatus, type RiskReason } from '@/api/admin/checkinRisk'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { beijingToday, formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import CheckinRiskSettings from './CheckinRiskSettings.vue'

const { t } = useI18n()
const tr = (key: string, params = {}) => t('admin.checkinRisk.' + key, params)
const app = useAppStore()
const tabs = ['reviews', 'associations', 'whitelist', 'settings'] as const
type Tab = typeof tabs[number]
type Row = RiskAssociation | RiskReview | RiskWhitelistEntry
const reviewStatuses: ReviewStatus[] = ['pending', 'observed', 'approved', 'rejected']
const availableStatuses = computed(() => tab.value === 'associations' ? ['none', ...reviewStatuses] : reviewStatuses)
const tab = ref<Tab>('reviews')
const open = ref(false)
const items = ref<Row[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const emptyFilters = () => ({ search: '', status: '' as ReviewStatus | 'none' | '', date: '', reason: '' as RiskReason | '', start_date: '', end_date: '' })
const draft = ref(emptyFilters())
const filters = ref(emptyFilters())
const loading = ref(false)
const loadError = ref('')
let generation = 0
const columnKeys = {
  reviews: ['user_email', 'owner_email', 'checkin_date', 'created_at', 'status', 'actions'],
  associations: ['user_email', 'related_email', 'signals', 'inviter_email', 'first_seen_at', 'last_seen_at', 'activity', 'status'],
  whitelist: ['email', 'reason', 'created_at', 'actions'],
}
const columns = computed(() => (tab.value === 'settings' ? [] : columnKeys[tab.value]).map(key => ({ key, label: tr('columns.' + key) })))
const usd = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 6 })
const formatUsd = (value: number) => usd.format(value)
const signalNames = (reasons: RiskReason[]) => reasons.map(signal => tr('signals.' + signal)).join(' · ')
function rowKey(row: Row) {
  if ('related_email' in row) return JSON.stringify([row.user_email, row.related_email])
  if ('user_id' in row) return JSON.stringify([row.user_id, row.browser_hash, row.checkin_date])
  return row.email
}

async function load() {
  const serial = ++generation
  if (!open.value || tab.value === 'settings') return
  loading.value = true
  items.value = []
  loadError.value = ''
  const f = filters.value
  const query = { page: page.value, page_size: pageSize.value, search: f.search || undefined }
  try {
    let result
    if (tab.value === 'reviews') {
      result = await checkinRisk.reviews({ ...query, date: f.date || undefined, status: f.status && f.status !== 'none' ? f.status : undefined })
    } else if (tab.value === 'associations') {
      result = await checkinRisk.associations({ ...query, status: f.status || undefined, reason: f.reason || undefined, start_date: f.start_date || undefined, end_date: f.end_date || undefined })
    } else {
      result = await checkinRisk.whitelist(query)
    }
    if (serial !== generation) return
    items.value = result.items
    total.value = result.total
    if (page.value > 1 && result.items.length === 0) {
      page.value = Math.max(1, Math.ceil(result.total / pageSize.value))
      if (page.value !== query.page) await load()
    }
  } catch (e) {
    if (serial === generation) loadError.value = extractApiErrorMessage(e, tr('loadError'))
  } finally {
    if (serial === generation) loading.value = false
  }
}
function toggle() {
  open.value = !open.value
  void load()
}
function selectTab(next: Tab) {
  if (tab.value === next) return
  tab.value = next
  draft.value = emptyFilters()
  total.value = 0
  applyFilters()
}
function applyFilters() {
  filters.value = { ...draft.value }
  page.value = 1
  void load()
}
function changePage(next: number) {
  page.value = next
  void load()
}
function changePageSize(size: number) {
  pageSize.value = size
  changePage(1)
}

type Action = { label: 'approve' | 'reject' | 'appeal'; review: RiskReview; email: string } | { label: 'addWhitelist' | 'removeWhitelist'; email: string }
const action = ref<Action | null>(null)
const saving = ref(false)
const reason = ref('')
function editReview(review: RiskReview, label: 'approve' | 'reject' | 'appeal') {
  reason.value = ''
  action.value = { label, review, email: review.user_email }
}
function editWhitelist(enabled: boolean, email = '') {
  reason.value = ''
  action.value = { label: enabled ? 'addWhitelist' : 'removeWhitelist', email }
}
function closeAction() {
  if (!saving.value) action.value = null
}
async function submitAction() {
  const input = action.value
  if (!input || !reason.value || !input.email || saving.value) return
  if (input.label === 'appeal' && input.review.checkin_date !== beijingToday()) {
    app.showError(tr('appealExpired'))
    action.value = null
    await load()
    return
  }
  saving.value = true
  try {
    if ('review' in input) {
      const { user_id, browser_hash, checkin_date } = input.review
      await checkinRisk.decide({ user_id, browser_hash, checkin_date, decision: input.label === 'reject' ? 'rejected' : 'approved', reason: reason.value })
    } else {
      await checkinRisk.changeWhitelist({ email: input.email, enabled: input.label === 'addWhitelist', reason: reason.value })
    }
    action.value = null
    app.showSuccess(tr('saved'))
    await load()
  } catch (e) {
    app.showError(extractApiErrorMessage(e, tr('saveError')))
  } finally {
    saving.value = false
  }
}
onBeforeUnmount(() => { generation++ })
</script>
