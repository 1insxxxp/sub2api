<template>
  <section
    data-testid="tier-summary"
    class="tier-identity card relative overflow-hidden"
    :data-tier-theme="presentation.theme"
  >
    <div class="tier-identity__grid" aria-hidden="true"></div>
    <div class="tier-identity__scan" aria-hidden="true"></div>

    <div class="tier-identity__layout relative z-[1]">
      <div class="tier-identity__primary min-w-0">
        <div class="flex min-w-0 items-start gap-3 sm:gap-5">
          <div class="tier-identity__badge-stage" aria-hidden="true">
            <span class="tier-identity__ring tier-identity__ring--outer"></span>
            <span class="tier-identity__ring tier-identity__ring--inner"></span>
            <span class="tier-identity__node tier-identity__node--one"></span>
            <span class="tier-identity__node tier-identity__node--two"></span>
            <span class="tier-identity__node tier-identity__node--three"></span>
            <span class="tier-identity__node tier-identity__node--four"></span>
          </div>

          <img
            data-testid="current-tier-badge"
            :src="badgeSource(currentLevel)"
            alt=""
            class="tier-identity__badge relative z-[1] h-[72px] w-[72px] shrink-0 object-contain sm:h-[92px] sm:w-[92px]"
          />

          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 flex-wrap items-start justify-between gap-x-4 gap-y-2">
              <div class="min-w-0">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                  {{ t('affiliate.tiers.currentLevel') }}
                </p>
                <div class="mt-1.5 flex min-w-0 flex-wrap items-center gap-2">
                  <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
                    {{ tierLabel(currentLevel) }}
                  </h2>
                  <span
                    v-if="detail.has_custom_rebate_rate"
                    class="rounded-md border border-cyan-200 bg-cyan-50 px-2 py-0.5 text-xs font-medium text-cyan-700 dark:border-cyan-800 dark:bg-cyan-950/50 dark:text-cyan-300"
                  >
                    {{ t('affiliate.tiers.customRate') }}
                  </span>
                </div>
              </div>

              <div class="shrink-0 text-left sm:text-right">
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('affiliate.tiers.effectiveRate') }}
                </p>
                <p class="mt-1 text-xl font-semibold text-primary-600 dark:text-cyan-400">
                  {{ safeFormattedRate }}%
                </p>
              </div>
            </div>

            <div class="mt-4 border-t border-blue-100/80 pt-3 dark:border-blue-900/60">
              <div class="flex flex-wrap items-end justify-between gap-2 text-sm">
                <div>
                  <span class="text-gray-500 dark:text-dark-400">
                    {{ t('affiliate.tiers.qualifiedCount') }}
                  </span>
                  <strong class="ml-2 text-gray-900 dark:text-white">
                    {{ formatCount(safeQualifiedCount) }}
                  </strong>
                </div>
                <span v-if="displayNextTier" class="text-xs font-medium text-gray-600 dark:text-gray-300">
                  {{ t('affiliate.tiers.nextProgress', {
                    current: safeQualifiedCount,
                    target: safeNextTierTarget
                  }) }}
                </span>
                <span v-else class="text-xs font-medium text-cyan-700 dark:text-cyan-300">
                  {{ t('affiliate.tiers.highestLevel') }}
                </span>
              </div>

              <div
                v-if="displayNextTier"
                class="mt-2 h-2 overflow-hidden rounded-md bg-blue-100/80 dark:bg-blue-950/70"
                role="progressbar"
                :aria-label="progressLabel"
                :aria-valuenow="safeProgress"
                aria-valuemin="0"
                aria-valuemax="100"
              >
                <div
                  class="tier-identity__progress h-full rounded-md"
                  :style="{ width: `${safeProgress}%` }"
                ></div>
              </div>
            </div>
          </div>
        </div>

        <div class="tier-identity__objective mt-5 min-w-0 border-l-2 border-cyan-400 pl-3">
          <p class="text-xs font-medium text-cyan-700 dark:text-cyan-300">
            {{ t('affiliate.tiers.identity.stageObjective') }}
          </p>
          <p class="mt-1 break-words text-sm leading-6 text-gray-700 dark:text-gray-200">
            {{ objectiveText }}
          </p>
        </div>
      </div>

      <div class="tier-identity__rules min-w-0">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('affiliate.tiers.rulesTitle') }}
        </h3>
        <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
          {{ t('affiliate.tiers.rulesDescription', {
            amount: formatCurrency(normalizeNumber(detail.qualification_amount))
          }) }}
        </p>

        <div class="mt-3 grid grid-cols-2 border-y border-blue-100/80 dark:border-blue-900/60">
          <div
            v-for="tier in detail.tiers"
            :key="tier.level"
            data-testid="tier-rule"
            :data-current="isCurrentTier(tier.level) ? 'true' : 'false'"
            class="tier-identity__rule min-w-0 border-b border-blue-100/80 px-2 py-2.5 odd:border-r dark:border-blue-900/60 [&:nth-last-child(-n+2)]:border-b-0"
            :class="{ 'tier-identity__rule--current': isCurrentTier(tier.level) }"
          >
            <div class="flex min-w-0 items-center gap-2">
              <img
                data-testid="tier-rule-badge"
                :src="badgeSource(tier.level)"
                alt=""
                class="h-9 w-9 shrink-0 object-contain"
              />
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium text-gray-800 dark:text-gray-200">
                  {{ tierLabel(tier.level) }}
                </p>
                <div class="mt-0.5 flex min-w-0 items-baseline gap-1.5">
                  <span class="shrink-0 text-sm font-semibold text-primary-600 dark:text-cyan-400">
                    {{ formatRate(tier.rate_percent) }}%
                  </span>
                  <span class="truncate text-xs text-gray-500 dark:text-dark-400">
                    {{ t('affiliate.tiers.requirement', {
                      count: normalizeNumber(tier.min_qualified_invitees, Infinity, true)
                    }) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import standardTierBadge from '@/assets/affiliate-tiers/standard.webp'
import bronzeTierBadge from '@/assets/affiliate-tiers/bronze.webp'
import silverTierBadge from '@/assets/affiliate-tiers/silver.webp'
import goldTierBadge from '@/assets/affiliate-tiers/gold.webp'
import {
  getAffiliateTierPresentation,
  normalizeAffiliateTier
} from '@/config/affiliateTierPresentation'
import type {
  AffiliateTier,
  AffiliateTierDefinition,
  UserAffiliateDetail
} from '@/types'
import { formatCurrency } from '@/utils/format'

const props = defineProps<{
  detail: UserAffiliateDetail
  nextTier: AffiliateTierDefinition | null
  progress: number
  formattedRate: string
}>()

const { t } = useI18n()

const tierBadgeSources: Readonly<Record<AffiliateTier, string>> = Object.freeze({
  standard: standardTierBadge,
  bronze: bronzeTierBadge,
  silver: silverTierBadge,
  gold: goldTierBadge
})

const currentLevel = computed<AffiliateTier>(() => normalizeAffiliateTier(props.detail.automatic_level))
const presentation = computed(() => getAffiliateTierPresentation(currentLevel.value))
const displayNextTier = computed(() => presentation.value.theme === 'core' ? null : props.nextTier)
const safeProgress = computed(() => normalizeNumber(props.progress, 100))
const safeAffCount = computed(() => normalizeNumber(props.detail.aff_count))
const safeQualifiedRatioCount = computed(() => normalizeNumber(props.detail.qualified_invitee_count))
const safeQualifiedCount = computed(() => normalizeNumber(props.detail.qualified_invitee_count, Infinity, true))
const safeRemainingCount = computed(() => normalizeNumber(props.detail.remaining_qualified_invitees, Infinity, true))
const safeHistoryRebate = computed(() => normalizeNumber(props.detail.aff_history_quota))
const safeFormattedRate = computed(() => formatSafeNumber(props.formattedRate, 100))
const safeNextTierTarget = computed(() => normalizeNumber(
  displayNextTier.value?.min_qualified_invitees ?? 0,
  Infinity,
  true
))
const qualifiedRatio = computed(() => {
  if (safeAffCount.value <= 0) return '0%'
  const ratio = normalizeNumber(
    (safeQualifiedRatioCount.value / safeAffCount.value) * 100,
    100
  )
  return `${Math.round(ratio)}%`
})
const progressLabel = computed(() => t('affiliate.tiers.nextProgress', {
  current: safeQualifiedCount.value,
  target: safeNextTierTarget.value
}))
const objectiveText = computed(() => t(presentation.value.objectiveKey, {
  count: safeRemainingCount.value,
  ratio: qualifiedRatio.value,
  rebate: formatCurrency(safeHistoryRebate.value),
  qualified: safeQualifiedCount.value,
  rate: `${safeFormattedRate.value}%`
}))

function tierLabel(level: AffiliateTier | string): string {
  return t(getAffiliateTierPresentation(level).labelKey)
}

function badgeSource(level: AffiliateTier | string): string {
  return tierBadgeSources[normalizeAffiliateTier(level)]
}

function isCurrentTier(level: AffiliateTier): boolean {
  return level === currentLevel.value
}

function normalizeNumber(value: number | string, max = Infinity, integer = false): number {
  const parsed = typeof value === 'string' && value.trim() === '' ? 0 : Number(value)
  if (!Number.isFinite(parsed)) return 0
  const normalized = Math.min(max, Math.max(0, parsed))
  return integer ? Math.floor(normalized) : normalized
}

function formatSafeNumber(value: number | string, max = Infinity): string {
  const rounded = Math.round(normalizeNumber(value, max) * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
}

function formatRate(value: number): string {
  return formatSafeNumber(value, 100)
}

function formatCount(value: number): string {
  return value.toLocaleString()
}
</script>

<style scoped>
.tier-identity {
  --tier-accent: 37 99 235;
  --tier-cyan: 6 182 212;
  isolation: isolate;
  background-color: rgb(255 255 255);
}

.tier-identity__layout {
  display: grid;
  gap: 1.25rem;
  padding: 1.25rem;
}

.tier-identity__grid,
.tier-identity__scan {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.tier-identity__grid {
  background-image:
    linear-gradient(rgb(var(--tier-accent) / 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgb(var(--tier-accent) / 0.055) 1px, transparent 1px);
  background-size: 28px 28px;
  mask-image: linear-gradient(90deg, black, transparent 72%);
  opacity: 0.35;
}

.tier-identity[data-tier-theme='pulse'] .tier-identity__grid {
  opacity: 0.5;
}

.tier-identity[data-tier-theme='orbit'] .tier-identity__grid {
  background-size: 24px 24px;
  opacity: 0.65;
}

.tier-identity[data-tier-theme='core'] .tier-identity__grid {
  background-size: 20px 20px;
  opacity: 0.8;
}

.tier-identity__scan {
  width: 46%;
  background: linear-gradient(90deg, transparent, rgb(var(--tier-cyan) / 0.08), transparent);
  opacity: 0;
  transform: translateX(-120%);
}

.tier-identity[data-tier-theme='pulse'] .tier-identity__scan {
  opacity: 0.35;
}

.tier-identity[data-tier-theme='orbit'] .tier-identity__scan {
  opacity: 0.55;
}

.tier-identity[data-tier-theme='core'] .tier-identity__scan {
  opacity: 0.75;
}

.tier-identity__badge-stage {
  position: absolute;
  top: 0.9rem;
  left: 0.5rem;
  width: 6.25rem;
  height: 6.25rem;
  pointer-events: none;
}

.tier-identity__ring {
  position: absolute;
  border-radius: 50%;
  border-color: rgb(var(--tier-accent) / 0.22);
}

.tier-identity__ring--outer {
  inset: 0;
  border-width: 1px;
  border-style: dashed;
}

.tier-identity__ring--inner {
  inset: 15%;
  border-width: 1px;
  border-style: solid;
  border-color: rgb(var(--tier-cyan) / 0.28);
}

.tier-identity__node {
  position: absolute;
  width: 5px;
  height: 5px;
  border: 1px solid rgb(255 255 255 / 0.9);
  border-radius: 50%;
  background: rgb(var(--tier-cyan));
  box-shadow: 0 0 0 2px rgb(var(--tier-cyan) / 0.14);
  opacity: 0;
}

.tier-identity__node--one {
  top: 9%;
  left: 48%;
  opacity: 1;
}

.tier-identity__node--two {
  top: 48%;
  right: -2px;
}

.tier-identity__node--three {
  bottom: 7%;
  left: 30%;
}

.tier-identity__node--four {
  top: 35%;
  left: -1px;
}

.tier-identity[data-tier-theme='pulse'] .tier-identity__node--two,
.tier-identity[data-tier-theme='orbit'] .tier-identity__node--two,
.tier-identity[data-tier-theme='orbit'] .tier-identity__node--three,
.tier-identity[data-tier-theme='core'] .tier-identity__node {
  opacity: 1;
}

.tier-identity__badge {
  filter: drop-shadow(0 5px 8px rgb(15 23 42 / 0.16));
}

.tier-identity__progress {
  background: linear-gradient(90deg, rgb(var(--tier-accent)), rgb(var(--tier-cyan)));
}

.tier-identity__rule--current {
  background: rgb(var(--tier-accent) / 0.06);
  box-shadow: inset 0 0 0 1px rgb(var(--tier-accent) / 0.18);
}

@media (min-width: 1024px) {
  .tier-identity__layout {
    grid-template-columns: minmax(0, 1fr) minmax(0, 0.85fr);
    gap: 1.5rem;
    padding: 1.5rem;
  }

  .tier-identity__rules {
    border-left: 1px solid rgb(219 234 254 / 0.9);
    padding-left: 1.5rem;
  }
}

@media (max-width: 374px) {
  .tier-identity__layout {
    padding: 1rem;
  }

  .tier-identity__badge-stage {
    left: 0.1rem;
    width: 5rem;
    height: 5rem;
  }
}

@media (prefers-reduced-motion: no-preference) {
  .tier-identity__progress {
    transition: width 300ms ease;
  }

  .tier-identity__rule {
    transition: background-color 160ms ease, box-shadow 160ms ease;
  }

  .tier-identity[data-tier-theme='pulse'] .tier-identity__ring--outer,
  .tier-identity[data-tier-theme='orbit'] .tier-identity__ring--outer,
  .tier-identity[data-tier-theme='core'] .tier-identity__ring--outer {
    animation: tier-ring-rotate 18s linear infinite;
  }

  .tier-identity[data-tier-theme='pulse'] .tier-identity__scan,
  .tier-identity[data-tier-theme='orbit'] .tier-identity__scan,
  .tier-identity[data-tier-theme='core'] .tier-identity__scan {
    animation: tier-scan 7s ease-in-out infinite;
  }
}

@keyframes tier-ring-rotate {
  to { transform: rotate(360deg); }
}

@keyframes tier-scan {
  0%, 18% { transform: translateX(-120%); }
  68%, 100% { transform: translateX(250%); }
}

:global(.dark) .tier-identity {
  background-color: rgb(15 23 42);
}

:global(.dark) .tier-identity__grid {
  opacity: 0.26;
}

@media (min-width: 1024px) {
  :global(.dark) .tier-identity__rules {
    border-left-color: rgb(30 64 175 / 0.45);
  }
}
</style>
