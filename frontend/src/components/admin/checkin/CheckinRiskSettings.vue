<template>
  <p v-if="loading" role="status">{{ t('common.loading') }}</p>
  <div v-else-if="error" role="alert" class="space-y-2">
    <p>{{ error }}</p>
    <button type="button" class="btn btn-secondary" @click="load">{{ t('common.refresh') }}</button>
  </div>
  <form v-else-if="settings" data-test="risk-settings-form" class="space-y-4" @submit.prevent="save">
    <fieldset :disabled="saving" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <label v-for="field in modeFields" :key="field">
        <span class="input-label">{{ tr(field) }}</span>
        <select v-model="settings[field]" :name="field" class="input">
          <option value="observe">{{ tr('observe') }}</option>
          <option value="enforce">{{ tr('enforce') }}</option>
        </select>
        <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ tr(field + 'Hint') }}</span>
      </label>
      <label v-for="field in numberFields" :key="field.key">
        <span class="input-label">{{ tr(field.key) }}</span>
        <input v-model.number="settings[field.key]" :name="field.key" required type="number" min="1" :max="field.max" step="1" class="input" />
      </label>
    </fieldset>
    <p v-if="hits" class="text-sm">{{ tr('hitsToday', hits) }}</p>
    <p v-if="settings.cooldown_count <= settings.challenge_count" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ tr('thresholdError') }}</p>
    <button type="submit" class="btn btn-primary" :disabled="saving || settings.cooldown_count <= settings.challenge_count">{{ t('common.save') }}</button>
  </form>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { checkinRisk, type RiskConfig, type RiskSettings } from '@/api/admin/checkinRisk'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const tr = (key: string, params = {}) => t('admin.checkinRisk.' + key, params)
const app = useAppStore()
const settings = ref<RiskSettings>()
const hits = ref<RiskConfig['registration_hits_today']>()
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const modeFields = ['registration_mode', 'checkin_mode'] as const
const numberFields = [
  { key: 'window_minutes', max: 1440 },
  { key: 'challenge_count', max: 10000 },
  { key: 'cooldown_count', max: 10000 },
  { key: 'cooldown_minutes', max: 10080 },
] as const

function applyConfig(config: RiskConfig) {
  const { registration_hits_today, ...editable } = config
  settings.value = editable
  if (registration_hits_today) hits.value = registration_hits_today
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    applyConfig(await checkinRisk.config())
  } catch (e) {
    error.value = extractApiErrorMessage(e, tr('loadError'))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!settings.value || saving.value || settings.value.cooldown_count <= settings.value.challenge_count) return
  saving.value = true
  try {
    applyConfig(await checkinRisk.saveConfig({ ...settings.value }))
    app.showSuccess(tr('saved'))
  } catch (e) {
    app.showError(extractApiErrorMessage(e, tr('saveError')))
  } finally {
    saving.value = false
  }
}
onMounted(load)
</script>
