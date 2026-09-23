<script setup lang="ts">
import { computed, getCurrentInstance, nextTick, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{ activeFilters?: number; filterId?: string }>(), { activeFilters: 0 })
const { t } = useI18n()
const mobile = useMediaQuery('(max-width: 1023px)')
const open = ref(false)
const toggle = ref<HTMLButtonElement>()
const instanceId = getCurrentInstance()?.uid
const panelId = computed(() => props.filterId || `admin-list-filters-${instanceId}`)
const filtersVisible = computed(() => !mobile.value || open.value)

async function closeFilters(event: KeyboardEvent) {
  if (!mobile.value || !open.value || event.defaultPrevented) return
  event.preventDefault()
  open.value = false
  await nextTick()
  toggle.value?.focus()
}
</script>

<template>
  <div class="admin-list-toolbar">
    <div class="admin-list-toolbar-main">
      <div v-if="$slots.search" class="admin-list-search"><slot name="search" /></div>
      <div class="admin-list-actions">
        <button
          v-if="$slots.filters"
          ref="toggle"
          type="button"
          class="admin-list-filter-toggle"
          :class="{ 'is-active': activeFilters > 0 || open }"
          :aria-label="t('common.filter')"
          :aria-expanded="filtersVisible"
          :aria-controls="panelId"
          :title="t('common.filter')"
          data-test="admin-filter-toggle"
          @click="open = !open"
        >
          <Icon name="filter" size="sm" />
          <span>{{ t('common.filter') }}</span>
          <span v-if="activeFilters" class="admin-list-filter-count">{{ activeFilters }}</span>
        </button>
        <slot name="actions" />
      </div>
    </div>
    <div
      v-if="$slots.filters"
      v-show="filtersVisible"
      :id="panelId"
      class="admin-list-filters"
      role="group"
      :aria-label="t('common.filter')"
      data-test="admin-filter-panel"
      @keydown.esc="closeFilters"
    ><slot name="filters" /></div>
    <div v-if="$slots.secondary" class="admin-list-secondary"><slot name="secondary" /></div>
  </div>
</template>

<style scoped>
.admin-list-toolbar { display: flex; min-width: 0; flex-direction: column; gap: 0.75rem; }
.admin-list-toolbar-main { display: flex; min-width: 0; align-items: center; gap: 1rem; }
.admin-list-search { min-width: 0; flex: 1 1 16rem; max-width: 26rem; }
.admin-list-search :deep(.input) { width: 100%; }
.admin-list-actions { display: flex; flex: 1 0 auto; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 0.5rem; }
.admin-list-filters { display: flex; flex-wrap: wrap; gap: 0.625rem; align-items: center; padding-top: 0.875rem; border-top: 1px solid var(--workspace-divider); }
.admin-list-secondary { min-width: 0; }
.admin-list-filter-toggle { display: none; align-items: center; justify-content: center; gap: 0.375rem; min-height: 2.5rem; padding: 0.5rem 0.625rem; border: 1px solid var(--workspace-rule); border-radius: 7px; color: var(--workspace-muted); background: var(--workspace-control); font-size: 0.8125rem; font-weight: 500; transition: color 150ms ease, border-color 150ms ease; }
.admin-list-filter-toggle.is-active { color: rgb(var(--brand-rgb)); border-color: rgb(var(--brand-rgb) / 35%); }
.admin-list-filter-toggle:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 50%); outline-offset: 2px; }
.admin-list-filter-count { display: inline-flex; min-width: 1.125rem; height: 1.125rem; align-items: center; justify-content: center; border-radius: 4px; background: rgb(var(--brand-rgb) / 9%); font-size: 0.6875rem; font-variant-numeric: tabular-nums; }
@media (max-width: 1023px) {
  .admin-list-toolbar-main { flex-wrap: wrap; gap: 0.625rem; }
  .admin-list-filter-toggle { display: inline-flex; }
  .admin-list-search { max-width: none; }
  .admin-list-actions { flex: 0 1 auto; }
  .admin-list-filters { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; }
  .admin-list-filters :deep(.select-container) { width: 100%; min-width: 0; }
}
@media (max-width: 639px) {
  .admin-list-search { flex-basis: 100%; }
  .admin-list-actions { flex: 1 1 100%; gap: 0.375rem; }
  .admin-list-filter-toggle { flex: none; margin-right: auto; width: 2.75rem; min-width: 2.75rem; min-height: 2.75rem; padding-inline: 0.5rem; }
  .admin-list-filter-toggle > span:first-of-type { display: none; }
}
@media (prefers-reduced-motion: reduce) {
  .admin-list-filter-toggle { transition: none; }
}
</style>
