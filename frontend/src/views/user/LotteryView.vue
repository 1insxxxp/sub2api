<template>
  <AppLayout>
    <div class="lottery-page mx-auto w-full max-w-6xl space-y-6">
      <section class="lottery-hero overflow-hidden rounded-2xl border border-blue-200/80 bg-white shadow-sm dark:border-blue-900/60 dark:bg-dark-900">
        <div class="relative px-5 py-7 sm:px-8 sm:py-10">
          <div class="relative flex flex-col gap-7 lg:flex-row lg:items-end lg:justify-between">
            <div class="max-w-2xl">
              <div class="mb-4 flex items-center gap-3 text-blue-600 dark:text-blue-300">
                <span class="flex h-11 w-11 items-center justify-center rounded-xl bg-blue-50 dark:bg-blue-900/30">
                  <Icon name="gift" size="lg" />
                </span>
                <span class="text-sm font-semibold uppercase tracking-[0.16em]">{{ t('lottery.title') }}</span>
              </div>
              <h1 class="text-3xl font-bold tracking-tight text-slate-950 dark:text-white sm:text-4xl">
                {{ state?.activity.name || t('lottery.title') }}
              </h1>
              <p class="mt-3 max-w-xl text-sm leading-6 text-slate-600 dark:text-slate-300">
                {{ state?.activity.description || t('lottery.description') }}
              </p>
            </div>
            <div v-if="state" class="lottery-attempt-card rounded-xl border border-blue-100 bg-blue-50/80 px-5 py-4 dark:border-blue-900/50 dark:bg-blue-950/40">
              <p class="text-xs font-medium text-blue-700 dark:text-blue-300">{{ t('lottery.attempts') }}</p>
              <div class="mt-1 flex items-end gap-2">
                <strong class="text-4xl leading-none text-blue-700 dark:text-blue-200">{{ state?.attempts_remaining ?? 0 }}</strong>
              </div>
              <p data-test="lottery-attempt-breakdown" class="mt-2 text-xs text-blue-600/80 dark:text-blue-300/80">
                {{ t('lottery.attemptBreakdown', { reward: state?.reward_attempts_remaining ?? state?.attempts_remaining ?? 0 }) }}
              </p>
              <p class="mt-1 text-xs text-blue-600/80 dark:text-blue-300/80">
                {{ t('lottery.attemptsUsed', { count: state?.attempts_used ?? 0 }) }}
              </p>
            </div>
            <div v-else class="lottery-attempt-card rounded-xl border border-amber-200 bg-amber-50/80 px-5 py-4 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200">
              {{ t('lottery.noActivity') }}
            </div>
          </div>
        </div>
      </section>

      <div v-if="loading" class="flex min-h-56 items-center justify-center rounded-2xl border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <LoadingSpinner />
      </div>
      <div v-else class="space-y-6">
        <div v-if="loadError" class="mb-6 rounded-2xl border border-amber-200 bg-amber-50 px-5 py-8 text-center text-sm text-amber-800 dark:border-amber-800/60 dark:bg-amber-950/20 dark:text-amber-200">
          {{ loadError }}
        </div>
        <template v-if="state">
          <section class="lottery-draw-panel space-y-4 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6">
            <LotterySlotMachine
              :prizes="state.prizes"
              :is-drawing="drawing"
              :winner-id="winnerPrizeId"
              :label="t('lottery.slotLabel')"
              :kicker="t('lottery.slotKicker')"
              :hint="t('lottery.slotHint')"
              :spinning-label="t('lottery.slotDrawing')"
              :winner-announcement="t('lottery.slotWinner')"
              :balance-label="t('lottery.slotBalance')"
              :product-label="t('lottery.slotProduct')"
              @settled="handleSlotSettled"
            />
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p class="text-sm font-semibold text-slate-950 dark:text-white">{{ t('lottery.prizes') }}</p>
                <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.description') }}</p>
              </div>
              <button
                type="button"
                class="lottery-draw-button inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-blue-600 to-cyan-500 px-7 text-sm font-semibold text-white shadow-lg shadow-blue-500/20 transition hover:from-blue-700 hover:to-cyan-600 disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto sm:min-w-52"
                :disabled="drawing || state.attempts_remaining <= 0 || state.prizes.length === 0"
                @click="handleDraw"
              >
                <Icon name="gift" size="sm" />
                {{ drawing ? t('lottery.drawing') : t('lottery.drawNow') }}
              </button>
            </div>
          </section>

          <section v-if="state.prizes.length" class="lottery-prize-section space-y-4 rounded-2xl border border-sky-100/90 bg-gradient-to-br from-white via-sky-50/40 to-cyan-50/35 p-4 shadow-sm dark:border-sky-900/50 dark:from-dark-900 dark:via-sky-950/20 dark:to-cyan-950/10 sm:p-5 lg:p-6">
            <div class="flex items-end justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold text-slate-950 dark:text-white">{{ t('lottery.prizeDetails') }}</h2>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('lottery.description') }}</p>
              </div>
            </div>
            <div class="lottery-prize-grid grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <article
                v-for="prize in state.prizes"
                :key="prize.id"
                class="lottery-prize-card flex min-h-40 flex-col rounded-xl border border-slate-200/90 bg-white/90 p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900/90 sm:p-5"
                :class="{ 'opacity-60': !prize.enabled || (prize.type === 'product' && prize.available_item_count <= 0) }"
              >
                <div class="flex items-start justify-between gap-3">
                  <span class="flex h-10 w-10 items-center justify-center rounded-lg" :class="prize.type === 'balance' ? 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/25 dark:text-emerald-300' : 'bg-violet-50 text-violet-600 dark:bg-violet-900/25 dark:text-violet-300'">
                    <Icon :name="prize.type === 'balance' ? 'creditCard' : 'gift'" size="sm" />
                  </span>
                  <span class="rounded-full bg-slate-100 px-2 py-1 text-[11px] font-semibold text-slate-500 dark:bg-dark-700 dark:text-slate-300">
                    {{ prize.type === 'balance' ? t('lottery.balancePrize') : t('lottery.productPrize') }}
                  </span>
                </div>
                <h3 class="mt-4 break-words text-base font-semibold text-slate-950 dark:text-white">{{ prize.name }}</h3>
                <p v-if="prize.description" class="mt-1 line-clamp-2 text-sm leading-5 text-slate-500 dark:text-slate-400">{{ prize.description }}</p>
                <div class="mt-auto flex items-center justify-between gap-3 pt-4 text-xs text-slate-500 dark:text-slate-400">
                  <strong v-if="prize.type === 'balance'" class="text-base text-emerald-600 dark:text-emerald-300">+{{ formatAmount(prize.balance_amount) }}</strong>
                  <span v-else>{{ prize.available_item_count > 0 ? t('lottery.inventory', { count: prize.available_item_count }) : t('lottery.noInventory') }}</span>
                </div>
              </article>
            </div>
          </section>
          <div v-else class="rounded-xl border border-dashed border-slate-300 px-5 py-10 text-center text-sm text-slate-500 dark:border-dark-600 dark:text-slate-400">
            {{ t('lottery.noPrizes') }}
          </div>
        </template>

        <section class="lottery-history-section rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6">
          <div class="flex flex-wrap items-end justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-slate-950 dark:text-white">{{ t('lottery.history') }}</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('lottery.attemptsUsed', { count: history.total }) }}</p>
            </div>
            <div v-if="history.pages > 1" class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <button type="button" class="btn btn-secondary btn-sm" :disabled="historyPage <= 1" @click="loadHistory(historyPage - 1)">{{ t('lottery.previous') }}</button>
              <span>{{ t('lottery.page', { page: historyPage }) }}</span>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="historyPage >= history.pages" @click="loadHistory(historyPage + 1)">{{ t('lottery.next') }}</button>
            </div>
          </div>
          <p v-if="historyError" class="mt-5 rounded-lg bg-amber-50 px-3 py-3 text-sm text-amber-800 dark:bg-amber-950/20 dark:text-amber-200">{{ historyError }}</p>
          <div v-else-if="history.items.length" class="mt-5 divide-y divide-slate-100 dark:divide-dark-700">
            <div v-for="item in history.items" :key="item.id" class="flex flex-col gap-2 py-4 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0">
                <p class="break-words text-sm font-semibold text-slate-900 dark:text-white">{{ item.prize_name }}</p>
                <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ formatDate(item.created_at) }}</p>
                <div v-if="item.product_content" class="mt-2 flex max-w-xl items-start gap-2 rounded-lg bg-violet-50 px-3 py-2 dark:bg-violet-950/20">
                  <code class="min-w-0 flex-1 break-all text-xs text-violet-800 dark:text-violet-200">{{ item.product_content }}</code>
                  <button type="button" class="shrink-0 text-xs font-semibold text-violet-700 hover:text-violet-900 dark:text-violet-300 dark:hover:text-violet-100" @click="copyProduct(item.product_content)">{{ t('lottery.copy') }}</button>
                </div>
              </div>
              <span class="w-fit rounded-lg px-3 py-1.5 text-sm font-semibold" :class="item.prize_type === 'balance' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300' : 'bg-violet-50 text-violet-700 dark:bg-violet-900/25 dark:text-violet-300'">
                {{ item.prize_type === 'balance' ? t('lottery.historyBalance', { amount: formatAmount(item.balance_amount) }) : t('lottery.historyProduct') }}
              </span>
            </div>
          </div>
          <p v-else class="py-10 text-center text-sm text-slate-500 dark:text-slate-400">{{ t('lottery.historyEmpty') }}</p>
        </section>
      </div>

      <div v-if="result" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4 backdrop-blur-sm" @click.self="result = null">
        <section class="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-6 shadow-2xl dark:border-dark-700 dark:bg-dark-900">
          <div class="text-center">
            <span class="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-amber-50 text-amber-500 dark:bg-amber-900/25 dark:text-amber-300"><Icon name="trophy" size="xl" /></span>
            <h2 class="mt-4 text-xl font-bold text-slate-950 dark:text-white">{{ t('lottery.resultTitle') }}</h2>
            <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">
              {{ result.draw.prize_type === 'balance' ? t('lottery.resultBalance', { amount: formatAmount(result.draw.balance_amount) }) : t('lottery.resultProduct') }}
            </p>
          </div>
          <div v-if="result.draw.product_content" class="mt-5 rounded-xl border border-violet-200 bg-violet-50 p-4 dark:border-violet-800/60 dark:bg-violet-950/20">
            <code class="block max-h-40 overflow-auto whitespace-pre-wrap break-all text-sm text-violet-900 dark:text-violet-100">{{ result.draw.product_content }}</code>
            <button type="button" class="btn btn-secondary mt-4 w-full justify-center" @click="copyProduct(result.draw.product_content)"><Icon name="copy" size="sm" /> {{ copied ? t('lottery.copied') : t('lottery.copy') }}</button>
          </div>
          <button type="button" class="btn btn-primary mt-5 w-full justify-center" @click="result = null">{{ t('lottery.close') }}</button>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import LotterySlotMachine from '@/components/lottery/LotterySlotMachine.vue'
import { lotteryAPI, type LotteryDrawResult, type LotteryPublicState } from '@/api/lottery'
import { useAppStore } from '@/stores/app'

const { t, locale } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const drawing = ref(false)
const loadError = ref('')
const state = ref<LotteryPublicState | null>(null)
const result = ref<LotteryDrawResult | null>(null)
const pendingResult = ref<LotteryDrawResult | null>(null)
const winnerPrizeId = ref<number | null>(null)
const copied = ref(false)
const historyError = ref('')
const historyPage = ref(1)
const history = ref({ items: [] as LotteryDrawResult['draw'][], total: 0, page: 1, page_size: 10, pages: 0 })

const formatAmount = (value?: number | null) => `$${Number(value || 0).toFixed(2)}`
const formatDate = (value: string) => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
const newAttemptKey = () => `${Date.now()}-${typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2)}`
const lotteryErrorMessage = (error: any, fallback: string) => {
  if (error?.code === 'LOTTERY_ACTIVITY_NOT_FOUND') return t('lottery.noActivity')
  if (error?.code === 'LOTTERY_DISABLED') return t('lottery.disabled')
  return error?.message || fallback
}

async function loadHistory(page = 1) {
  historyError.value = ''
  try {
    const data = await lotteryAPI.history({ page, page_size: 10 })
    history.value = data
    historyPage.value = page
  } catch (error: any) {
    historyError.value = lotteryErrorMessage(error, t('lottery.unavailable'))
    throw error
  }
}

async function load() {
  loading.value = true
  loadError.value = ''
  historyError.value = ''
  winnerPrizeId.value = null
  pendingResult.value = null
  try {
    state.value = await lotteryAPI.getState()
  } catch (error: any) {
    loadError.value = lotteryErrorMessage(error, t('lottery.unavailable'))
  }
  try { await loadHistory() } catch { /* historyError already contains the localized message */ }
  loading.value = false
}

function resolveWinnerPrizeId(draw: LotteryDrawResult['draw']) {
  if (draw.prize_id) return draw.prize_id
  return state.value?.prizes.find(prize => prize.name === draw.prize_name && prize.type === draw.prize_type)?.id ?? null
}

function handleSlotSettled() {
  if (!pendingResult.value) return
  result.value = pendingResult.value
  pendingResult.value = null
  drawing.value = false
}

async function handleDraw() {
  if (!state.value || state.value.attempts_remaining <= 0 || drawing.value) return
  drawing.value = true
  winnerPrizeId.value = null
  pendingResult.value = null
  result.value = null
  try {
    const drawResult = await lotteryAPI.draw(newAttemptKey())
    pendingResult.value = drawResult
    state.value.attempts_remaining = drawResult.attempts_remaining
    state.value.attempts_used = drawResult.attempts_used
    state.value.activity_attempts_remaining = drawResult.activity_attempts_remaining ?? 0
    state.value.reward_attempts_remaining = drawResult.reward_attempts_remaining ?? 0
    winnerPrizeId.value = resolveWinnerPrizeId(drawResult.draw)
    try { await loadHistory(1) } catch { /* historyError already contains the localized message */ }
    if (winnerPrizeId.value === null) {
      result.value = pendingResult.value
      pendingResult.value = null
      drawing.value = false
    }
  } catch (error: any) {
    pendingResult.value = null
    winnerPrizeId.value = null
    drawing.value = false
    appStore.showError(lotteryErrorMessage(error, t('lottery.unavailable')))
  }
}

async function copyProduct(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    copied.value = true
    window.setTimeout(() => { copied.value = false }, 1800)
  } catch {
    appStore.showError(t('lottery.unavailable'))
  }
}

onMounted(load)
</script>

<style scoped>
.lottery-page { padding-bottom: 2rem; }
.lottery-hero { background: linear-gradient(135deg, rgba(239,246,255,.92), rgba(255,255,255,.98) 58%, rgba(236,254,255,.86)); }
.dark .lottery-hero { background: linear-gradient(135deg, rgba(15,23,42,.98), rgba(15,23,42,.98) 58%, rgba(8,47,73,.55)); }
.lottery-draw-panel { background: linear-gradient(145deg, rgba(239,246,255,.72), rgba(255,255,255,.98) 55%, rgba(236,254,255,.66)); }
.dark .lottery-draw-panel { background: linear-gradient(145deg, rgba(15,23,42,.98), rgba(15,23,42,.98) 55%, rgba(8,47,73,.45)); }
.lottery-prize-card { transition: transform 160ms ease, box-shadow 160ms ease; }
.lottery-prize-card:hover { transform: translateY(-2px); box-shadow: 0 12px 28px rgba(15,23,42,.08); }
@media (max-width: 640px) { .lottery-draw-button { width: 100%; } }
</style>
