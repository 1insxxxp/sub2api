<template>
  <section
    data-testid="lottery-slot-machine"
    class="lottery-slot-machine"
    :aria-label="label || 'Lottery slot machine'"
    :aria-busy="isBusy"
    :data-slot-state="slotState"
  >
    <div class="lottery-slot-heading">
      <span class="lottery-slot-kicker">{{ kicker || 'LUCKY DRAW' }}</span>
      <span class="lottery-slot-hint">{{ hint || 'Match the winning reward' }}</span>
    </div>

    <div class="lottery-slot-frame">
      <div class="lottery-slot-glow" aria-hidden="true" />
      <div class="lottery-slot-center-band" aria-hidden="true" />
      <div class="lottery-slot-reels">
        <div
          v-for="(prize, index) in visiblePrizes"
          :key="`${prize.id}-${index}`"
          data-testid="lottery-slot-reel"
          class="lottery-slot-reel"
          :class="{ 'is-spinning': slotState === 'spinning', 'is-settling': slotState === 'settling', 'is-settled': slotState === 'settled' }"
          :style="{ '--reel-delay': `${index * 90}ms` }"
        >
          <div class="lottery-slot-symbol" :class="prize.type === 'balance' ? 'is-balance' : 'is-product'">
            <span class="lottery-slot-icon" aria-hidden="true">
              <Icon :name="prize.type === 'balance' ? 'creditCard' : 'gift'" size="lg" />
            </span>
            <span class="lottery-slot-prize-name" :title="prize.name">{{ prize.name }}</span>
            <span class="lottery-slot-prize-type">{{ prize.type === 'balance' ? balanceLabel : productLabel }}</span>
          </div>
        </div>
      </div>
    </div>

    <p
      v-if="slotState === 'settled' && winnerPrize"
      data-testid="lottery-slot-center"
      class="lottery-slot-announcement"
      aria-live="polite"
    >
      {{ winnerAnnouncement || 'Winning reward' }}: {{ winnerPrize.name }}
    </p>
    <p v-else-if="slotState === 'spinning' || slotState === 'settling'" class="lottery-slot-announcement" aria-live="off">
      {{ spinningLabel || 'Drawing…' }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import type { LotteryPrize } from '@/api/lottery'

type SlotState = 'idle' | 'spinning' | 'settling' | 'settled'

interface Props {
  prizes: LotteryPrize[]
  isDrawing: boolean
  winnerId?: number | null
  reducedMotion?: boolean
  label?: string
  kicker?: string
  hint?: string
  spinningLabel?: string
  winnerAnnouncement?: string
  balanceLabel?: string
  productLabel?: string
}

const props = withDefaults(defineProps<Props>(), {
  winnerId: null,
  reducedMotion: undefined,
  label: '',
  kicker: '',
  hint: '',
  spinningLabel: '',
  winnerAnnouncement: '',
  balanceLabel: 'Balance',
  productLabel: 'Product',
})

const emit = defineEmits<{ settled: [] }>()

const slotState = ref<SlotState>('idle')
const spinRound = ref(0)
const prefersReducedMotion = ref(false)
let settleTimer: number | undefined
let motionMediaQuery: MediaQueryList | undefined

const availablePrizes = computed(() => {
  const enabled = props.prizes.filter(prize => prize.enabled && (prize.type === 'balance' || prize.available_item_count > 0))
  return enabled.length ? enabled : props.prizes
})

const previewPrizes = computed(() => availablePrizes.value.slice(0, 3))
const visiblePrizes = ref<LotteryPrize[]>([])
const winnerPrize = computed(() => props.prizes.find(prize => prize.id === props.winnerId) || null)
const isBusy = computed(() => props.isDrawing || slotState.value === 'spinning' || slotState.value === 'settling')
const useReducedMotion = computed(() => props.reducedMotion ?? prefersReducedMotion.value)

function setVisiblePrizes(prizes: LotteryPrize[]) {
  const source = prizes.length ? prizes : availablePrizes.value
  visiblePrizes.value = Array.from({ length: 3 }, (_, index) => source[index % source.length]).filter(Boolean)
}

function beginSpin() {
  clearSettleTimer()
  spinRound.value += 1
  const source = availablePrizes.value
  if (!source.length) return
  setVisiblePrizes(Array.from({ length: source.length }, (_, index) => source[(index + spinRound.value) % source.length]))
  slotState.value = 'spinning'
}

function settle() {
  const prize = winnerPrize.value || availablePrizes.value[0]
  if (!prize) return
  setVisiblePrizes([prize, prize, prize])
  slotState.value = 'settling'
  clearSettleTimer()
  settleTimer = window.setTimeout(() => {
    slotState.value = 'settled'
    emit('settled')
  }, useReducedMotion.value ? 40 : 920)
}

function clearSettleTimer() {
  if (settleTimer !== undefined) {
    window.clearTimeout(settleTimer)
    settleTimer = undefined
  }
}

function handleMotionChange(event: MediaQueryListEvent) {
  prefersReducedMotion.value = event.matches
}

watch(() => props.isDrawing, drawing => {
  if (drawing) {
    beginSpin()
  } else if (!props.winnerId && slotState.value !== 'settled') {
    clearSettleTimer()
    slotState.value = 'idle'
    setVisiblePrizes(previewPrizes.value)
  }
})

watch(() => props.winnerId, winnerId => {
  if (winnerId !== null && winnerId !== undefined && props.isDrawing) settle()
})

watch(previewPrizes, prizes => {
  if (!props.isDrawing && slotState.value !== 'settled') setVisiblePrizes(prizes)
}, { immediate: true })

onMounted(() => {
  motionMediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  prefersReducedMotion.value = motionMediaQuery.matches
  motionMediaQuery.addEventListener?.('change', handleMotionChange)
})

onUnmounted(() => {
  clearSettleTimer()
  motionMediaQuery?.removeEventListener?.('change', handleMotionChange)
})
</script>

<style scoped>
.lottery-slot-machine {
  --slot-blue: #2563eb;
  --slot-cyan: #22d3ee;
  --slot-ink: #0f2f71;
  position: relative;
  overflow: hidden;
  border-radius: 1.5rem;
  border: 1px solid rgba(147, 197, 253, .7);
  background: linear-gradient(145deg, #eff8ff 0%, #ffffff 47%, #ecfeff 100%);
  padding: 1.25rem;
  box-shadow: 0 22px 50px rgba(37, 99, 235, .12), inset 0 1px 0 rgba(255, 255, 255, .9);
}
.dark .lottery-slot-machine { border-color: rgba(30, 64, 175, .8); background: linear-gradient(145deg, #0f2348 0%, #111827 48%, #083344 100%); box-shadow: 0 22px 50px rgba(2, 6, 23, .35), inset 0 1px 0 rgba(147, 197, 253, .12); }
.lottery-slot-heading { display: flex; align-items: baseline; justify-content: space-between; gap: .75rem; padding: .15rem .35rem .9rem; }
.lottery-slot-kicker { color: var(--slot-blue); font-size: .72rem; font-weight: 800; letter-spacing: .18em; }
.lottery-slot-hint { color: #64748b; font-size: .75rem; }
.dark .lottery-slot-hint { color: #94a3b8; }
.lottery-slot-frame { position: relative; overflow: hidden; border: .7rem solid #bfdbfe; border-radius: 1.25rem; background: linear-gradient(180deg, #dbeafe, #eff6ff 48%, #bfdbfe); padding: .55rem; box-shadow: inset 0 0 0 .18rem #2563eb, inset 0 0 1.2rem rgba(37, 99, 235, .35), 0 12px 25px rgba(37, 99, 235, .14); }
.dark .lottery-slot-frame { border-color: #1e40af; background: linear-gradient(180deg, #172554, #0f172a 48%, #164e63); box-shadow: inset 0 0 0 .18rem #38bdf8, inset 0 0 1.2rem rgba(56, 189, 248, .25), 0 12px 25px rgba(2, 6, 23, .32); }
.lottery-slot-glow { position: absolute; inset: 1.1rem 8%; background: radial-gradient(circle, rgba(255, 255, 255, .85), transparent 68%); pointer-events: none; }
.lottery-slot-center-band { position: absolute; z-index: 2; inset: 20% 0; border-top: 2px solid rgba(56, 189, 248, .75); border-bottom: 2px solid rgba(56, 189, 248, .75); background: rgba(255, 255, 255, .22); pointer-events: none; }
.lottery-slot-reels { position: relative; z-index: 1; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .45rem; }
.lottery-slot-reel { display: grid; min-width: 0; min-height: 10rem; place-items: center; overflow: hidden; border-radius: .95rem; background: linear-gradient(180deg, rgba(255, 255, 255, .92), rgba(248, 250, 252, .84)); box-shadow: inset 0 0 0 1px rgba(147, 197, 253, .8), 0 3px 8px rgba(15, 23, 42, .08); }
.dark .lottery-slot-reel { background: linear-gradient(180deg, rgba(30, 41, 59, .96), rgba(15, 23, 42, .94)); box-shadow: inset 0 0 0 1px rgba(96, 165, 250, .35), 0 3px 8px rgba(2, 6, 23, .3); }
.lottery-slot-symbol { display: grid; width: 100%; min-width: 0; place-items: center; gap: .35rem; padding: 1.1rem .45rem; text-align: center; color: var(--slot-ink); transition: filter 180ms ease, transform 180ms ease; }
.dark .lottery-slot-symbol { color: #dbeafe; }
.lottery-slot-icon { display: grid; height: 3.1rem; width: 3.1rem; place-items: center; border-radius: 1rem; background: #e0f2fe; color: #0284c7; box-shadow: inset 0 1px 0 rgba(255, 255, 255, .85); }
.is-product .lottery-slot-icon { background: #ede9fe; color: #7c3aed; }
.dark .lottery-slot-icon { background: rgba(14, 116, 144, .35); color: #67e8f9; }
.dark .is-product .lottery-slot-icon { background: rgba(109, 40, 217, .3); color: #c4b5fd; }
.lottery-slot-prize-name { display: block; width: 100%; overflow: hidden; color: inherit; font-size: .74rem; font-weight: 800; line-height: 1.2; text-overflow: ellipsis; white-space: nowrap; }
.lottery-slot-prize-type { color: #64748b; font-size: .65rem; }
.dark .lottery-slot-prize-type { color: #94a3b8; }
.is-spinning .lottery-slot-symbol { animation: lottery-slot-spin 180ms linear infinite; animation-delay: var(--reel-delay); filter: blur(.7px); }
.is-settling .lottery-slot-symbol { animation: lottery-slot-settle 920ms cubic-bezier(.16, 1, .3, 1) both; animation-delay: var(--reel-delay); }
.is-settled .lottery-slot-symbol { transform: scale(1.035); }
.lottery-slot-announcement { margin: .95rem .25rem 0; min-height: 1.2rem; color: #1d4ed8; font-size: .82rem; font-weight: 700; text-align: center; }
.dark .lottery-slot-announcement { color: #93c5fd; }
@keyframes lottery-slot-spin { 0% { transform: translateY(-9%); } 50% { transform: translateY(9%); } 100% { transform: translateY(-9%); } }
@keyframes lottery-slot-settle { 0% { transform: translateY(-12%) scale(.97); filter: blur(1.8px); } 48% { transform: translateY(5%) scale(1.015); filter: blur(.2px); } 100% { transform: translateY(0) scale(1); filter: blur(0); } }
@media (max-width: 640px) { .lottery-slot-machine { padding: .85rem; } .lottery-slot-frame { border-width: .45rem; padding: .35rem; } .lottery-slot-reel { min-height: 8.5rem; } .lottery-slot-icon { height: 2.5rem; width: 2.5rem; } .lottery-slot-symbol { padding: .8rem .25rem; } .lottery-slot-prize-name { font-size: .65rem; } .lottery-slot-prize-type { font-size: .58rem; } }
@media (prefers-reduced-motion: reduce) { .lottery-slot-symbol { animation-duration: 1ms !important; transition-duration: 1ms !important; } }
</style>
