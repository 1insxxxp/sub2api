<template>
  <div
    class="pagination-shell flex items-center justify-between border-t border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800 sm:px-6"
    :class="{ 'pagination-compact': variant === 'compact' }"
  >
    <div class="pagination-mobile-controls flex flex-1 items-center justify-between sm:hidden">
      <span v-if="variant === 'compact'" class="pagination-total">{{ t('pagination.totalCount', { total }) }}</span>
      <!-- Mobile pagination -->
      <button
        type="button"
        @click="goToPage(page - 1)"
        :disabled="page === 1"
        :aria-label="t('pagination.previous')"
        :title="t('pagination.previous')"
        class="relative inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
      >
        <Icon v-if="variant === 'compact'" name="chevronLeft" size="sm" />
        <template v-else>{{ t('pagination.previous') }}</template>
      </button>
      <span class="pagination-current text-sm text-gray-700 dark:text-gray-300" role="status" :aria-label="t('pagination.pageOf', { page, total: totalPages })">
        <template v-if="variant === 'compact'">
          <strong>{{ page }}</strong><span class="pagination-separator" aria-hidden="true">/</span><span>{{ totalPages }}</span>
        </template>
        <template v-else>{{ t('pagination.pageOf', { page, total: totalPages }) }}</template>
      </span>
      <button
        type="button"
        @click="goToPage(page + 1)"
        :disabled="page === totalPages"
        :aria-label="t('pagination.next')"
        :title="t('pagination.next')"
        class="relative ml-3 inline-flex items-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
      >
        <Icon v-if="variant === 'compact'" name="chevronRight" size="sm" />
        <template v-else>{{ t('pagination.next') }}</template>
      </button>
    </div>

    <div class="pagination-desktop-controls hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
      <!-- Desktop pagination info -->
      <div class="pagination-summary flex items-center space-x-4">
        <p class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('pagination.showing') }}
          <span class="font-medium">{{ fromItem }}</span>
          {{ t('pagination.to') }}
          <span class="font-medium">{{ toItem }}</span>
          {{ t('pagination.of') }}
          <span class="font-medium">{{ total }}</span>
          {{ t('pagination.results') }}
        </p>

        <!-- Page size selector -->
        <div v-if="showPageSizeSelector" class="flex items-center space-x-2">
          <span class="text-sm text-gray-700 dark:text-gray-300"
            >{{ t('pagination.perPage') }}:</span
          >
          <div class="page-size-select w-20">
            <Select
              :model-value="pageSize"
              :options="pageSizeSelectOptions"
              :aria-label="t('pagination.perPage')"
              @update:model-value="handlePageSizeChange"
            />
          </div>
        </div>

        <div v-if="showJump" class="flex items-center space-x-2">
          <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('pagination.jumpTo') }}</span>
          <input
            v-model="jumpPage"
            type="number"
            min="1"
            :max="totalPages"
            class="input w-20 text-sm"
            :placeholder="t('pagination.jumpPlaceholder')"
            @keyup.enter="submitJump"
          />
          <button type="button" class="btn btn-ghost btn-sm" @click="submitJump">
            {{ t('pagination.jumpAction') }}
          </button>
        </div>
      </div>

      <!-- Desktop pagination buttons -->
      <nav
        class="relative z-0 inline-flex -space-x-px rounded-md shadow-sm"
        aria-label="Pagination"
      >
        <!-- Previous button -->
        <button
          type="button"
          @click="goToPage(page - 1)"
          :disabled="page === 1"
          class="relative inline-flex items-center rounded-l-md border border-gray-300 bg-white px-2 py-2 text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
          :aria-label="t('pagination.previous')"
          :title="t('pagination.previous')"
        >
          <Icon name="chevronLeft" size="md" />
        </button>

        <!-- Page numbers -->
        <button
          type="button"
          v-for="(pageNum, index) in visiblePages"
          :key="`${pageNum}-${index}`"
          @click="typeof pageNum === 'number' && goToPage(pageNum)"
          :disabled="typeof pageNum !== 'number'"
          :class="[
            'relative inline-flex items-center border px-4 py-2 text-sm font-medium',
            pageNum === page
              ? 'z-10 border-primary-500 bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400'
              : 'border-gray-300 bg-white text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600',
            typeof pageNum !== 'number' && 'cursor-default'
          ]"
          :aria-label="
            typeof pageNum === 'number' ? t('pagination.goToPage', { page: pageNum }) : undefined
          "
          :aria-current="pageNum === page ? 'page' : undefined"
        >
          {{ pageNum }}
        </button>

        <!-- Next button -->
        <button
          type="button"
          @click="goToPage(page + 1)"
          :disabled="page === totalPages"
          class="relative inline-flex items-center rounded-r-md border border-gray-300 bg-white px-2 py-2 text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
          :aria-label="t('pagination.next')"
          :title="t('pagination.next')"
        >
          <Icon name="chevronRight" size="md" />
        </button>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from './Select.vue'
import { useAdminAppearance } from '@/composables/useAdminAppearance'
import { getConfiguredTablePageSizeOptions, normalizeTablePageSize } from '@/utils/tablePreferences'
import { setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()

interface Props {
  total: number
  page: number
  pageSize: number
  pageSizeOptions?: number[]
  showPageSizeSelector?: boolean
  showJump?: boolean
  variant?: 'default' | 'compact'
}

interface Emits {
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', pageSize: number): void
}

const props = withDefaults(defineProps<Props>(), {
  pageSizeOptions: () => getConfiguredTablePageSizeOptions(),
  showPageSizeSelector: true,
  showJump: false
})
const adminAppearance = useAdminAppearance()
const variant = computed(() => props.variant ?? (adminAppearance.value ? 'compact' : 'default'))

const emit = defineEmits<Emits>()

const totalPages = computed(() => Math.ceil(props.total / props.pageSize))

const fromItem = computed(() => {
  if (props.total === 0) return 0
  return (props.page - 1) * props.pageSize + 1
})

const toItem = computed(() => {
  const to = props.page * props.pageSize
  return to > props.total ? props.total : to
})

const pageSizeSelectOptions = computed(() => {
  const options = Array.from(
    new Set([
      ...getConfiguredTablePageSizeOptions(),
      normalizeTablePageSize(props.pageSize)
    ])
  ).sort((a, b) => a - b)

  return options.map((size) => ({
    value: size,
    label: String(size)
  }))
})

const jumpPage = ref('')

const visiblePages = computed(() => {
  const pages: (number | string)[] = []
  const maxVisible = 7
  const total = totalPages.value

  if (total <= maxVisible) {
    // Show all pages if total is small
    for (let i = 1; i <= total; i++) {
      pages.push(i)
    }
  } else {
    // Always show first page
    pages.push(1)

    const start = Math.max(2, props.page - 2)
    const end = Math.min(total - 1, props.page + 2)

    // Add ellipsis before if needed
    if (start > 2) {
      pages.push('...')
    }

    // Add middle pages
    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    // Add ellipsis after if needed
    if (end < total - 1) {
      pages.push('...')
    }

    // Always show last page
    pages.push(total)
  }

  return pages
})

const goToPage = (newPage: number) => {
  if (newPage >= 1 && newPage <= totalPages.value && newPage !== props.page) {
    emit('update:page', newPage)
  }
}

const handlePageSizeChange = (value: string | number | boolean | null) => {
  if (value === null || typeof value === 'boolean') return
  const newPageSize = normalizeTablePageSize(typeof value === 'string' ? parseInt(value, 10) : value)
  setPersistedPageSize(newPageSize)
  emit('update:pageSize', newPageSize)
}

const submitJump = () => {
  const value = String(jumpPage.value).trim()
  if (!value) return
  const pageNum = Number.parseInt(value, 10)
  if (Number.isNaN(pageNum)) return
  const nextPage = Math.min(Math.max(pageNum, 1), totalPages.value)
  jumpPage.value = ''
  goToPage(nextPage)
}
</script>

<style scoped>
.page-size-select :deep(.select-trigger) {
  @apply px-3 py-1.5 text-sm;
}

.pagination-compact {
  --pagination-ink: #334155;
  --pagination-muted: #7b8492;
  --pagination-rule: rgb(148 163 184 / 22%);
  --pagination-surface: rgb(255 255 255 / 76%);
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  font-variant-numeric: tabular-nums;
}
.dark .pagination-compact {
  --pagination-ink: #e2e8f0;
  --pagination-muted: #a0a9b7;
  --pagination-rule: rgb(255 255 255 / 11%);
  --pagination-surface: rgb(39 41 46 / 88%);
  background: transparent;
}
.pagination-compact .pagination-total { color: var(--pagination-muted); font-size: 0.75rem; }
.pagination-compact .pagination-current {
  display: inline-flex;
  align-items: baseline;
  justify-content: center;
  gap: 0.5rem;
  min-width: 3.5rem;
  color: var(--pagination-muted);
  font-size: 0.8125rem;
}
.pagination-compact .pagination-current strong { color: var(--pagination-ink); font-weight: 600; }
.pagination-compact .pagination-separator { opacity: 0.5; }
.pagination-compact .pagination-mobile-controls > button,
.pagination-compact nav > button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  min-width: 2.25rem;
  height: 2.25rem;
  margin: 0;
  padding: 0 0.5rem;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--pagination-ink);
  font-size: 0.8125rem;
  box-shadow: none;
  transition: background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}
.pagination-compact .pagination-mobile-controls > button {
  width: 2.75rem;
  height: 2.75rem;
  border-color: var(--pagination-rule);
  background: var(--pagination-surface);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 12%), 0 1px 2px rgb(15 23 42 / 3%);
}
.pagination-compact .pagination-mobile-controls > button:disabled,
.pagination-compact nav > button:disabled { opacity: 0.35; box-shadow: none; }
.pagination-compact .pagination-mobile-controls > button:focus-visible,
.pagination-compact nav > button:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 0.6); outline-offset: 2px; }
.pagination-compact nav > button[aria-current="page"] {
  border-color: var(--pagination-rule);
  background: var(--pagination-surface);
  font-weight: 600;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 12%), 0 1px 3px rgb(15 23 42 / 4%);
}
.pagination-compact .pagination-desktop-controls { flex-wrap: wrap; gap: 0.75rem 1rem; }
.pagination-compact .pagination-summary { flex-wrap: wrap; gap: 0.5rem 1rem; }
.pagination-compact .pagination-summary > * { margin: 0; }
.pagination-compact .pagination-summary p,
.pagination-compact .pagination-summary > div > span { color: var(--pagination-muted); font-size: 0.75rem; }
.pagination-compact .pagination-summary p > span { color: var(--pagination-ink); }
.pagination-compact nav { flex-wrap: wrap; justify-content: flex-end; gap: 0.25rem; margin-left: auto; box-shadow: none; }
.pagination-compact .page-size-select :deep(.select-trigger) {
  border-color: var(--pagination-rule);
  border-radius: 6px;
  background: var(--pagination-surface);
  box-shadow: none;
}
@media (max-width: 639px) {
  .pagination-compact .pagination-mobile-controls { display: grid; grid-template-columns: minmax(0, 1fr) 2.75rem auto 2.75rem; gap: 0.5rem; }
  .pagination-compact .pagination-total { min-width: 0; white-space: normal; overflow-wrap: anywhere; }
}
@media (hover: hover) {
  .pagination-compact .pagination-mobile-controls > button:not(:disabled):hover,
  .pagination-compact nav > button:not(:disabled):hover { border-color: var(--pagination-rule); background: var(--pagination-surface); }
}
@media (prefers-reduced-motion: reduce) {
  .pagination-compact button { transition: none; }
}
</style>
