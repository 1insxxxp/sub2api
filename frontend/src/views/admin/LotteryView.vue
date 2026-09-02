<template>
  <AppLayout>
    <div class="admin-lottery-page mx-auto w-full max-w-7xl space-y-6">
      <div class="admin-toolbar-surface">
        <div class="admin-toolbar">
          <div class="admin-toolbar-group min-w-0 flex-1">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/25 dark:text-blue-300"><Icon name="gift" size="sm" /></div>
            <div class="min-w-0">
              <h1 class="truncate text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.title') }}</h1>
              <p class="mt-1 truncate text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.description') }}</p>
            </div>
          </div>
          <div class="admin-toolbar-group w-full justify-end lg:w-auto lg:flex-none">
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="loading" @click="loadConfig"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</button>
            <button type="button" data-test="save-lottery-activity" class="btn btn-primary inline-flex items-center gap-2" :disabled="savingActivity" @click="saveActivityForm"><Icon name="check" size="sm" />{{ savingActivity ? t('common.saving') : t('lottery.admin.saveActivity') }}</button>
          </div>
        </div>
      </div>

      <div v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300">{{ loadError }}</div>

      <section class="admin-surface p-5 sm:p-6">
        <div class="mb-5 flex items-start gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/25 dark:text-blue-300"><Icon name="calendar" size="sm" /></div>
          <div>
            <h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.activity') }}</h2>
            <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.activityHint') }}</p>
          </div>
        </div>
        <div class="grid gap-4 md:grid-cols-2">
          <div class="md:col-span-2"><label class="input-label">{{ t('lottery.admin.name') }}</label><input v-model="activityForm.name" class="input" :placeholder="t('lottery.admin.namePlaceholder')" /></div>
          <div class="md:col-span-2"><label class="input-label">{{ t('lottery.admin.descriptionLabel') }}</label><textarea v-model="activityForm.description" class="input min-h-24 resize-y" :placeholder="t('lottery.admin.descriptionPlaceholder')"></textarea></div>
          <div><label class="input-label">{{ t('lottery.admin.status') }}</label><select v-model="activityForm.status" class="input"><option value="draft">{{ t('lottery.admin.draft') }}</option><option value="active">{{ t('lottery.admin.active') }}</option><option value="disabled">{{ t('lottery.admin.disabledStatus') }}</option><option value="ended">{{ t('lottery.admin.ended') }}</option></select></div>
          <div><label class="input-label">{{ t('lottery.admin.attemptMode') }}</label><select v-model="activityForm.attempt_mode" class="input"><option value="daily">{{ t('lottery.admin.dailyMode') }}</option><option value="total">{{ t('lottery.admin.totalMode') }}</option></select></div>
          <div><label class="input-label">{{ t('lottery.admin.attemptLimit') }}</label><input v-model.number="activityForm.attempt_limit" data-test="lottery-attempt-limit" class="input" type="number" min="0" step="1" /><p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.attemptLimitHint') }}</p></div>
          <div><label class="input-label">{{ t('lottery.admin.startsAt') }}</label><input v-model="activityForm.starts_at" class="input" type="datetime-local" /></div>
          <div><label class="input-label">{{ t('lottery.admin.endsAt') }}</label><input v-model="activityForm.ends_at" class="input" type="datetime-local" /></div>
        </div>
      </section>

      <section class="admin-surface p-5 sm:p-6">
        <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex items-start gap-3">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-900/25 dark:text-amber-300"><Icon name="trophy" size="sm" /></div>
            <div><h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.prizes') }}</h2><p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.prizesHint') }}</p></div>
          </div>
          <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" @click="startNewPrize"><Icon name="plus" size="sm" />{{ t('lottery.admin.addPrize') }}</button>
        </div>

        <div v-if="editing" class="mb-6 rounded-xl border border-blue-200 bg-blue-50/60 p-4 dark:border-blue-900/60 dark:bg-blue-950/20">
          <div class="mb-4 flex items-center justify-between gap-3"><h3 class="text-sm font-semibold text-slate-950 dark:text-white">{{ editing.id ? t('lottery.admin.editPrize') : t('lottery.admin.newPrize') }}</h3><button type="button" class="icon-button" :title="t('lottery.close')" @click="editing = null"><Icon name="x" size="sm" /></button></div>
          <div class="grid gap-4 md:grid-cols-2">
            <div><label class="input-label">{{ t('lottery.admin.prizeName') }}</label><input v-model="editing.name" class="input" /></div>
            <div><label class="input-label">{{ t('lottery.admin.prizeType') }}</label><select v-model="editing.type" class="input"><option value="balance">{{ t('lottery.admin.balanceType') }}</option><option value="product">{{ t('lottery.admin.productType') }}</option></select></div>
            <div class="md:col-span-2"><label class="input-label">{{ t('lottery.admin.prizeDescription') }}</label><input v-model="editing.description" class="input" /></div>
            <div v-if="editing.type === 'balance'"><label class="input-label">{{ t('lottery.admin.balanceAmount') }}</label><input v-model.number="editing.balance_amount" class="input" type="number" min="0.01" step="0.01" /></div>
            <div><label class="input-label">{{ t('lottery.admin.weight') }}</label><input v-model.number="editing.weight" class="input" type="number" min="1" step="1" /></div>
            <div><label class="input-label">{{ t('lottery.admin.sortOrder') }}</label><input v-model.number="editing.sort_order" class="input" type="number" step="1" /></div>
            <label class="flex cursor-pointer items-center gap-3 self-end pb-2 text-sm font-medium text-slate-700 dark:text-slate-200"><input v-model="editing.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />{{ t('lottery.admin.enabled') }}</label>
          </div>
          <div class="mt-4 flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="editing = null">{{ t('lottery.close') }}</button><button type="button" class="btn btn-primary" :disabled="savingPrize" @click="savePrize"><Icon name="check" size="sm" />{{ savingPrize ? t('common.saving') : t('lottery.admin.savePrize') }}</button></div>
        </div>

        <div v-if="prizes.length" class="space-y-4">
          <article v-for="prize in prizes" :key="prize.id" class="rounded-xl border border-slate-200 dark:border-dark-700">
            <div class="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><h3 class="break-words text-sm font-semibold text-slate-950 dark:text-white">{{ prize.name }}</h3><span class="rounded bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500 dark:bg-dark-700 dark:text-slate-300">{{ prize.type === 'balance' ? t('lottery.admin.balanceType') : t('lottery.admin.productType') }}</span><span v-if="!prize.enabled" class="rounded bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500 dark:bg-dark-700 dark:text-slate-300">{{ t('lottery.admin.disabledStatus') }}</span></div><p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ prize.type === 'balance' ? `${t('lottery.admin.balanceAmount')}: ${formatAmount(prize.balance_amount)}` : `${t('lottery.admin.available', { count: prize.available_item_count })}` }} · {{ t('lottery.admin.weight') }} {{ prize.weight }}</p></div>
              <div class="flex flex-wrap items-center gap-2"><button v-if="prize.type === 'product'" type="button" class="btn btn-secondary btn-sm" @click="toggleInventory(prize.id)"><Icon name="database" size="sm" />{{ t('lottery.admin.inventory') }}</button><button type="button" class="btn btn-secondary btn-sm" @click="editPrize(prize)"><Icon name="edit" size="sm" />{{ t('lottery.admin.editPrize') }}</button><button type="button" class="btn btn-secondary btn-sm text-red-600 hover:text-red-700" @click="removePrize(prize)"><Icon name="trash" size="sm" /></button></div>
            </div>
            <div v-if="prize.type === 'product' && inventoryOpen === prize.id" class="border-t border-slate-200 bg-slate-50/70 p-4 dark:border-dark-700 dark:bg-dark-950/40">
              <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
                <div><div class="mb-2 flex items-center justify-between gap-2"><p class="text-xs font-semibold text-slate-700 dark:text-slate-200">{{ t('lottery.admin.inventory') }}</p><span class="text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.available', { count: inventoryItems.length ? inventoryItems.filter(item => item.status === 'available').length : prize.available_item_count }) }}</span></div><div v-if="inventoryItems.length" class="max-h-56 space-y-2 overflow-y-auto"> <label v-for="item in inventoryItems" :key="item.id" class="flex items-start gap-2 rounded-lg border border-slate-200 bg-white p-2 text-xs dark:border-dark-700 dark:bg-dark-900"><input v-if="item.status === 'available'" v-model="selectedItemIds" :value="item.id" type="checkbox" class="mt-0.5 rounded border-slate-300 text-blue-600" /><code class="min-w-0 flex-1 break-all" :class="item.status === 'claimed' ? 'text-slate-400 line-through' : 'text-slate-700 dark:text-slate-200'">{{ item.content }}</code><span class="shrink-0 text-[10px] text-slate-400">{{ item.status }}</span></label></div><p v-else class="py-5 text-center text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.noInventory') }}</p><button v-if="selectedItemIds.length" type="button" class="btn btn-secondary btn-sm mt-3 text-red-600" @click="deleteSelectedItems(prize.id)">{{ t('lottery.admin.deleteAvailable') }} ({{ selectedItemIds.length }})</button></div>
                <div><label class="input-label">{{ t('lottery.admin.appendInventory') }}</label><p class="mb-2 text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.inventoryHint') }}</p><textarea v-model="inventoryText" class="input min-h-32 resize-y font-mono text-xs" :placeholder="t('lottery.admin.inventoryPlaceholder')"></textarea><button type="button" class="btn btn-primary mt-3 w-full justify-center" :disabled="!inventoryText.trim() || inventorySaving" @click="appendInventory(prize.id)"><Icon name="upload" size="sm" />{{ inventorySaving ? t('common.saving') : t('lottery.admin.appendInventory') }}</button></div>
              </div>
            </div>
          </article>
        </div>
        <p v-else class="rounded-xl border border-dashed border-slate-300 px-5 py-10 text-center text-sm text-slate-500 dark:border-dark-600 dark:text-slate-400">{{ t('lottery.noPrizes') }}</p>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { lotteryAdminAPI, type LotteryPrizeItem } from '@/api/admin/lottery'
import type { LotteryActivity, LotteryAttemptMode, LotteryPrize } from '@/api/lottery'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const loadError = ref('')
const savingActivity = ref(false)
const savingPrize = ref(false)
const inventorySaving = ref(false)
const activityId = ref(0)
const prizes = ref<LotteryPrize[]>([])
const editing = ref<PrizeDraft | null>(null)
const inventoryOpen = ref(0)
const inventoryText = ref('')
const inventoryItems = ref<LotteryPrizeItem[]>([])
const selectedItemIds = ref<number[]>([])

interface ActivityForm { id?: number; name: string; description: string; status: string; attempt_mode: LotteryAttemptMode; attempt_limit: number; starts_at: string; ends_at: string }
interface PrizeDraft { id?: number; name: string; description: string; type: 'balance' | 'product'; weight: number; balance_amount?: number | null; enabled: boolean; sort_order: number }

const activityForm = reactive<ActivityForm>({ name: '', description: '', status: 'draft', attempt_mode: 'daily', attempt_limit: 0, starts_at: '', ends_at: '' })
const formatAmount = (value?: number | null) => `$${Number(value || 0).toFixed(2)}`
const toDateTimeLocal = (value?: string | null) => value ? new Date(value).toISOString().slice(0, 16) : ''
const toISOStringOrNull = (value: string) => value ? new Date(value).toISOString() : null

function applyActivity(activity?: LotteryActivity | null) {
  activityId.value = activity?.id || 0
  activityForm.id = activity?.id
  activityForm.name = activity?.name || ''
  activityForm.description = activity?.description || ''
  activityForm.status = activity?.status || 'draft'
  activityForm.attempt_mode = activity?.attempt_mode || 'daily'
  activityForm.attempt_limit = activity?.attempt_limit ?? 0
  activityForm.starts_at = toDateTimeLocal(activity?.starts_at)
  activityForm.ends_at = toDateTimeLocal(activity?.ends_at)
}

async function loadConfig() {
  loading.value = true
  loadError.value = ''
  try {
    const config = await lotteryAdminAPI.getConfig()
    applyActivity(config.activity)
    prizes.value = config.prizes || []
  } catch (error: any) {
    loadError.value = error?.message || t('lottery.admin.loadFailed')
  } finally { loading.value = false }
}

async function saveActivityForm() {
  if (!activityForm.name.trim()) { appStore.showError(t('lottery.admin.saveFailed')); return }
  savingActivity.value = true
  try {
    const saved = await lotteryAdminAPI.saveActivity({ id: activityForm.id, name: activityForm.name.trim(), description: activityForm.description, status: activityForm.status, attempt_mode: activityForm.attempt_mode, attempt_limit: Math.max(0, Math.floor(Number(activityForm.attempt_limit) || 0)), starts_at: toISOStringOrNull(activityForm.starts_at), ends_at: toISOStringOrNull(activityForm.ends_at) })
    applyActivity(saved)
    appStore.showSuccess(t('lottery.admin.saved'))
  } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) } finally { savingActivity.value = false }
}

function startNewPrize() {
  if (!activityId.value) { appStore.showWarning(t('lottery.admin.saveFailed')); return }
  editing.value = { name: '', description: '', type: 'balance', weight: 1, balance_amount: 1, enabled: true, sort_order: prizes.value.length }
}

function editPrize(prize: LotteryPrize) {
  editing.value = { id: prize.id, name: prize.name, description: prize.description, type: prize.type, weight: prize.weight, balance_amount: prize.balance_amount, enabled: prize.enabled, sort_order: prize.sort_order }
}

async function savePrize() {
  if (!editing.value || !activityId.value) return
  savingPrize.value = true
  try {
    const draft = editing.value
    const request = { name: draft.name.trim(), description: draft.description, type: draft.type, weight: Math.max(1, Number(draft.weight) || 1), balance_amount: draft.type === 'balance' ? Number(draft.balance_amount) : null, enabled: draft.enabled, sort_order: Number(draft.sort_order) || 0 }
    const saved = draft.id ? await lotteryAdminAPI.updatePrize(draft.id, request) : await lotteryAdminAPI.createPrize({ ...request, activity_id: activityId.value })
    const index = prizes.value.findIndex(item => item.id === saved.id)
    if (index >= 0) prizes.value[index] = saved
    else prizes.value.push(saved)
    editing.value = null
    appStore.showSuccess(t('lottery.admin.prizeSaved'))
  } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) } finally { savingPrize.value = false }
}

async function removePrize(prize: LotteryPrize) {
  if (!window.confirm(t('lottery.admin.deleteConfirm'))) return
  try { await lotteryAdminAPI.deletePrize(prize.id); prizes.value = prizes.value.filter(item => item.id !== prize.id); if (inventoryOpen.value === prize.id) inventoryOpen.value = 0; appStore.showSuccess(t('lottery.admin.prizeDeleted')) } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) }
}

async function toggleInventory(prizeId: number) {
  if (inventoryOpen.value === prizeId) { inventoryOpen.value = 0; return }
  inventoryOpen.value = prizeId; inventoryText.value = ''; selectedItemIds.value = []
  try { inventoryItems.value = await lotteryAdminAPI.listPrizeItems(prizeId) } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.loadFailed')) }
}

async function appendInventory(prizeId: number) {
  const contents = inventoryText.value.split(/\r?\n/).map(item => item.trim()).filter(Boolean)
  if (!contents.length) return
  inventorySaving.value = true
  try { const result = await lotteryAdminAPI.appendPrizeItems(prizeId, contents); inventoryText.value = ''; inventoryItems.value = await lotteryAdminAPI.listPrizeItems(prizeId); await loadConfig(); inventoryOpen.value = prizeId; appStore.showSuccess(t('lottery.admin.inventoryAdded', { count: result.added })) } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) } finally { inventorySaving.value = false }
}

async function deleteSelectedItems(prizeId: number) {
  if (!selectedItemIds.value.length) return
  try { await lotteryAdminAPI.deletePrizeItems(prizeId, selectedItemIds.value); inventoryItems.value = await lotteryAdminAPI.listPrizeItems(prizeId); selectedItemIds.value = []; await loadConfig(); inventoryOpen.value = prizeId } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) }
}

onMounted(loadConfig)
</script>

<style scoped>
.admin-lottery-page { padding-bottom: 2rem; }
</style>
