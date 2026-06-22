<template>
  <section class="image-studio-settings space-y-6" data-test="image-studio-settings-panel">
    <div class="admin-toolbar-surface">
      <div class="admin-toolbar">
        <div class="admin-toolbar-group flex-1">
          <span
            class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="form.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'"
          >
            {{ form.enabled ? t('admin.settings.imageStudio.enabledStatus') : t('admin.settings.imageStudio.disabledStatus') }}
          </span>
          <span class="admin-page-meta-chip">
            <span>{{ t('admin.settings.imageStudio.models') }}</span>
            <strong>{{ form.allowed_models.length }}</strong>
          </span>
          <span class="admin-page-meta-chip">
            <span>{{ t('admin.settings.imageStudio.storage') }}</span>
            <strong>{{ form.storage_driver.toUpperCase() }}</strong>
          </span>
        </div>

        <div class="admin-toolbar-group w-full justify-end lg:w-auto lg:flex-none">
          <button
            type="button"
            class="btn btn-secondary inline-flex items-center gap-2"
            :disabled="loading || testingStorage || saving"
            data-test="image-studio-test-storage"
            @click="handleTestStorage"
          >
            <Icon name="cloud" size="sm" :class="testingStorage ? 'animate-pulse' : ''" />
            {{ testingStorage ? t('admin.settings.imageStudio.testingStorage') : t('admin.settings.imageStudio.testStorage') }}
          </button>
          <button
            type="button"
            class="btn btn-primary inline-flex items-center gap-2"
            :disabled="loading || saving"
            data-test="image-studio-save"
            @click="handleSave"
          >
            <Icon name="check" size="sm" />
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="loading" class="admin-surface flex items-center justify-center p-10">
      <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
    </div>

    <template v-else>
      <section class="grid gap-6 xl:grid-cols-[minmax(280px,360px),1fr]">
        <div class="admin-surface overflow-hidden">
          <div class="admin-panel-header">
            <div class="flex items-center gap-3">
              <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
                <Icon name="sparkles" size="sm" />
              </div>
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.settings.imageStudio.basicTitle') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.settings.imageStudio.basicHint') }}
                </p>
              </div>
            </div>
          </div>

          <div class="space-y-5 p-5">
            <label
              class="admin-form-section flex cursor-pointer items-center justify-between gap-4 !space-y-0 px-4 py-3 transition-colors"
              :class="form.enabled ? 'border-primary-200 bg-primary-50/70 dark:border-primary-500/30 dark:bg-primary-900/10' : ''"
            >
              <span>
                <span class="block text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.settings.imageStudio.enabled') }}
                </span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.settings.imageStudio.enabledHint') }}
                </span>
              </span>
              <input
                v-model="form.enabled"
                data-test="image-studio-enabled"
                type="checkbox"
                class="h-5 w-5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
            </label>

            <div>
              <label class="input-label">{{ t('admin.settings.imageStudio.defaultModel') }}</label>
              <input
                v-model.trim="form.default_model"
                data-test="image-studio-default-model"
                type="text"
                class="input"
                placeholder="gpt-image-1"
              />
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.settings.imageStudio.defaultModelHint') }}
              </p>
            </div>

            <div>
              <label class="input-label">{{ t('admin.settings.imageStudio.allowedModels') }}</label>
              <textarea
                v-model="modelsText"
                data-test="image-studio-models"
                rows="5"
                class="input font-mono text-sm"
                placeholder="gpt-image-1"
              ></textarea>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.settings.imageStudio.allowedModelsHint') }}
              </p>
            </div>
          </div>
        </div>

        <div class="admin-surface overflow-hidden">
          <div class="admin-panel-header">
            <div class="flex items-center gap-3">
              <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-50 text-cyan-600 dark:bg-cyan-900/20 dark:text-cyan-300">
                <Icon name="dollar" size="sm" />
              </div>
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.settings.imageStudio.billingTitle') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.settings.imageStudio.billingHint') }}
                </p>
              </div>
            </div>
          </div>

          <div class="grid gap-4 p-5 md:grid-cols-3">
            <div>
              <label class="input-label">{{ t('admin.settings.imageStudio.retentionDays') }}</label>
              <input v-model.number="form.retention_days" type="number" min="1" step="1" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.settings.imageStudio.maxImagesPerUser') }}</label>
              <input v-model.number="form.max_images_per_user" type="number" min="1" step="1" class="input" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.settings.imageStudio.maxReferenceImageMB') }}</label>
              <input v-model.number="form.max_reference_image_mb" type="number" min="1" step="1" class="input" />
            </div>
          </div>

          <div class="px-5 pb-5">
            <div class="image-studio-ratio-table overflow-hidden rounded-2xl border border-blue-100/80 dark:border-blue-400/10">
              <div class="grid grid-cols-[1fr,1.2fr,1fr] bg-blue-50/70 px-4 py-2 text-xs font-semibold uppercase text-blue-700 dark:bg-blue-950/30 dark:text-blue-200">
                <span>{{ t('admin.settings.imageStudio.ratio') }}</span>
                <span>{{ t('admin.settings.imageStudio.size') }}</span>
                <span>{{ t('admin.settings.imageStudio.billingTier') }}</span>
              </div>
              <div
                v-for="item in form.aspect_ratios"
                :key="item.ratio"
                class="grid grid-cols-[1fr,1.2fr,1fr] border-t border-blue-100/70 px-4 py-3 text-sm dark:border-blue-400/10"
              >
                <strong class="text-gray-900 dark:text-white">{{ item.ratio }}</strong>
                <span class="font-mono text-gray-600 dark:text-dark-300">{{ item.size }}</span>
                <span class="font-semibold text-primary-600 dark:text-primary-300">{{ item.billing_tier }}</span>
              </div>
            </div>
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.settings.imageStudio.aspectRatioHint') }}
            </p>
          </div>
        </div>
      </section>

      <section class="admin-surface overflow-hidden">
        <div class="admin-panel-header">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-indigo-50 text-indigo-600 dark:bg-indigo-900/20 dark:text-indigo-300">
              <Icon name="cloud" size="sm" />
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.settings.imageStudio.storageTitle') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.settings.imageStudio.storageHint') }}
              </p>
            </div>
          </div>
          <span
            class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="storageStatusClass"
          >
            {{ storageStatusLabel }}
          </span>
        </div>

        <div class="grid gap-5 p-5 lg:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.settings.imageStudio.storageDriver') }}</label>
            <select
              v-model="form.storage_driver"
              data-test="image-studio-storage-driver"
              class="input"
            >
              <option value="local">{{ t('admin.settings.imageStudio.storageDrivers.local') }}</option>
              <option value="r2">{{ t('admin.settings.imageStudio.storageDrivers.r2') }}</option>
            </select>
          </div>

          <div v-if="form.storage_driver === 'local'">
            <label class="input-label">{{ t('admin.settings.imageStudio.localPublicBaseURL') }}</label>
            <input
              v-model.trim="form.local_public_base_url"
              type="url"
              class="input"
              placeholder="https://your-domain.com/assets/images"
            />
          </div>

          <div v-if="form.storage_driver === 'local'" class="lg:col-span-2">
            <label class="input-label">{{ t('admin.settings.imageStudio.localRootDir') }}</label>
            <input
              v-model.trim="form.local_root_dir"
              type="text"
              class="input"
              placeholder="/var/lib/sub2api/images"
            />
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.settings.imageStudio.localRootDirHint') }}
            </p>
          </div>

          <div v-if="form.storage_driver === 'r2'" class="lg:col-span-2">
            <label class="input-label">{{ t('admin.settings.imageStudio.r2PublicBaseURL') }}</label>
            <input
              v-model.trim="form.r2_public_base_url"
              data-test="image-studio-r2-public-base-url"
              type="url"
              class="input"
              placeholder="https://images.example.com"
            />
            <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.settings.imageStudio.r2PublicBaseURLHint') }}
            </p>
          </div>

          <div class="admin-form-section !space-y-2 lg:col-span-2">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.settings.imageStudio.storageStatusTitle') }}
                </h4>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ storageStatus?.message || t('admin.settings.imageStudio.storageStatusHint') }}
                </p>
              </div>
              <span class="font-mono text-xs text-gray-500 dark:text-dark-400">
                {{ storageStatus?.driver || form.storage_driver }}
              </span>
            </div>
          </div>
        </div>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api'
import type {
  ImageStudioAspectRatio,
  ImageStudioSettings,
  ImageStudioStorageStatus,
} from '@/api/admin/settings'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const defaultAspectRatios: ImageStudioAspectRatio[] = [
  { ratio: '1:1', size: '1024x1024', billing_tier: '1k' },
  { ratio: '16:9', size: '1536x864', billing_tier: '2k' },
  { ratio: '9:16', size: '864x1536', billing_tier: '2k' },
  { ratio: '4:3', size: '1024x768', billing_tier: '1k' },
  { ratio: '3:4', size: '768x1024', billing_tier: '1k' },
]

const form = reactive<ImageStudioSettings>({
  enabled: false,
  allowed_models: ['gpt-image-1'],
  default_model: 'gpt-image-1',
  storage_driver: 'local',
  local_root_dir: '',
  local_public_base_url: '',
  r2_public_base_url: '',
  retention_days: 30,
  max_images_per_user: 100,
  max_reference_image_mb: 20,
  aspect_ratios: defaultAspectRatios,
})

const loading = ref(true)
const saving = ref(false)
const testingStorage = ref(false)
const modelsText = ref('gpt-image-1')
const storageStatus = ref<ImageStudioStorageStatus | undefined>()

const storageStatusLabel = computed(() => {
  const status = storageStatus.value?.status || 'untested'
  return t(`admin.settings.imageStudio.storageStatus.${status}`)
})

const storageStatusClass = computed(() => {
  const status = storageStatus.value?.status
  if (status === 'ok') {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
  }
  if (status === 'failed' || status === 'misconfigured') {
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'
  }
  return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
})

function parseModels(text: string): string[] {
  const seen = new Set<string>()
  return text
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item) => {
      if (seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function positiveInteger(value: unknown, fallback: number): number {
  const normalized = Math.floor(Number(value))
  return Number.isFinite(normalized) && normalized > 0 ? normalized : fallback
}

function applySettings(settings: ImageStudioSettings): void {
  Object.assign(form, {
    ...settings,
    allowed_models: settings.allowed_models?.length ? [...settings.allowed_models] : ['gpt-image-1'],
    aspect_ratios: settings.aspect_ratios?.length ? [...settings.aspect_ratios] : defaultAspectRatios,
    storage_driver: settings.storage_driver || 'local',
  })
  modelsText.value = form.allowed_models.join('\n')
  storageStatus.value = settings.storage_status
}

function buildPayload(): ImageStudioSettings {
  const allowedModels = parseModels(modelsText.value)
  const models = allowedModels.length ? allowedModels : ['gpt-image-1']
  const defaultModel = form.default_model.trim() && models.includes(form.default_model.trim())
    ? form.default_model.trim()
    : models[0]

  return {
    ...form,
    enabled: Boolean(form.enabled),
    allowed_models: models,
    default_model: defaultModel,
    storage_driver: form.storage_driver === 'r2' ? 'r2' : 'local',
    local_root_dir: form.local_root_dir?.trim() || '',
    local_public_base_url: form.local_public_base_url?.trim().replace(/\/+$/, '') || '',
    r2_public_base_url: form.r2_public_base_url?.trim().replace(/\/+$/, '') || '',
    retention_days: positiveInteger(form.retention_days, 30),
    max_images_per_user: positiveInteger(form.max_images_per_user, 100),
    max_reference_image_mb: positiveInteger(form.max_reference_image_mb, 20),
    aspect_ratios: form.aspect_ratios?.length ? form.aspect_ratios : defaultAspectRatios,
  }
}

async function loadSettings(): Promise<void> {
  loading.value = true
  try {
    const settings = await adminAPI.settings.getImageStudioSettings()
    applySettings(settings)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.imageStudio.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function handleSave(): Promise<void> {
  saving.value = true
  try {
    const updated = await adminAPI.settings.updateImageStudioSettings(buildPayload())
    applySettings(updated)
    appStore.showSuccess(t('admin.settings.imageStudio.saveSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.imageStudio.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function handleTestStorage(): Promise<void> {
  testingStorage.value = true
  try {
    storageStatus.value = await adminAPI.settings.testImageStudioStorage()
    appStore.showSuccess(t('admin.settings.imageStudio.storageTestSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.imageStudio.storageTestFailed')))
  } finally {
    testingStorage.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.image-studio-ratio-table {
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(248, 250, 252, 0.78)),
    rgba(255, 255, 255, 0.84);
}

:global(.dark) .image-studio-ratio-table {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.82), rgba(2, 6, 23, 0.78)),
    rgba(15, 23, 42, 0.72);
}
</style>
