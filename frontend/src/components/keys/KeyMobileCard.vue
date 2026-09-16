<template>
  <article class="key-card" :class="{ 'is-selected': selected }">
    <header class="key-card-header">
      <div class="key-card-name"><Cell field="name" /></div>
      <Cell v-if="visible('status')" field="status" />
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

    <dl v-if="details.length" class="key-card-details">
      <div v-for="column in details" :key="column.key" :data-field="column.key">
        <dt>{{ column.label }}</dt>
        <dd><Cell :field="column.key" /></dd>
      </div>
    </dl>

    <div v-if="visible('actions')" class="key-card-actions" data-field="actions">
      <Cell field="actions" />
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, type Slots } from 'vue'
import type { ApiKey } from '@/types'
import type { Column } from '@/components/common/types'

const props = defineProps<{
  row: ApiKey
  columns: Column[]
  cells: Slots
  selectable: boolean
  selected: boolean
  selectionLabel: string
}>()

const emit = defineEmits<{ select: [checked: boolean] }>()
const primaryFields = new Set(['name', 'status', 'key', 'group', 'usage', 'actions'])
const visible = (field: string) => props.columns.some(column => column.key === field)
const details = computed(() => props.columns.filter(column => !primaryFields.has(column.key)))

// Reuse the table cells so card actions, formatting and quota controls stay in sync.
const Cell = ({ field }: { field: string }) => {
  const value = props.row[field as keyof ApiKey]
  const slot = props.cells[`cell-${field}`]
  if (slot) return slot({ row: props.row, value, expanded: false })
  const column = props.columns.find(column => column.key === field)
  return String(column?.formatter ? column.formatter(value, props.row) : value ?? '')
}
</script>

<style scoped>
.key-card {
  --card-rule: #edf0f2;
  --card-muted: #6b7280;
  --card-well: #f5f6f8;
  min-width: 0;
  padding: 1rem;
  border: 1px solid #e1e5ea;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 2px 4px rgb(0 0 0 / 2%);
}

.dark .key-card {
  --card-rule: #30343b;
  --card-muted: #9ca3af;
  --card-well: #24272e;
  border-color: #353942;
  background: #1b1e24;
}

.key-card.is-selected {
  border-color: rgb(var(--brand-rgb) / 0.65);
  box-shadow: 0 0 0 1px rgb(var(--brand-rgb) / 0.15);
}

.key-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 1.75rem;
  margin-bottom: 0.75rem;
}

.key-card-name {
  flex: 1;
  min-width: 0;
  font-size: 1rem;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.key-card-name :deep(.keys-name > span) { font-weight: 600; }
.key-card-name :deep(svg),
.key-card-header :deep(.badge) { flex-shrink: 0; }

.key-card-select {
  display: flex;
  align-items: center;
  justify-content: center;
  align-self: stretch;
  flex: 0 0 2rem;
  margin: -0.375rem -0.5rem -0.375rem 0;
  cursor: pointer;
}

.key-card-secret {
  padding: 0.25rem 0.5rem 0.25rem 0.75rem;
  border-radius: 6px;
  background: var(--card-well);
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
  flex: 0 0 2rem;
  height: 2rem;
}

.key-card-group { margin-top: 0.625rem; }
.key-card-group :deep(.keys-group-selector) {
  width: 100%;
  margin: 0;
  padding: 0.25rem 0;
  flex-wrap: nowrap;
  text-align: left;
}
.key-card-group :deep(.keys-group-selector > :first-child) { min-width: 0; }
.key-card-group :deep(.keys-group-selector > .keys-group-hint) { display: none; }
.key-card-group :deep(.keys-group-selector > svg) {
  flex-shrink: 0;
  margin-left: auto;
}
.key-card-group :deep([data-test="group-badge-name"]) {
  white-space: normal;
  overflow-wrap: anywhere;
}

.key-card-usage { padding-block: 1rem 0.875rem; }
.key-card-usage :deep(.keys-usage) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.625rem 1rem;
}
.key-card-usage :deep(.keys-usage-period) {
  display: flex;
  min-width: 0;
  margin: 0;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.25rem;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.key-card-usage :deep(.keys-usage-period > :first-child) { font-size: 0.75rem; }
.key-card-usage :deep(.keys-usage-period > :last-child) {
  font-size: 1.0625rem;
  font-weight: 600;
  line-height: 1.5rem;
}
.key-card-usage :deep(.keys-quota) {
  grid-column: 1 / -1;
  margin: 0;
}
.key-card-usage :deep(.keys-quota > :first-child) { flex-wrap: wrap; }

.key-card-details {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  margin-top: 0.25rem;
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
  margin-top: 0.875rem;
  padding-top: 0.625rem;
  border-top: 1px solid var(--card-rule);
}
.key-card-actions :deep(.keys-row-actions) {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr);
  grid-auto-flow: column;
  grid-auto-columns: minmax(0, 1fr);
  gap: 0.125rem;
}
.key-card-actions :deep(button) {
  flex-direction: row;
  justify-content: center;
  min-width: 0;
  min-height: 2.5rem;
  padding: 0.375rem 0.125rem;
  gap: 0.375rem;
  border-radius: 6px;
}
.key-card-actions :deep(button:first-child) {
  background: rgb(var(--brand-rgb) / 0.08);
  color: rgb(var(--brand-rgb));
}
.dark .key-card-actions :deep(button) { color: #b5bdc9; }
.dark .key-card-actions :deep(button:first-child) {
  background: rgb(96 165 250 / 12%);
  color: #93c5fd;
}
.key-card-actions :deep(button:not(:first-child) > span) { display: none; }
.key-card-actions :deep(button > svg) { flex-shrink: 0; }
.key-card-actions :deep(button > span) { overflow-wrap: anywhere; }
</style>
