<template>
  <article class="key-card" :class="{ 'is-selected': selected }" :style="{ '--key-accent': accentColor }">
    <header class="key-card-header">
      <div class="key-card-name" :title="row.name"><Cell field="name" /></div>
      <div v-if="visible('status')" class="key-card-status"><Cell field="status" /></div>
      <label v-if="selectable" class="key-card-select" @click.stop>
        <input
          type="checkbox"
          class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
          :checked="selected"
          :aria-label="selectionLabel"
          data-test="select-row"
          @change="emit('select', ($event.target as HTMLInputElement).checked)"
        />
      </label>
    </header>

    <div v-if="visible('key')" class="key-card-secret" data-field="key">
      <Cell field="key" />
    </div>
    <div v-if="visible('group')" class="key-card-group" data-field="group">
      <Cell field="group" />
    </div>

    <div v-if="visible('usage')" class="key-card-usage" data-field="usage">
      <Cell field="usage" />
    </div>

    <dl v-if="details.length" v-show="detailsOpen" :id="detailsId" class="key-card-details">
      <div v-for="column in details" :key="column.key" :data-field="column.key">
        <dt>{{ column.label }}</dt>
        <dd><Cell :field="column.key" /></dd>
      </div>
    </dl>

    <div v-if="visible('actions') || details.length" class="key-card-actions">
      <div v-if="visible('actions')" class="key-card-primary-actions" data-field="actions">
        <Cell field="actions" />
      </div>
      <button
        v-if="details.length"
        type="button"
        class="key-card-details-toggle"
        :aria-expanded="detailsOpen"
        :aria-controls="detailsId"
        :aria-label="detailsToggleLabel"
        :title="detailsToggleLabel"
        @click="detailsOpen = !detailsOpen"
      >
        <Icon name="chevronDown" size="sm" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, type Slots } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import type { ApiKey } from '@/types'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { platformAccentColor } from '@/utils/platformColors'

const props = defineProps<{
  row: ApiKey
  columns: Column[]
  cells: Slots
  selectable: boolean
  selected: boolean
  selectionLabel: string
}>()

const emit = defineEmits<{ select: [checked: boolean] }>()
const { t } = useI18n()
const detailsOpen = ref(false)
const useMobileActions = useMediaQuery('(max-width: 767px)')
const detailsToggleLabel = computed(() => detailsOpen.value ? t('common.collapse') : t('common.details'))
const detailsId = computed(() => `key-details-${props.row.id}`)
const accentColor = computed(() => {
  if (props.row.group?.platform) return platformAccentColor(props.row.group.platform)
  if (props.row.custom_group) return platformAccentColor('composite')
  return '#94A3B8'
})
const primaryFields = new Set(['name', 'status', 'key', 'group', 'usage', 'actions'])
const visible = (field: string) => props.columns.some(column => column.key === field)
const details = computed(() => props.columns.filter(column => !primaryFields.has(column.key)))

// Reuse the table cells so card actions, formatting and quota controls stay in sync.
const Cell = ({ field }: { field: string }) => {
  const value = props.row[field as keyof ApiKey]
  const slot = props.cells[`cell-${field}`]
  if (slot) return slot({ row: props.row, value, expanded: false, mobile: useMobileActions.value })
  const column = props.columns.find(column => column.key === field)
  return String(column?.formatter ? column.formatter(value, props.row) : value ?? '')
}
</script>

<style scoped>
.key-card {
  --card-rule: rgb(148 163 184 / 12%);
  --card-muted: #677282;
  position: relative;
  min-width: 0;
  padding: 1rem 1rem 0.375rem;
  border: 1px solid rgb(148 163 184 / 24%);
  border-radius: 8px;
  background-color: rgb(255 255 255 / 96%);
  background-image: linear-gradient(125deg, rgb(255 255 255 / 82%) 15%, transparent 65%, color-mix(in srgb, var(--key-accent) 5%, transparent));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 4px rgb(15 23 42 / 4%);
  transition: border-color 180ms ease, background-color 180ms ease, box-shadow 180ms ease;
  animation: key-card-enter 280ms cubic-bezier(0.22, 1, 0.36, 1) backwards;
}

[data-mobile-table-row]:nth-child(2) .key-card { animation-delay: 25ms; }
[data-mobile-table-row]:nth-child(3) .key-card { animation-delay: 50ms; }
[data-mobile-table-row]:nth-child(4) .key-card { animation-delay: 75ms; }

.dark .key-card {
  --card-rule: rgb(255 255 255 / 7%);
  --card-muted: #a0a9b7;
  border-color: rgb(255 255 255 / 11%);
  background-color: #27292e;
  background-image: linear-gradient(125deg, rgb(255 255 255 / 4%), transparent 65%, color-mix(in srgb, var(--key-accent) 5%, transparent));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%), 0 2px 5px rgb(0 0 0 / 12%);
}

.key-card.is-selected {
  border-color: rgb(var(--brand-rgb) / 0.65);
  background-color: rgb(var(--brand-rgb) / 0.06);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 95%), 0 0 0 1px rgb(var(--brand-rgb) / 0.06), 0 2px 5px rgb(var(--brand-rgb) / 0.08);
}

.dark .key-card.is-selected {
  background-color: #262e3a;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 8%), 0 0 0 1px rgb(var(--brand-rgb) / 0.08), 0 2px 5px rgb(0 0 0 / 14%);
}

.key-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 1.75rem;
  margin-bottom: 0;
}

.key-card-name {
  flex: 1;
  min-width: 0;
  font-size: 0.9375rem;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.key-card-name :deep(.keys-name > span) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 600;
}
.key-card-name :deep(svg),
.key-card-header :deep(.badge) { flex-shrink: 0; }

.key-card-status { display: flex; align-items: center; flex-shrink: 0; }
.key-card-status :deep(.badge) {
  gap: 0.375rem;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  white-space: nowrap;
}
.key-card-status :deep(.badge::before) {
  content: '';
  width: 0.3125rem;
  height: 0.3125rem;
  flex-shrink: 0;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.65;
}
.key-card-select {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 2rem;
  min-height: 2rem;
  margin: 0 -0.5rem 0 0;
  cursor: pointer;
}

.key-card-secret {
  margin-right: -0.5rem;
}

.key-card-secret :deep(.keys-key-value) { justify-content: space-between; }
.key-card-secret :deep(code) {
  min-width: 0;
  padding: 0;
  background: transparent;
  color: var(--card-muted);
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}
.key-card-secret :deep(button) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 2.75rem;
  height: 2.75rem;
  border: 0;
  border-radius: 6px;
  background: transparent;
  box-shadow: none;
}

.key-card-group { margin-top: 0; }
.key-card-group :deep(.keys-group-selector) {
  width: 100%;
  margin: 0;
  min-height: 2.75rem;
  padding: 0.25rem 0;
  flex-wrap: nowrap;
  text-align: left;
}
.key-card-group :deep(.keys-group-selector > :first-child) { min-width: 0; }
.key-card-group :deep(.group-badge-wrap) {
  display: grid;
  flex: 1;
  grid-template-columns: 1rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.375rem 0.5rem;
  padding: 0;
  background: transparent;
  line-height: 1.25rem;
}
.key-card-group :deep([data-test="group-badge-name"] + span:not(.group-badge-peak)) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--card-muted);
  font-size: 0.75rem;
  font-weight: 500;
}
.key-card-group :deep(.group-badge-peak) { grid-column: 2 / -1; justify-self: start; }
.key-card-group :deep(.keys-group-selector > .keys-group-hint) { display: none; }
.key-card-group :deep(.keys-group-selector > svg) {
  flex-shrink: 0;
  margin-left: auto;
}
.key-card-group :deep([data-test="group-badge-name"]) {
  white-space: normal;
  overflow-wrap: anywhere;
  color: #202731;
}
.dark .key-card-group :deep([data-test="group-badge-name"]) { color: #e8ecf2; }

.key-card-usage {
  container: key-usage / inline-size;
  padding: 0.375rem 0 0.75rem;
}
.key-card-usage :deep(.keys-usage) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem 0.75rem;
}
.key-card-usage :deep(.keys-usage-period) {
  display: flex;
  min-width: 0;
  margin: 0;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.125rem 0.3125rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.key-card-usage :deep(.keys-usage-period > :first-child) { font-size: 0.75rem; white-space: nowrap; }
.key-card-usage :deep(.keys-usage-period:nth-child(2)) {
  justify-content: flex-end;
}
.key-card-usage :deep(.keys-usage-period > :last-child) {
  font-size: 0.875rem;
  font-weight: 600;
  line-height: 1.25rem;
  white-space: nowrap;
}
.key-card-usage :deep(.keys-quota) {
  grid-column: 1 / -1;
  margin: 0;
}
.key-card-usage :deep(.keys-quota > :first-child) { flex-wrap: wrap; }

@container key-usage (max-width: 280px) {
  .key-card-usage :deep(.keys-usage-period) { flex-direction: column; align-items: flex-start; }
  .key-card-usage :deep(.keys-usage-period:nth-child(2)) { align-items: flex-end; }
}

.key-card-details {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.875rem;
  margin-top: 0.75rem;
  font-size: 0.75rem;
  line-height: 1.25rem;
  color: var(--card-muted);
}
.key-card-details > div {
  display: flex;
  align-items: baseline;
  gap: 0.375rem;
  min-width: 0;
  max-width: 100%;
}
.key-card-details dt { flex-shrink: 0; }
.key-card-details dd { min-width: 0; overflow-wrap: anywhere; }
.key-card-details :deep(dd > span) { font-size: inherit; }
.key-card-details :deep([data-field="current_concurrency"] dd > span) {
  min-width: 0;
  padding: 0;
  background: none;
  box-shadow: none;
}
.key-card-details > [data-field="created_at"],
.key-card-details > [data-field="last_used_at"],
.key-card-details > [data-field="last_used_ip"],
.key-card-details > [data-field="rate_limit"] { width: 100%; }
.key-card-details > [data-field="rate_limit"] dd { flex: 1; }

.key-card-actions {
  display: flex;
  align-items: stretch;
  gap: 0.25rem;
  margin: 0;
  padding: 0.25rem 0 0;
  border-top: 1px solid var(--card-rule);
}
.key-card-primary-actions { flex: 1; min-width: 0; }
.key-card-details-toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  flex: 0 0 2.75rem;
  width: 2.75rem;
  margin-left: auto;
  color: var(--card-muted);
}
.key-card-details-toggle svg { transition: transform 180ms ease; }
.key-card-details-toggle[aria-expanded="true"] svg { transform: rotate(180deg); }
.key-card-actions :deep(.keys-row-actions) {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.25rem;
}
.key-card-actions :deep(button) {
  flex-direction: row;
  justify-content: center;
  min-width: 0;
  min-height: 2.75rem;
  padding: 0.375rem 0.125rem;
  gap: 0.25rem;
  border-radius: 6px;
  transition: background-color 160ms ease, color 160ms ease, transform 160ms ease;
}
.dark .key-card-actions :deep(button) { color: #b5bdc9; }
.key-card-actions :deep(button > svg) { flex-shrink: 0; }
.key-card-actions :deep(button > span) { overflow-wrap: anywhere; font-size: 0.75rem; }
.key-card-actions :deep(button:active) { transform: scale(0.96); }
.key-card-secret :deep(button) { transition: background-color 160ms ease, transform 160ms ease; }
.key-card-secret :deep(button:active) { transform: scale(0.94); }

.key-card :deep(button:focus-visible) {
  outline: 2px solid rgb(var(--brand-rgb) / 0.7);
  outline-offset: 2px;
}

@media (hover: hover) {
  .key-card-details-toggle:hover { color: rgb(var(--brand-rgb)); }
  .key-card:hover:not(.is-selected) {
    border-color: color-mix(in srgb, var(--key-accent) 25%, #d1d5db);
    box-shadow: inset 0 1px 0 #fff, 0 4px 10px rgb(15 23 42 / 6%);
  }
  .dark .key-card:hover:not(.is-selected) {
    border-color: color-mix(in srgb, var(--key-accent) 25%, #52525b);
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 8%), 0 4px 10px rgb(0 0 0 / 16%);
  }
}

@keyframes key-card-enter {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
  .key-card,
  .key-card-details-toggle svg,
  .key-card :deep(button) { animation: none; transition: none; }
  .key-card :deep(button:active) { transform: none; }
}
</style>
