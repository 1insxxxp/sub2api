<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import type { CustomEndpoint } from '@/types'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  apiBaseUrl: string
  customEndpoints: CustomEndpoint[]
  inlineDetails?: boolean
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const copiedEndpoint = ref<string | null>(null)

let copiedResetTimer: number | undefined

const allEndpoints = computed(() => {
  const items: Array<{ name: string; endpoint: string; description: string; isDefault: boolean }> = []
  if (props.apiBaseUrl) {
    items.push({
      name: t('keys.endpoints.title'),
      endpoint: props.apiBaseUrl,
      description: '',
      isDefault: true,
    })
  }
  for (const ep of props.customEndpoints) {
    items.push({ ...ep, isDefault: false })
  }
  return items
})

async function copy(url: string) {
  const success = await copyToClipboard(url, false)
  if (!success) return

  copiedEndpoint.value = url
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
  copiedResetTimer = window.setTimeout(() => {
    if (copiedEndpoint.value === url) {
      copiedEndpoint.value = null
    }
  }, 1800)
}

function tooltipHint(endpoint: string): string {
  return copiedEndpoint.value === endpoint
    ? t('keys.endpoints.copiedHint')
    : t('keys.endpoints.clickToCopy')
}

function speedTestUrl(endpoint: string): string {
  return `https://www.tcptest.cn/http/${encodeURIComponent(endpoint)}`
}

onBeforeUnmount(() => {
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
})
</script>

<template>
  <div v-if="allEndpoints.length > 0" class="endpoint-popover" :class="{ 'endpoint-popover--inline': inlineDetails }">
    <div
      v-for="(item, index) in allEndpoints"
      :key="index"
      class="endpoint-item text-xs"
    >
      <div class="endpoint-content group/endpoint relative">
        <div class="endpoint-heading">
          <span class="endpoint-name font-semibold text-gray-700 dark:text-gray-200" :title="item.name">{{ item.name }}</span>
          <span
            v-if="item.isDefault"
            class="endpoint-default rounded bg-primary-50 px-1 py-px text-[10px] font-medium leading-tight text-primary-600 dark:bg-primary-900/30 dark:text-primary-400"
          >{{ t('keys.endpoints.default') }}</span>
          <span
            v-if="inlineDetails"
            role="status"
            class="shrink-0 text-[10px] font-medium text-emerald-600 dark:text-emerald-400"
          >{{ copiedEndpoint === item.endpoint ? t('keys.endpoints.copied') : '' }}</span>
        </div>
        <div
          v-if="!inlineDetails"
          class="endpoint-tooltip pointer-events-none absolute bottom-full left-0 z-20 mb-2 w-max max-w-full translate-y-1 rounded-lg border border-slate-200 bg-white px-3 py-2.5 text-left opacity-0 shadow-lg transition-all duration-150 group-hover/endpoint:translate-y-0 group-hover/endpoint:opacity-100 group-focus-within/endpoint:translate-y-0 group-focus-within/endpoint:opacity-100 dark:border-slate-700 dark:bg-dark-800"
        >
          <p
            v-if="item.description"
            class="max-w-[24rem] break-words text-xs leading-5 text-slate-600 dark:text-slate-200"
          >
            {{ item.description }}
          </p>
          <p
            class="flex items-center gap-1.5 text-[11px] leading-4 text-primary-600 dark:text-primary-300"
            :class="item.description ? 'mt-1.5' : ''"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-primary-500 dark:bg-primary-300"></span>
            {{ tooltipHint(item.endpoint) }}
          </p>
          <div class="absolute left-4 top-full h-3 w-3 -translate-y-1/2 rotate-45 border-b border-r border-slate-200 bg-white dark:border-slate-700 dark:bg-dark-800"></div>
        </div>

        <code
          class="endpoint-code min-w-0 cursor-pointer overflow-hidden text-ellipsis whitespace-nowrap font-mono text-gray-500 decoration-gray-400 decoration-dashed underline-offset-2 hover:text-primary-600 hover:underline focus:text-primary-600 focus:underline focus:outline-none dark:text-gray-400 dark:decoration-gray-500 dark:hover:text-primary-400 dark:focus:text-primary-400"
          role="button"
          tabindex="0"
          :aria-label="`${tooltipHint(item.endpoint)}: ${item.endpoint}`"
          @click="copy(item.endpoint)"
          @keydown.enter.prevent="copy(item.endpoint)"
          @keydown.space.prevent="copy(item.endpoint)"
        >{{ item.endpoint }}</code>

        <p
          v-if="inlineDetails && item.description"
          class="endpoint-description text-gray-500 dark:text-gray-400"
        >{{ item.description }}</p>

        <button
          type="button"
          class="endpoint-copy endpoint-action transition-colors"
          :class="copiedEndpoint === item.endpoint
            ? 'text-emerald-500 dark:text-emerald-400'
            : 'text-gray-400 hover:text-primary-500 dark:text-gray-500 dark:hover:text-primary-400'"
          :aria-label="tooltipHint(item.endpoint)"
          :title="tooltipHint(item.endpoint)"
          @click="copy(item.endpoint)"
        >
          <Icon :name="copiedEndpoint === item.endpoint ? 'check' : 'copy'" size="sm" />
        </button>

        <a
          :href="speedTestUrl(item.endpoint)"
          target="_blank"
          rel="noopener noreferrer"
          class="endpoint-speed endpoint-action text-gray-400 transition-colors hover:text-amber-500 dark:text-gray-500 dark:hover:text-amber-400"
          :title="t('keys.endpoints.speedTest')"
          :aria-label="t('keys.endpoints.speedTest')"
        >
          <Icon name="bolt" size="sm" />
        </a>
      </div>
    </div>
  </div>
</template>

<style scoped>
.endpoint-popover {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr));
  gap: 0.625rem;
  width: 100%;
  padding-top: 0.875rem;
  border-top: 1px solid rgb(148 163 184 / 18%);
}

.endpoint-item {
  min-width: 0;
  padding: 0.625rem 0.75rem;
  border: 1px solid rgb(148 163 184 / 24%);
  border-radius: 8px;
  background: linear-gradient(125deg, rgb(255 255 255 / 90%), rgb(248 250 252 / 80%));
  box-shadow: inset 0 1px 0 #fff, 0 2px 4px rgb(15 23 42 / 3%);
  transition: border-color 180ms ease;
}

.endpoint-content {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 2.25rem 2.25rem;
  grid-template-rows: auto auto;
  align-items: center;
  gap: 0.125rem 0.375rem;
  min-width: 0;
}

.endpoint-heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}
.endpoint-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.endpoint-default { flex-shrink: 0; }
.endpoint-code { display: block; grid-column: 1; grid-row: 2; font-size: 11px; line-height: 1rem; }
.endpoint-description {
  grid-column: 1 / -1;
  grid-row: 3;
  padding-top: 0.375rem;
  overflow-wrap: anywhere;
  font-size: 11px;
  line-height: 1.5;
}
.endpoint-copy { grid-column: 2; }
.endpoint-speed { grid-column: 3; }
.endpoint-action {
  display: inline-flex;
  grid-row: 1 / 3;
  width: 2.25rem;
  height: 2.25rem;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(148 163 184 / 18%);
  border-radius: 6px;
  background: rgb(255 255 255 / 72%);
  box-shadow: inset 0 1px 0 #fff, 0 1px 2px rgb(15 23 42 / 3%);
}
.endpoint-action:focus-visible,
.endpoint-code:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 0.5); outline-offset: 2px; }

.dark .endpoint-popover { border-color: rgb(255 255 255 / 8%); }
.dark .endpoint-item {
  border-color: rgb(255 255 255 / 12%);
  background: linear-gradient(125deg, rgb(255 255 255 / 4%), transparent), #27292e;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 6%), 0 2px 4px rgb(0 0 0 / 10%);
}
.dark .endpoint-action {
  border-color: rgb(255 255 255 / 10%);
  background: rgb(255 255 255 / 3%);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%);
}

.endpoint-popover--inline {
  grid-template-columns: minmax(0, 1fr);
  gap: 0;
  padding: 0;
  border: 0;
}
.endpoint-popover--inline .endpoint-item {
  padding: 0.625rem;
  border: 0;
  border-radius: 0;
  background: none;
  box-shadow: none;
}
.endpoint-popover--inline .endpoint-item + .endpoint-item {
  border-top: 1px solid rgb(148 163 184 / 18%);
}
.endpoint-popover--inline .endpoint-content {
  grid-template-columns: minmax(0, 1fr) 2.5rem 2.5rem;
  gap: 0.125rem 0.25rem;
}
.endpoint-popover--inline .endpoint-name { font-size: 13px; }
.endpoint-popover--inline .endpoint-code {
  grid-column: 1;
  color: #6b7280;
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 1.5;
  text-decoration: none;
}
.endpoint-popover--inline .endpoint-description { padding-top: 0.125rem; }
.endpoint-popover--inline .endpoint-action {
  grid-row: 1 / 3;
  width: 2.5rem;
  height: 2.5rem;
  border: 0;
  background: rgb(148 163 184 / 6%);
  box-shadow: none;
}
.endpoint-popover--inline .endpoint-action:hover,
.endpoint-popover--inline .endpoint-action:active { background: rgb(148 163 184 / 10%); }
.dark .endpoint-popover--inline .endpoint-code { color: #9ca3af; }
.dark .endpoint-popover--inline .endpoint-item + .endpoint-item { border-color: rgb(255 255 255 / 8%); }

@media (hover: hover) {
  .endpoint-item:hover { border-color: rgb(var(--brand-rgb) / 0.3); }
  .endpoint-action:hover { border-color: rgb(var(--brand-rgb) / 0.35); }
}

@media (prefers-reduced-motion: reduce) {
  .endpoint-item,
  .endpoint-action,
  .endpoint-tooltip { transition: none; }
}
</style>
