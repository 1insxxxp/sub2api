<template>
  <div class="gateway-errors">
    <div v-if="loading" class="py-8 text-center text-sm text-gray-500" role="status">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="loadError" class="space-y-2 py-5 text-sm" role="alert" data-testid="gateway-load-error">
      <p class="font-medium">{{ t('admin.publicGatewayErrors.failedToLoad') }}</p>
      <p class="text-gray-500 dark:text-gray-400">{{ loadError }}</p>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="gateway-retry" @click="loadSettings">
        {{ t('common.retry') }}
      </button>
    </div>

    <fieldset v-else-if="settings" :disabled="saving" class="min-w-0 space-y-4">
      <div class="gateway-toolbar">
        <div class="min-w-0 space-y-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
          <p>{{ t('admin.publicGatewayErrors.description') }}</p>
          <p>{{ t('admin.publicGatewayErrors.orderHint') }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <span v-if="dirty" role="status" class="text-xs text-amber-700 dark:text-amber-300">
            {{ t('admin.publicGatewayErrors.unsaved') }}
          </span>
          <button type="button" class="btn btn-primary btn-sm" :disabled="saving || !dirty" data-testid="gateway-save" @click="save">
            {{ saving ? t('common.saving') : t('admin.publicGatewayErrors.saveAll') }}
          </button>
        </div>
      </div>
      <p v-if="saveError" role="alert" class="text-sm text-red-600 dark:text-red-400" data-testid="gateway-save-error">{{ saveError }}</p>

      <section :aria-label="t('admin.publicGatewayErrors.rules')">
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-sm font-medium">{{ t('admin.publicGatewayErrors.rules') }} <span class="ml-1 text-xs font-normal text-gray-500">{{ settings.rules.length }}/100</span></h4>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="settings.rules.length >= 100" data-testid="gateway-add" @click="createRule">
            <Icon name="plus" size="sm" class="mr-1" />{{ t('admin.publicGatewayErrors.createRule') }}
          </button>
        </div>
        <p v-if="settings.rules.length === 0" class="py-5 text-center text-sm text-gray-500">{{ t('admin.publicGatewayErrors.empty') }}</p>
        <div v-else class="gateway-rule-list">
          <div class="gateway-rule gateway-rule-heading" aria-hidden="true">
            <span>{{ t('admin.publicGatewayErrors.enabled') }}</span>
            <span>{{ t('admin.publicGatewayErrors.priority') }}</span>
            <span>{{ t('admin.publicGatewayErrors.name') }}</span>
            <span>{{ t('admin.publicGatewayErrors.sourceStatuses') }}</span>
            <span>{{ t('admin.publicGatewayErrors.responseStatus') }}</span>
            <span>{{ t('admin.publicGatewayErrors.message') }}</span>
            <span class="text-right">{{ t('common.actions') }}</span>
          </div>
          <ul>
            <li v-for="rule in settings.rules" :key="rule.id" class="gateway-rule" :data-rule-id="rule.id">
              <div class="gateway-rule-enabled">
                <button type="button" role="switch" :aria-checked="rule.enabled" :aria-label="t('admin.publicGatewayErrors.toggleRule', { name: rule.name })"
                  class="gateway-icon-button" @click="rule.enabled = !rule.enabled">
                  <span class="gateway-switch" :class="{ 'gateway-switch-on': rule.enabled }"><span /></span>
                </button>
              </div>
              <div class="gateway-rule-priority text-xs tabular-nums">
                <span class="gateway-mobile-label">{{ t('admin.publicGatewayErrors.priority') }}</span>{{ rule.priority }}
              </div>
              <div class="gateway-rule-name min-w-0">
                <p class="break-words text-sm font-medium">{{ rule.name }}</p>
                <p v-if="rule.pattern" class="mt-0.5 truncate font-mono text-xs text-gray-500 dark:text-gray-400" :title="rule.pattern">{{ rule.pattern }}</p>
              </div>
              <div class="gateway-rule-source min-w-0 text-xs tabular-nums">
                <span class="gateway-mobile-label">{{ t('admin.publicGatewayErrors.sourceStatuses') }}</span>
                <span class="break-words">{{ rule.upstream_statuses.length ? rule.upstream_statuses.join(', ') : t('admin.publicGatewayErrors.allStatuses') }}</span>
              </div>
              <div class="gateway-rule-status text-xs tabular-nums">
                <span class="gateway-mobile-label">{{ t('admin.publicGatewayErrors.responseStatus') }}</span>{{ rule.status }}
              </div>
              <div class="gateway-rule-message min-w-0">
                <code class="break-all text-xs text-gray-500 dark:text-gray-400">{{ rule.code }}</code>
                <p class="mt-0.5 whitespace-pre-wrap break-words text-sm">{{ rule.message }}</p>
              </div>
              <div class="gateway-rule-actions flex justify-end">
                <button type="button" class="gateway-icon-button" :aria-label="t('admin.publicGatewayErrors.editRuleName', { name: rule.name })" :title="t('common.edit')"
                  :data-testid="`edit-rule-${rule.id}`" @click="editRule(rule)"><Icon name="edit" size="sm" /></button>
                <button type="button" class="gateway-icon-button hover:text-red-600 dark:hover:text-red-400" :aria-label="t('admin.publicGatewayErrors.deleteRuleName', { name: rule.name })" :title="t('common.delete')"
                  :data-testid="`delete-rule-${rule.id}`" @click="settings.rules = settings.rules.filter(item => item.id !== rule.id)"><Icon name="trash" size="sm" /></button>
              </div>
            </li>
          </ul>
        </div>
      </section>

      <section class="gateway-section" :aria-label="t('admin.publicGatewayErrors.defaults')">
        <h4 class="mb-2 text-sm font-medium">{{ t('admin.publicGatewayErrors.defaults') }}</h4>
        <div class="grid gap-4 sm:grid-cols-2">
          <fieldset v-for="section in fallbackSections" :key="section.key" class="min-w-0 space-y-2" :data-testid="section.key">
            <legend class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-300">{{ section.label }}</legend>
            <div class="grid grid-cols-[5.5rem_minmax(0,1fr)] gap-2">
              <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.responseStatus') }}</span>
                <input v-model.number="settings[section.key].status" name="status" type="number" min="400" max="599" step="1" class="input text-sm" required />
              </label>
              <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.code') }}</span>
                <input v-model="settings[section.key].code" name="code" type="text" class="input font-mono text-sm" required />
              </label>
            </div>
            <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.message') }}</span>
              <textarea v-model="settings[section.key].message" name="message" rows="2" class="input text-sm" required />
            </label>
          </fieldset>
        </div>
      </section>

      <section class="gateway-section" :aria-label="t('admin.publicGatewayErrors.preview.title')">
        <h4 class="mb-2 text-sm font-medium">{{ t('admin.publicGatewayErrors.preview.title') }}</h4>
        <form class="space-y-2" data-testid="gateway-preview-form" @submit.prevent="runPreview">
          <div class="grid grid-cols-2 gap-2 sm:max-w-md">
            <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.preview.status') }}</span>
              <input v-model.number="previewForm.status" type="number" min="0" max="599" step="1" class="input text-sm" required aria-describedby="gateway-preview-status-hint" data-testid="gateway-preview-status" />
            </label>
            <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.preview.format') }}</span>
              <select v-model="previewForm.format" class="input text-sm" data-testid="gateway-preview-format">
                <option value="http">HTTP JSON</option><option value="sse">SSE</option><option value="responses">Responses SSE</option><option value="gemini">Gemini JSON</option>
              </select>
            </label>
          </div>
          <p id="gateway-preview-status-hint" class="input-hint">{{ t('admin.publicGatewayErrors.preview.unknownStatusHint') }}</p>
          <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.preview.diagnostic') }}</span>
            <textarea v-model="previewForm.diagnostic" rows="3" class="input font-mono text-sm" data-testid="gateway-preview-diagnostic" />
          </label>
          <div class="flex flex-wrap items-center gap-2">
            <button type="submit" class="btn btn-secondary btn-sm" :disabled="previewing">{{ previewing ? t('common.loading') : t('admin.publicGatewayErrors.preview.run') }}</button>
            <p v-if="previewForm.format === 'sse' || previewForm.format === 'responses'" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.publicGatewayErrors.preview.streamHint') }}</p>
          </div>
        </form>
        <p v-if="previewError" role="alert" class="mt-2 text-sm text-red-600 dark:text-red-400" data-testid="gateway-preview-error">{{ previewError }}</p>
        <div v-if="previewResult" class="mt-3 space-y-2" aria-live="polite" data-testid="gateway-preview-result">
          <dl class="flex flex-wrap gap-x-5 gap-y-1 text-xs">
            <div class="flex gap-2"><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.publicGatewayErrors.preview.matchedRule') }}</dt><dd>{{ previewResult.rule_name || previewResult.rule_id || t('admin.publicGatewayErrors.preview.defaultMatch') }}</dd></div>
            <div class="flex gap-2"><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.publicGatewayErrors.preview.mappedStatus') }}</dt><dd>{{ previewResult.status }}</dd></div>
          </dl>
          <pre class="gateway-payload text-xs">{{ JSON.stringify(previewResult.payload, null, 2) }}</pre>
        </div>
      </section>
    </fieldset>

    <BaseDialog :show="editingRule !== null" :title="creating ? t('admin.publicGatewayErrors.createRule') : t('admin.publicGatewayErrors.editRule')" width="wide" appearance="neutral" :z-index="60" @close="editingRule = null">
      <form v-if="editingRule" id="public-gateway-rule-form" class="space-y-3" data-testid="gateway-rule-form" @submit.prevent="applyRule">
        <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_7rem]">
          <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.name') }}</span>
            <input v-model="editingRule.name" name="name" type="text" class="input text-sm" required />
          </label>
          <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.priority') }}</span>
            <input v-model.number="editingRule.priority" name="priority" type="number" step="1" class="input text-sm" required />
          </label>
        </div>
        <label class="flex min-h-11 items-center gap-2 text-sm"><input v-model="editingRule.enabled" name="enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />{{ t('admin.publicGatewayErrors.enabled') }}</label>
        <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.sourceStatuses') }}</span>
          <input v-model="statusesInput" name="upstream_statuses" type="text" class="input font-mono text-sm" placeholder="400, 429, 503" aria-describedby="gateway-status-hint" />
        </label>
        <p id="gateway-status-hint" class="input-hint">{{ t('admin.publicGatewayErrors.statusesHint') }}</p>
        <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.pattern') }}</span>
          <input v-model="editingRule.pattern" name="pattern" type="text" class="input font-mono text-sm" aria-describedby="gateway-pattern-hint" />
        </label>
        <p id="gateway-pattern-hint" class="input-hint">{{ t('admin.publicGatewayErrors.patternHint') }}</p>
        <div class="grid grid-cols-[6rem_minmax(0,1fr)] gap-3">
          <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.responseStatus') }}</span>
            <input v-model.number="editingRule.status" name="status" type="number" min="400" max="599" step="1" class="input text-sm" required />
          </label>
          <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.code') }}</span>
            <input v-model="editingRule.code" name="code" type="text" class="input font-mono text-sm" required />
          </label>
        </div>
        <label class="block"><span class="input-label">{{ t('admin.publicGatewayErrors.message') }}</span>
          <textarea v-model="editingRule.message" name="message" rows="3" class="input text-sm" required />
        </label>
        <p v-if="ruleError" role="alert" class="text-sm text-red-600 dark:text-red-400" data-testid="gateway-rule-error">{{ ruleError }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" data-testid="gateway-rule-cancel" @click="editingRule = null">{{ t('common.cancel') }}</button>
          <button type="submit" form="public-gateway-rule-form" class="btn btn-primary">{{ t('admin.publicGatewayErrors.applyToDraft') }}</button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  publicGatewayErrorsAPI,
  type PublicGatewayErrorFormat,
  type PublicGatewayErrorOutput,
  type PublicGatewayErrorPreviewResult,
  type PublicGatewayErrorRule,
  type PublicGatewayErrorSettings
} from '@/api/admin/publicGatewayErrors'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const emit = defineEmits<{
  'dirty-change': [dirty: boolean]
  'saving-change': [saving: boolean]
}>()
const { t } = useI18n()
const appStore = useAppStore()
const settings = ref<PublicGatewayErrorSettings | null>(null)
const savedSnapshot = ref('')
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
const saveError = ref('')
const dirty = computed(() => settings.value !== null && JSON.stringify(settings.value) !== savedSnapshot.value)
watch(dirty, value => emit('dirty-change', value), { immediate: true })
watch(saving, value => emit('saving-change', value), { immediate: true })

const fallbackSections = computed(() => [
  { key: 'bad_request_fallback' as const, label: t('admin.publicGatewayErrors.badRequestFallback') },
  { key: 'fallback' as const, label: t('admin.publicGatewayErrors.otherFallback') }
])

function cloneSettings(value: PublicGatewayErrorSettings): PublicGatewayErrorSettings {
  return JSON.parse(JSON.stringify(value))
}

async function loadSettings() {
  if (loading.value) return
  loading.value = true
  loadError.value = ''
  try {
    const loaded = await publicGatewayErrorsAPI.getSettings()
    settings.value = cloneSettings(loaded)
    savedSnapshot.value = JSON.stringify(settings.value)
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.publicGatewayErrors.failedToLoad'))
  } finally {
    loading.value = false
  }
}
onMounted(loadSettings)

function validStatus(status: number): boolean {
  return Number.isInteger(status) && status >= 400 && status <= 599
}

function validateOutput(output: PublicGatewayErrorOutput): string {
  if (!/^[a-z][a-z0-9_]{0,63}$/.test(output.code)) return t('admin.publicGatewayErrors.validation.code')
  if (!validStatus(output.status) || !output.message.trim()) return t('admin.publicGatewayErrors.validation.output')
  return ''
}

function validateRule(rule: PublicGatewayErrorRule): string {
  if (!rule.name.trim()) return t('admin.publicGatewayErrors.validation.name')
  if (!Number.isSafeInteger(rule.priority)) return t('admin.publicGatewayErrors.validation.priority')
  if (!rule.upstream_statuses.every(validStatus)) return t('admin.publicGatewayErrors.validation.sourceStatuses')
  if (!rule.upstream_statuses.length && !rule.pattern.trim()) return t('admin.publicGatewayErrors.validation.conditions')
  return validateOutput(rule)
}

function validateSettings(value: PublicGatewayErrorSettings): string {
  if (value.rules.length > 100) return t('admin.publicGatewayErrors.validation.limit')
  for (const rule of value.rules) {
    const error = validateRule(rule)
    if (error) return `${rule.name} · ${error}`
  }
  return validateOutput(value.fallback) || validateOutput(value.bad_request_fallback)
}

async function save() {
  if (!settings.value || saving.value || !dirty.value) return
  saveError.value = validateSettings(settings.value)
  if (saveError.value) return
  saving.value = true
  try {
    const saved = await publicGatewayErrorsAPI.updateSettings(cloneSettings(settings.value))
    settings.value = cloneSettings(saved)
    savedSnapshot.value = JSON.stringify(settings.value)
    appStore.showSuccess(t('admin.publicGatewayErrors.saved'))
  } catch (error) {
    saveError.value = extractApiErrorMessage(error, t('admin.publicGatewayErrors.failedToSave'))
  } finally {
    saving.value = false
  }
}

const editingRule = ref<PublicGatewayErrorRule | null>(null)
const creating = ref(false)
const statusesInput = ref('')
const ruleError = ref('')

function createRule() {
  if (!settings.value || settings.value.rules.length >= 100) return
  creating.value = true
  statusesInput.value = ''
  ruleError.value = ''
  editingRule.value = {
    id: `rule_${crypto.randomUUID()}`,
    name: '',
    enabled: true,
    priority: 100,
    upstream_statuses: [],
    pattern: '',
    status: 502,
    code: 'upstream_error',
    message: ''
  }
}

function editRule(rule: PublicGatewayErrorRule) {
  creating.value = false
  statusesInput.value = rule.upstream_statuses.join(', ')
  ruleError.value = ''
  editingRule.value = { ...rule, upstream_statuses: [...rule.upstream_statuses] }
}

function applyRule() {
  if (!editingRule.value || !settings.value) return
  const tokens = statusesInput.value.trim() ? statusesInput.value.trim().split(/[,，\s]+/) : []
  if (tokens.some(value => !/^\d{3}$/.test(value) || !validStatus(Number(value)))) {
    ruleError.value = t('admin.publicGatewayErrors.validation.sourceStatuses')
    return
  }
  const rule = { ...editingRule.value, name: editingRule.value.name.trim(), upstream_statuses: [...new Set(tokens.map(Number))] }
  ruleError.value = validateRule(rule)
  if (ruleError.value) return
  if (creating.value) settings.value.rules.push(rule)
  else settings.value.rules = settings.value.rules.map(existing => existing.id === rule.id ? rule : existing)
  editingRule.value = null
}

const previewForm = reactive({ status: 502, diagnostic: '', format: 'http' as PublicGatewayErrorFormat })
const previewing = ref(false)
const previewError = ref('')
const previewResult = ref<PublicGatewayErrorPreviewResult | null>(null)
let previewRevision = 0
watch([settings, previewForm], () => {
  previewRevision++
  previewResult.value = null
  previewError.value = ''
}, { deep: true, flush: 'sync' })
onBeforeUnmount(() => { previewRevision++ })

async function runPreview() {
  if (!settings.value || previewing.value) return
  previewError.value = validateSettings(settings.value)
  if (previewForm.status !== 0 && !validStatus(previewForm.status)) previewError.value = t('admin.publicGatewayErrors.validation.previewStatus')
  if (previewError.value) return
  const revision = ++previewRevision
  previewing.value = true
  previewResult.value = null
  try {
    const result = await publicGatewayErrorsAPI.preview({ settings: cloneSettings(settings.value), ...previewForm })
    if (revision === previewRevision) previewResult.value = result
  } catch (error) {
    if (revision === previewRevision) previewError.value = extractApiErrorMessage(error, t('admin.publicGatewayErrors.preview.failed'))
  } finally {
    previewing.value = false
  }
}
</script>

<style scoped>
.gateway-errors { color: var(--workspace-ink); }
.gateway-toolbar { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.gateway-section { border-top: 1px solid var(--workspace-divider); padding-top: 14px; }
.gateway-rule-list { overflow: hidden; border: 1px solid var(--workspace-rule); border-radius: 7px; background: linear-gradient(125deg, var(--workspace-highlight), transparent 65%), var(--workspace-surface); box-shadow: var(--workspace-shadow); }
.gateway-rule { display: grid; grid-template-columns: 44px 48px minmax(100px, 1fr) minmax(85px, .7fr) 62px minmax(140px, 1.5fr) 88px; gap: 8px; align-items: center; padding: 6px 10px; }
.gateway-rule + .gateway-rule { border-top: 1px solid var(--workspace-divider); }
.gateway-rule-heading { min-height: 34px; color: var(--workspace-muted); background: var(--workspace-hover); font-size: 11px; }
.gateway-rule-list ul { max-height: 360px; overflow-y: auto; overscroll-behavior: contain; }
.gateway-icon-button { display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; flex-shrink: 0; border-radius: 5px; color: var(--workspace-muted); transition: background 120ms ease, color 120ms ease; }
.gateway-icon-button:hover { background: var(--workspace-hover); }
.gateway-icon-button:focus-visible { outline: 2px solid var(--workspace-focus); outline-offset: -2px; }
.gateway-switch { width: 28px; height: 16px; padding: 2px; border-radius: 999px; background: var(--workspace-muted); }
.gateway-switch > span { display: block; width: 12px; height: 12px; border-radius: 50%; background: white; transition: transform 120ms ease; }
.gateway-switch-on { background: rgb(var(--brand-rgb)); }
.gateway-switch-on > span { transform: translateX(12px); }
.gateway-mobile-label { display: none; }
.gateway-payload { max-height: 280px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; border: 1px solid var(--workspace-rule); border-radius: 6px; padding: 10px 12px; background: var(--workspace-control); }
@media (max-width: 767px) {
  .gateway-toolbar { flex-direction: column; gap: 10px; }
  .gateway-toolbar > :last-child { align-self: flex-end; }
  .gateway-rule-heading { display: none; }
  .gateway-rule { grid-template-columns: 44px minmax(0, 1fr) 88px; grid-template-areas: 'enabled name actions' '. priority actions' 'source source status' 'message message message'; gap: 3px 6px; padding: 8px 10px; }
  .gateway-rule-enabled { grid-area: enabled; }
  .gateway-rule-name { grid-area: name; }
  .gateway-rule-priority { grid-area: priority; }
  .gateway-rule-source { grid-area: source; }
  .gateway-rule-status { grid-area: status; }
  .gateway-rule-message { grid-area: message; padding: 4px 0; }
  .gateway-rule-actions { grid-area: actions; align-self: start; }
  .gateway-mobile-label { display: inline; margin-right: 6px; color: var(--workspace-muted); font-size: 11px; }
  .gateway-rule-list ul { max-height: 420px; }
}
@media (prefers-reduced-motion: reduce) {
  .gateway-icon-button, .gateway-switch > span { transition: none; }
}
</style>
