<template>
  <AppLayout>
    <div class="image-studio-shell mx-auto w-full">
      <section v-if="loadError" class="image-studio-alert image-studio-alert-error">
        <Icon name="exclamationCircle" size="md" />
        <span>{{ loadError }}</span>
        <button type="button" class="image-studio-link-button" @click="loadInitialData">
          {{ t('imageStudio.retry') }}
        </button>
      </section>

      <div class="image-studio-grid">
        <form
          class="image-studio-workspace"
          data-testid="image-studio-submit"
          @submit.prevent="handleSubmit"
        >
          <div class="image-studio-panel-header">
            <div>
              <p class="image-studio-section-label">{{ t('imageStudio.creationPanel') }}</p>
              <h2>{{ t('imageStudio.promptTitle') }}</h2>
            </div>
            <div class="image-studio-mode-switch" role="tablist" :aria-label="t('imageStudio.mode')">
              <button
                type="button"
                :class="{ active: mode === 'generation' }"
                @click="mode = 'generation'"
              >
                <Icon name="sparkles" size="sm" />
                {{ t('imageStudio.textToImage') }}
              </button>
              <button type="button" :class="{ active: mode === 'edit' }" @click="mode = 'edit'">
                <Icon name="edit" size="sm" />
                {{ t('imageStudio.imageEdit') }}
              </button>
            </div>
          </div>

          <div v-if="config && !config.enabled" class="image-studio-disabled">
            <Icon name="lock" size="lg" />
            <div>
              <strong>{{ t('imageStudio.disabledTitle') }}</strong>
              <span>{{ t('imageStudio.disabledDescription') }}</span>
            </div>
          </div>

          <div v-else-if="options && groupOptions.length === 0" class="image-studio-disabled">
            <Icon name="lock" size="lg" />
            <div>
              <strong>{{ t('imageStudio.noGroupsTitle') }}</strong>
              <span>{{ t('imageStudio.noGroupsDescription') }}</span>
            </div>
          </div>

          <div v-else-if="options && availableAPIKeys.length === 0" class="image-studio-disabled">
            <Icon name="key" size="lg" />
            <div>
              <strong>{{ t('imageStudio.noApiKeysTitle') }}</strong>
              <span>{{ t('imageStudio.noApiKeysDescription') }}</span>
            </div>
          </div>

          <div class="image-studio-command-surface">
            <section class="image-studio-prompt-section">
              <label class="image-studio-field image-studio-prompt-field">
                <span>{{ t('imageStudio.promptLabel') }}</span>
                <textarea
                  v-model="prompt"
                  data-testid="image-studio-prompt"
                  :placeholder="t('imageStudio.promptPlaceholder')"
                  :disabled="submitting || !canUse"
                  rows="8"
                ></textarea>
              </label>
            </section>

            <section class="image-studio-control-dock">
              <div class="image-studio-foundation-row">
                <div class="image-studio-field image-studio-select-field">
                  <span>{{ t('imageStudio.apiKey') }}</span>
                  <div
                    class="image-studio-select"
                    :class="{ 'is-open': openSelect === 'apiKey', 'is-disabled': submitting || !canUse }"
                    @click.stop
                  >
                    <button
                      type="button"
                      class="image-studio-select-trigger"
                      data-testid="image-studio-api-key-select"
                      :disabled="submitting || !canUse"
                      aria-haspopup="listbox"
                      :aria-expanded="openSelect === 'apiKey'"
                      @click="toggleSelect('apiKey')"
                      @keydown.escape.stop="closeSelectMenus"
                    >
                      <span class="image-studio-select-value">{{ selectedAPIKeyLabel }}</span>
                      <Icon name="chevronDown" size="sm" />
                    </button>
                    <transition name="image-studio-select-pop">
                      <div
                        v-if="openSelect === 'apiKey'"
                        class="image-studio-select-menu"
                        data-testid="image-studio-api-key-menu"
                        role="listbox"
                      >
                        <button
                          v-for="item in availableAPIKeys"
                          :key="item.id"
                          type="button"
                          class="image-studio-select-option"
                          :class="{ 'is-selected': item.id === selectedAPIKeyID }"
                          role="option"
                          :aria-selected="item.id === selectedAPIKeyID"
                          @click="chooseAPIKey(item.id)"
                        >
                          <span>{{ apiKeyOptionLabel(item) }}</span>
                          <small>{{ apiKeyOptionMeta(item) }}</small>
                          <Icon v-if="item.id === selectedAPIKeyID" name="check" size="sm" />
                        </button>
                      </div>
                    </transition>
                  </div>
                  <small>{{ selectedAPIKeyMeta }}</small>
                </div>

                <div class="image-studio-field image-studio-select-field">
                  <span>{{ t('imageStudio.model') }}</span>
                  <div
                    class="image-studio-select"
                    :class="{ 'is-open': openSelect === 'model', 'is-disabled': submitting || !canUse }"
                    @click.stop
                  >
                    <button
                      type="button"
                      class="image-studio-select-trigger"
                      data-testid="image-studio-model-select"
                      :disabled="submitting || !canUse"
                      aria-haspopup="listbox"
                      :aria-expanded="openSelect === 'model'"
                      @click="toggleSelect('model')"
                      @keydown.escape.stop="closeSelectMenus"
                    >
                      <span class="image-studio-select-value">{{ selectedModelLabel || t('imageStudio.model') }}</span>
                      <Icon name="chevronDown" size="sm" />
                    </button>
                    <transition name="image-studio-select-pop">
                      <div
                        v-if="openSelect === 'model'"
                        class="image-studio-select-menu"
                        data-testid="image-studio-model-menu"
                        role="listbox"
                      >
                        <button
                          v-for="item in modelOptions"
                          :key="item.model"
                          type="button"
                          class="image-studio-select-option"
                          :class="{ 'is-selected': item.model === model }"
                          role="option"
                          :aria-selected="item.model === model"
                          @click="chooseModelOption(item.model)"
                        >
                          <span>{{ item.label || item.model }}</span>
                          <Icon v-if="item.model === model" name="check" size="sm" />
                        </button>
                      </div>
                    </transition>
                  </div>
                  <small>{{ selectedGroup?.platform || '-' }}</small>
                </div>
              </div>

              <div class="image-studio-output-row">
                <div class="image-studio-field image-studio-choice-section">
                  <span>{{ t('imageStudio.quality') }}</span>
                  <div class="image-studio-choice-picker image-studio-quality-picker">
                    <button
                      v-for="item in qualityOptions"
                      :key="item.quality"
                      type="button"
                      :data-testid="`image-studio-quality-${item.quality}`"
                      :class="{ active: quality === item.quality }"
                      :disabled="submitting || !canUse"
                      @click="quality = item.quality"
                    >
                      <strong>{{ item.label || item.quality }}</strong>
                      <small>{{ formatCurrency(item.estimated_cost || 0) }}</small>
                    </button>
                  </div>
                </div>

                <div class="image-studio-field image-studio-choice-section">
                  <span>{{ t('imageStudio.aspectRatio') }}</span>
                  <div class="image-studio-choice-picker image-studio-ratio-picker">
                    <button
                      v-for="item in aspectRatioOptions"
                      :key="item.ratio"
                      type="button"
                      :data-testid="`image-studio-ratio-${item.ratio}`"
                      :class="{ active: aspectRatio === item.ratio }"
                      :disabled="submitting || !canUse"
                      @click="aspectRatio = item.ratio"
                    >
                      <strong>{{ item.ratio }}</strong>
                      <small>{{ ratioPreviewLabel(item) }}</small>
                    </button>
                  </div>
                </div>
              </div>

              <transition name="image-studio-fade">
                <div v-if="mode === 'edit'" class="image-studio-upload-panel">
                  <input
                    ref="fileInput"
                    class="sr-only"
                    type="file"
                    accept="image/*"
                    multiple
                    @change="handleFileInput"
                  />
                  <button
                    type="button"
                    class="image-studio-dropzone"
                    :class="{ 'is-dragging': dragging }"
                    :disabled="submitting || !canUse"
                    @click="fileInput?.click()"
                    @dragenter.prevent="dragging = true"
                    @dragover.prevent="dragging = true"
                    @dragleave.prevent="dragging = false"
                    @drop.prevent="handleDrop"
                    @paste="handlePaste"
                  >
                    <Icon name="upload" size="lg" />
                    <span>{{ t('imageStudio.uploadTitle') }}</span>
                    <small>{{ t('imageStudio.uploadHint', { mb: maxReferenceImageMB }) }}</small>
                  </button>

                  <div v-if="referenceFiles.length" class="image-studio-reference-list">
                    <div v-for="file in referenceFiles" :key="file.name + file.size" class="image-studio-reference-item">
                      <span>{{ file.name }}</span>
                      <small>{{ formatBytes(file.size) }}</small>
                      <button type="button" :aria-label="t('imageStudio.removeReference')" @click="removeReference(file)">
                        <Icon name="x" size="sm" />
                      </button>
                    </div>
                  </div>
                </div>
              </transition>
            </section>

            <section class="image-studio-action-bar">
              <div class="image-studio-cost-preview">
                <span>{{ t('imageStudio.estimatedCost') }}</span>
                <strong>{{ estimatedCostLabel }}</strong>
                <small>{{ selectedPreviewMeta }}</small>
              </div>
              <button
                data-testid="image-studio-generate-button"
                type="submit"
                class="btn btn-primary image-studio-submit-button"
                :disabled="!canSubmit"
              >
                <svg v-if="submitting" class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                <Icon v-else name="sparkles" size="md" />
                {{ submitting ? t('imageStudio.generating') : submitLabel }}
              </button>
            </section>
          </div>
        </form>

        <aside class="image-studio-preview-panel image-studio-stage-panel">
          <div class="image-studio-panel-header">
            <div>
              <p class="image-studio-section-label">{{ t('imageStudio.resultPanel') }}</p>
              <h2>{{ t('imageStudio.latestResult') }}</h2>
            </div>
            <button type="button" class="image-studio-icon-button" :disabled="loadingHistory" @click="loadHistory">
              <Icon name="refresh" size="md" :class="loadingHistory ? 'animate-spin' : ''" />
            </button>
          </div>

          <div class="image-studio-preview-frame">
            <div v-if="submitting" class="image-studio-generating-state">
              <div class="image-studio-loader"></div>
              <strong>{{ t('imageStudio.generatingTitle') }}</strong>
              <span>{{ t('imageStudio.generatingHint') }}</span>
            </div>
            <img v-else-if="currentImage" :src="currentImage.image_url" :alt="currentImage.prompt" />
            <div v-else class="image-studio-empty-preview">
              <Icon name="sparkles" size="xl" />
              <strong>{{ t('imageStudio.emptyPreviewTitle') }}</strong>
              <span>{{ t('imageStudio.emptyPreviewHint') }}</span>
            </div>
          </div>

          <div v-if="currentImage" class="image-studio-current-actions">
            <button type="button" @click="copyImageURL(currentImage)">
              <Icon name="copy" size="sm" />
              {{ t('imageStudio.copyLink') }}
            </button>
            <button type="button" @click="downloadImage(currentImage)">
              <Icon name="download" size="sm" />
              {{ t('imageStudio.download') }}
            </button>
            <button type="button" @click="useAsReference(currentImage)">
              <Icon name="edit" size="sm" />
              {{ t('imageStudio.useAsReference') }}
            </button>
          </div>
        </aside>
      </div>

      <section class="image-studio-gallery image-studio-gallery-rail">
        <div class="image-studio-panel-header">
          <div>
            <p class="image-studio-section-label">{{ t('imageStudio.galleryPanel') }}</p>
            <h2>{{ t('imageStudio.recentImages') }}</h2>
          </div>
          <span class="image-studio-gallery-count">
            {{ t('imageStudio.galleryCount', { count: images.length }) }}
          </span>
        </div>

        <div v-if="loadingHistory && !images.length" class="image-studio-gallery-loading">
          <div v-for="n in 6" :key="n"></div>
        </div>
        <div v-else-if="images.length" class="image-studio-gallery-grid">
          <article v-for="item in images" :key="item.id" class="image-studio-image-card">
            <img :src="item.image_url" :alt="item.prompt" loading="lazy" />
            <div class="image-studio-image-card-body">
              <div>
                <strong>{{ item.model }}</strong>
                <span>{{ item.aspect_ratio }} / {{ item.size }}</span>
              </div>
              <p>{{ item.prompt }}</p>
              <div class="image-studio-image-card-actions">
                <button type="button" @click="copyImageURL(item)">
                  <Icon name="copy" size="sm" />
                </button>
                <button type="button" @click="downloadImage(item)">
                  <Icon name="download" size="sm" />
                </button>
                <button type="button" @click="deleteHistoryImage(item)">
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </div>
          </article>
        </div>
        <div v-else class="image-studio-empty-gallery">
          <Icon name="inbox" size="xl" />
          <strong>{{ t('imageStudio.emptyGalleryTitle') }}</strong>
          <span>{{ t('imageStudio.emptyGalleryHint') }}</span>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  imageStudioAPI,
  type ImageStudioAspectRatio,
  type ImageStudioConfig,
  type ImageStudioGroupOption,
  type ImageStudioImage,
  type ImageStudioMode,
  type ImageStudioModelOption,
  type ImageStudioOptions,
  type ImageStudioQualityOption,
} from '@/api/images'
import { keysAPI } from '@/api/keys'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatBytes, formatCurrency } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const config = ref<ImageStudioConfig | null>(null)
const options = ref<ImageStudioOptions | null>(null)
const apiKeys = ref<ApiKey[]>([])
const images = ref<ImageStudioImage[]>([])
const currentImage = ref<ImageStudioImage | null>(null)
const mode = ref<ImageStudioMode>('generation')
const prompt = ref('')
const selectedAPIKeyID = ref<number | null>(null)
const model = ref('')
const aspectRatio = ref('1:1')
const quality = ref('1K')
const referenceFiles = ref<File[]>([])
const loadingConfig = ref(false)
const loadingHistory = ref(false)
const submitting = ref(false)
const dragging = ref(false)
const loadError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const totalImages = ref(0)
type ImageStudioSelectMenu = 'apiKey' | 'model'
const openSelect = ref<ImageStudioSelectMenu | null>(null)

const groupOptions = computed<ImageStudioGroupOption[]>(() => options.value?.groups ?? [])
const imageGroupIDs = computed(() => new Set(groupOptions.value.map((item) => item.id)))
const availableAPIKeys = computed<ApiKey[]>(() =>
  apiKeys.value.filter((item) => {
    if (item.status !== 'active' || item.group_id == null || !imageGroupIDs.value.has(item.group_id)) {
      return false
    }
    if (item.expires_at && new Date(item.expires_at).getTime() <= Date.now()) {
      return false
    }
    return true
  }),
)
const selectedAPIKey = computed(() =>
  availableAPIKeys.value.find((item) => item.id === selectedAPIKeyID.value) ?? availableAPIKeys.value[0] ?? null,
)
const selectedGroup = computed(() =>
  groupOptions.value.find((item) => item.id === selectedAPIKey.value?.group_id) ?? null,
)
const modelOptions = computed<ImageStudioModelOption[]>(() => {
  if (selectedGroup.value?.models?.length) {
    return selectedGroup.value.models
  }
  return (config.value?.allowed_models ?? []).map((item) => ({
    model: item,
    label: item,
    capabilities: ['generation', 'edit'],
  }))
})
const qualityOptions = computed<ImageStudioQualityOption[]>(() => {
  if (selectedGroup.value?.qualities?.length) {
    return selectedGroup.value.qualities
  }
  return ['1K', '2K', '4K'].map((item) => ({
    quality: item,
    label: item,
    billing_tier: item,
    estimated_cost: 0,
  }))
})
const aspectRatioOptions = computed<ImageStudioAspectRatio[]>(() => config.value?.aspect_ratios ?? [])
const selectedRatio = computed(() => aspectRatioOptions.value.find((item) => item.ratio === aspectRatio.value))
const selectedQuality = computed(() => qualityOptions.value.find((item) => item.quality === quality.value))
const selectedModelOption = computed(() => modelOptions.value.find((item) => item.model === model.value))
const selectedModelLabel = computed(() => selectedModelOption.value?.label || selectedModelOption.value?.model || model.value)
const selectedAPIKeyLabel = computed(() =>
  selectedAPIKey.value ? apiKeyOptionLabel(selectedAPIKey.value) : t('imageStudio.apiKeyPlaceholder'),
)
const selectedAPIKeyMeta = computed(() =>
  selectedAPIKey.value ? apiKeyOptionMeta(selectedAPIKey.value) : t('imageStudio.noApiKeysDescription'),
)
const selectedPricePreview = computed(() =>
  selectedGroup.value?.prices?.find((item) => item.ratio === aspectRatio.value && item.quality === quality.value),
)
const maxReferenceImageMB = computed(() => config.value?.max_reference_image_mb ?? 20)
const canUse = computed(() =>
  Boolean(
    config.value?.enabled &&
      options.value?.enabled &&
      groupOptions.value.length > 0 &&
      availableAPIKeys.value.length > 0 &&
      selectedAPIKey.value &&
      selectedGroup.value &&
      modelOptions.value.length > 0,
  ),
)
const canSubmit = computed(() => {
  if (submitting.value || loadingConfig.value || !canUse.value) return false
  if (!selectedAPIKeyID.value || !selectedGroup.value?.id || !prompt.value.trim() || !model.value || !aspectRatio.value || !quality.value) return false
  if (mode.value === 'edit' && referenceFiles.value.length === 0) return false
  return true
})
const submitLabel = computed(() =>
  mode.value === 'edit' ? t('imageStudio.editButton') : t('imageStudio.generateButton'),
)
const estimatedCostLabel = computed(() => {
  const cost = selectedPricePreview.value?.estimated_cost ?? selectedQuality.value?.estimated_cost
  return typeof cost === 'number' ? formatCurrency(cost) : '-'
})
const selectedPreviewMeta = computed(() => {
  const size = selectedPricePreview.value?.size ?? selectedRatio.value?.size ?? '-'
  const tier =
    selectedPricePreview.value?.billing_tier ??
    selectedQuality.value?.billing_tier ??
    selectedRatio.value?.billing_tier ??
    '-'
  return `${size} / ${tier}`
})

watch(modelOptions, (items) => {
  if (!items.some((item) => item.model === model.value)) {
    model.value = chooseModel(options.value?.default_model || config.value?.default_model, items)
  }
})

watch(qualityOptions, (items) => {
  if (!items.some((item) => item.quality === quality.value)) {
    quality.value = items[0]?.quality ?? '1K'
  }
})

watch(aspectRatioOptions, (items) => {
  if (!items.some((item) => item.ratio === aspectRatio.value)) {
    aspectRatio.value = items[0]?.ratio ?? '1:1'
  }
})

watch(availableAPIKeys, (items) => {
  if (!items.some((item) => item.id === selectedAPIKeyID.value)) {
    selectedAPIKeyID.value = items[0]?.id ?? null
  }
})

watch(selectedAPIKeyID, () => {
  model.value = chooseModel(options.value?.default_model || config.value?.default_model, modelOptions.value)
  quality.value = qualityOptions.value[0]?.quality ?? '1K'
})

function toggleSelect(menu: ImageStudioSelectMenu) {
  openSelect.value = openSelect.value === menu ? null : menu
}

function closeSelectMenus() {
  openSelect.value = null
}

function chooseAPIKey(apiKeyID: number) {
  selectedAPIKeyID.value = apiKeyID
  closeSelectMenus()
}

function chooseModelOption(nextModel: string) {
  model.value = nextModel
  closeSelectMenus()
}

async function loadInitialData() {
  loadError.value = ''
  loadingConfig.value = true
  try {
    const [cfg, selectableOptions, keyResponse] = await Promise.all([
      imageStudioAPI.getConfig(),
      imageStudioAPI.getOptions(),
      keysAPI.list(1, 100, { status: 'active' }),
    ])
    config.value = cfg
    options.value = selectableOptions
    apiKeys.value = keyResponse.items ?? []
    selectedAPIKeyID.value = defaultAPIKeyID(selectableOptions, apiKeys.value)
    model.value = chooseModel(selectableOptions.default_model || cfg.default_model, modelOptions.value)
    quality.value = qualityOptions.value[0]?.quality ?? '1K'
    aspectRatio.value = cfg.aspect_ratios[0]?.ratio || '1:1'
    await loadHistory()
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('imageStudio.loadFailed'))
  } finally {
    loadingConfig.value = false
  }
}

async function loadHistory() {
  loadingHistory.value = true
  try {
    const response = await imageStudioAPI.list({ page: 1, page_size: 12 })
    images.value = response.items ?? []
    totalImages.value = response.total ?? images.value.length
    if (!currentImage.value && images.value.length > 0) {
      currentImage.value = images.value[0]
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('imageStudio.historyLoadFailed')))
  } finally {
    loadingHistory.value = false
  }
}

async function handleSubmit() {
  if (!canSubmit.value) return
  submitting.value = true
  try {
    const normalizedPrompt = prompt.value.trim()
    const payload = {
      api_key_id: selectedAPIKeyID.value,
      group_id: selectedGroup.value?.id ?? null,
      model: model.value,
      prompt: normalizedPrompt,
      aspect_ratio: aspectRatio.value,
      quality: quality.value,
    }
    const created =
      mode.value === 'edit'
        ? await imageStudioAPI.edit({
            ...payload,
            images: referenceFiles.value,
          })
        : await imageStudioAPI.generate(payload)

    currentImage.value = created
    images.value = [created, ...images.value.filter((item) => item.id !== created.id)].slice(0, 12)
    totalImages.value += 1
    await authStore.refreshUser().catch(() => undefined)
    appStore.showSuccess(t('imageStudio.generateSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('imageStudio.generateFailed')))
  } finally {
    submitting.value = false
  }
}

function addReferenceFiles(files: File[]) {
  const maxBytes = maxReferenceImageMB.value * 1024 * 1024
  const accepted = files.filter((file) => file.type.startsWith('image/') && file.size <= maxBytes)
  const rejected = files.length - accepted.length
  if (rejected > 0) {
    appStore.showError(t('imageStudio.referenceRejected', { count: rejected, mb: maxReferenceImageMB.value }))
  }
  referenceFiles.value = [...referenceFiles.value, ...accepted].slice(0, 4)
}

function handleFileInput(event: Event) {
  const input = event.target as HTMLInputElement
  addReferenceFiles(Array.from(input.files ?? []))
  input.value = ''
}

function handleDrop(event: DragEvent) {
  dragging.value = false
  addReferenceFiles(Array.from(event.dataTransfer?.files ?? []))
}

function handlePaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files ?? [])
  if (files.length > 0) {
    addReferenceFiles(files)
  }
}

function removeReference(file: File) {
  referenceFiles.value = referenceFiles.value.filter((item) => item !== file)
}

function defaultAPIKeyID(selectableOptions: ImageStudioOptions, keys: ApiKey[]): number | null {
  const groups = selectableOptions.groups ?? []
  const groupIDs = new Set(groups.map((item) => item.id))
  const defaultGroupID = selectableOptions.default_group_id
  const usableKeys = keys.filter((item) => item.status === 'active' && item.group_id != null && groupIDs.has(item.group_id))
  if (defaultGroupID != null) {
    const defaultKey = usableKeys.find((item) => item.group_id === defaultGroupID)
    if (defaultKey) {
      return defaultKey.id
    }
  }
  return usableKeys[0]?.id ?? null
}

function chooseModel(preferred: string | undefined, models: ImageStudioModelOption[]): string {
  if (!models.length) {
    return ''
  }
  if (preferred && models.some((item) => item.model === preferred)) {
    return preferred
  }
  return models[0]?.model ?? ''
}

function ratioPreviewLabel(item: ImageStudioAspectRatio): string {
  const preview = selectedGroup.value?.prices?.find(
    (price) => price.ratio === item.ratio && price.quality === quality.value,
  )
  const size = preview?.size ?? item.size
  const tier = preview?.billing_tier ?? quality.value ?? item.billing_tier
  return `${size} / ${tier}`
}

function apiKeyOptionLabel(key: ApiKey): string {
  return key.name?.trim() || `#${key.id}`
}

function apiKeyOptionMeta(key: ApiKey): string {
  const group = groupOptions.value.find((item) => item.id === key.group_id)
  const groupName = group?.name || key.group?.name || t('imageStudio.group')
  return `${groupName} / #${key.id}`
}

async function copyImageURL(image: ImageStudioImage) {
  try {
    await navigator.clipboard.writeText(image.image_url)
    appStore.showSuccess(t('imageStudio.linkCopied'))
  } catch {
    appStore.showError(t('imageStudio.copyFailed'))
  }
}

function downloadImage(image: ImageStudioImage) {
  const anchor = document.createElement('a')
  anchor.href = image.image_url
  anchor.download = `passion-api-image-${image.id}.png`
  anchor.rel = 'noopener'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
}

async function useAsReference(image: ImageStudioImage) {
  mode.value = 'edit'
  prompt.value = image.prompt
  try {
    const file = await imageToReferenceFile(image)
    referenceFiles.value = [file, ...referenceFiles.value.filter((item) => item.name !== file.name)].slice(0, 4)
    appStore.showSuccess(t('imageStudio.referenceModeHint'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('imageStudio.referenceLoadFailed')))
  }
}

async function imageToReferenceFile(image: ImageStudioImage): Promise<File> {
  const response = await fetch(image.image_url, { credentials: 'same-origin' })
  if (!response.ok) {
    throw new Error(`Failed to load reference image (${response.status})`)
  }
  const blob = await response.blob()
  const type = blob.type || image.mime_type || 'image/png'
  return new File([blob], imageReferenceFileName(image, type), { type })
}

function imageReferenceFileName(image: ImageStudioImage, mimeType: string): string {
  return `passion-api-image-${image.id}${imageFileExtension(image, mimeType)}`
}

function imageFileExtension(image: ImageStudioImage, mimeType: string): string {
  const normalized = mimeType.toLowerCase()
  if (normalized.includes('jpeg') || normalized.includes('jpg')) return '.jpg'
  if (normalized.includes('webp')) return '.webp'
  if (normalized.includes('gif')) return '.gif'
  const match = image.image_url.match(/\.(png|jpe?g|webp|gif)(?:[?#]|$)/i)
  if (match?.[1]) {
    const ext = match[1].toLowerCase()
    return ext === 'jpeg' ? '.jpg' : `.${ext}`
  }
  return '.png'
}

async function deleteHistoryImage(image: ImageStudioImage) {
  const ok = window.confirm(t('imageStudio.deleteConfirm'))
  if (!ok) return

  try {
    await imageStudioAPI.delete(image.id)
    images.value = images.value.filter((item) => item.id !== image.id)
    if (currentImage.value?.id === image.id) {
      currentImage.value = images.value[0] ?? null
    }
    totalImages.value = Math.max(0, totalImages.value - 1)
    appStore.showSuccess(t('imageStudio.deleteSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('imageStudio.deleteFailed')))
  }
}

onMounted(() => {
  void loadInitialData()
  document.addEventListener('click', closeSelectMenus)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeSelectMenus)
})
</script>

<style scoped>
.image-studio-shell {
  display: grid;
  max-width: min(100%, 96rem);
  gap: clamp(0.9rem, 1.35vw, 1.25rem);
  padding-bottom: 2rem;
  --studio-surface: rgba(255, 255, 255, 0.88);
  --studio-surface-soft: rgba(248, 250, 252, 0.82);
  --studio-border: rgba(148, 163, 184, 0.24);
  --studio-border-soft: rgba(203, 213, 225, 0.58);
  --studio-text: rgb(15, 23, 42);
  --studio-muted: rgb(100, 116, 139);
  --studio-subtle: rgb(71, 85, 105);
  --studio-radius: 1rem;
  --studio-radius-sm: 0.72rem;
  --studio-shadow: 0 18px 44px rgba(15, 23, 42, 0.07);
}

.image-studio-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(22rem, 0.82fr);
  gap: clamp(0.95rem, 1.5vw, 1.35rem);
  align-items: start;
}

.image-studio-workspace,
.image-studio-preview-panel,
.image-studio-gallery {
  position: relative;
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.78)),
    var(--studio-surface);
  box-shadow: var(--studio-shadow);
}

.image-studio-workspace {
  overflow: visible;
  z-index: 2;
}

.image-studio-preview-panel {
  position: sticky;
  top: 1rem;
}

.image-studio-workspace::before,
.image-studio-preview-panel::before,
.image-studio-gallery::before {
  display: none;
}

.image-studio-workspace,
.image-studio-preview-panel {
  padding: clamp(0.95rem, 1.45vw, 1.2rem);
}

.image-studio-gallery {
  padding: clamp(0.95rem, 1.45vw, 1.2rem);
}

.image-studio-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.9rem;
  padding-bottom: 0.72rem;
  border-bottom: 1px solid rgba(226, 232, 240, 0.76);
}

.image-studio-panel-header h2 {
  font-size: 1.05rem;
  font-weight: 750;
  color: var(--studio-text);
}

.image-studio-section-label {
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0;
  text-transform: uppercase;
  color: var(--brand-700);
}

.image-studio-mode-switch {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.28rem;
  border-radius: 9999px;
  border: 1px solid rgba(203, 213, 225, 0.62);
  background: rgba(241, 245, 249, 0.86);
  padding: 0.25rem;
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.06);
}

.image-studio-mode-switch button {
  display: inline-flex;
  min-height: 2.25rem;
  align-items: center;
  gap: 0.4rem;
  border-radius: 9999px;
  padding: 0.42rem 0.75rem;
  color: var(--studio-subtle);
  font-size: 0.82rem;
  font-weight: 700;
  transition:
    background 180ms ease,
    color 180ms ease,
    box-shadow 180ms ease;
}

.image-studio-mode-switch button.active {
  color: white;
  background: linear-gradient(135deg, var(--brand-600), var(--brand-500), var(--brand-cyan));
  box-shadow: 0 8px 18px rgba(37, 99, 235, 0.18);
}

.image-studio-field {
  display: grid;
  gap: 0.5rem;
  margin-top: 1rem;
}

.image-studio-command-surface .image-studio-field {
  margin-top: 0;
}

.image-studio-field > span {
  color: rgb(51, 65, 85);
  font-size: 0.84rem;
  font-weight: 750;
}

.image-studio-field > small {
  color: rgb(100, 116, 139);
  font-size: 0.74rem;
  line-height: 1.35;
}

.image-studio-field textarea {
  width: 100%;
  border-radius: 0.9rem;
  border: 1px solid var(--studio-border-soft);
  background: rgba(255, 255, 255, 0.82);
  color: var(--studio-text);
  outline: none;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background 180ms ease;
}

.image-studio-field textarea {
  min-height: clamp(10.5rem, 17vw, 14.5rem);
  resize: vertical;
  padding: 1rem;
  line-height: 1.62;
}

.image-studio-field textarea:focus {
  border-color: rgba(var(--brand-rgb), 0.58);
  box-shadow:
    0 0 0 3px rgba(var(--brand-rgb), 0.12),
    0 14px 34px rgba(37, 99, 235, 0.1);
}

.image-studio-command-surface {
  margin-top: 1rem;
  display: grid;
  gap: 0;
}

.image-studio-prompt-section,
.image-studio-action-bar {
  margin-top: 0.95rem;
}

.image-studio-prompt-section {
  margin-top: 0;
}

.image-studio-control-dock {
  margin-top: 0.95rem;
  display: grid;
  gap: 0.85rem;
  border: 1px solid rgba(203, 213, 225, 0.62);
  border-radius: 1rem;
  background:
    linear-gradient(180deg, rgba(248, 250, 252, 0.94), rgba(241, 245, 249, 0.7)),
    rgba(248, 250, 252, 0.82);
  padding: 0.78rem;
  box-shadow:
    inset 0 1px 1px rgba(255, 255, 255, 0.86),
    inset 0 -1px 0 rgba(148, 163, 184, 0.08);
}

.image-studio-foundation-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
  align-items: start;
}

.image-studio-output-row {
  display: grid;
  grid-template-columns: minmax(14rem, 0.76fr) minmax(19rem, 1fr);
  gap: 0.85rem;
  align-items: start;
}

.image-studio-select-field {
  margin-top: 0;
}

.image-studio-select {
  position: relative;
}

.image-studio-select-trigger {
  display: flex;
  min-height: 3.1rem;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border-radius: 0.78rem;
  border: 1px solid rgba(203, 213, 225, 0.7);
  background: rgba(255, 255, 255, 0.86);
  color: var(--studio-text);
  padding: 0.75rem 0.9rem;
  text-align: left;
  box-shadow: 0 1px 0 rgba(15, 23, 42, 0.02);
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background 180ms ease;
}

.image-studio-select-trigger:disabled {
  cursor: not-allowed;
  opacity: 0.66;
}

.image-studio-select-trigger:hover:not(:disabled),
.image-studio-select-trigger:focus-visible,
.image-studio-select.is-open .image-studio-select-trigger {
  border-color: rgba(var(--brand-rgb), 0.58);
  box-shadow:
    0 0 0 3px rgba(var(--brand-rgb), 0.12),
    0 10px 26px rgba(37, 99, 235, 0.08);
}

.image-studio-select-trigger svg {
  flex-shrink: 0;
  color: var(--brand-600);
  transition: transform 180ms ease;
}

.image-studio-select.is-open .image-studio-select-trigger svg {
  transform: rotate(180deg);
}

.image-studio-select-value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 760;
}

.image-studio-select-menu {
  position: absolute;
  z-index: 30;
  inset-inline: 0;
  top: calc(100% + 0.45rem);
  display: grid;
  max-height: min(18rem, 48vh);
  overflow: auto;
  border-radius: var(--studio-radius);
  border: 1px solid var(--studio-border);
  background: rgba(255, 255, 255, 0.98);
  padding: 0.35rem;
  box-shadow:
    0 16px 36px rgba(15, 23, 42, 0.13),
    0 0 0 1px rgba(255, 255, 255, 0.72) inset;
}

.image-studio-select-option {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  column-gap: 0.75rem;
  row-gap: 0.18rem;
  border-radius: 0.55rem;
  min-height: 2.5rem;
  padding: 0.62rem 0.75rem;
  color: rgb(51, 65, 85);
  text-align: left;
  transition:
    background 160ms ease,
    color 160ms ease,
    box-shadow 160ms ease;
}

.image-studio-select-option span,
.image-studio-select-option small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-studio-select-option span {
  font-weight: 760;
}

.image-studio-select-option small {
  grid-column: 1;
  color: rgb(100, 116, 139);
  font-size: 0.72rem;
}

.image-studio-select-option svg {
  grid-column: 2;
  grid-row: 1 / span 2;
  color: var(--brand-600);
}

.image-studio-select-option:hover,
.image-studio-select-option:focus-visible,
.image-studio-select-option.is-selected {
  background: linear-gradient(135deg, rgba(var(--brand-rgb), 0.09), rgba(var(--brand-cyan-rgb), 0.13));
  color: var(--brand-700);
  box-shadow: inset 0 0 0 1px rgba(var(--brand-rgb), 0.16);
}

.image-studio-select-pop-enter-active,
.image-studio-select-pop-leave-active {
  transition:
    opacity 150ms ease,
    transform 150ms ease;
}

.image-studio-select-pop-enter-from,
.image-studio-select-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.image-studio-choice-section {
  margin-top: 0;
  border: 0;
  border-radius: var(--studio-radius);
  background: transparent;
  box-shadow: none;
  padding: 0;
}

.image-studio-choice-picker {
  display: grid;
  gap: 0.25rem;
  border-radius: 0.86rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background:
    linear-gradient(180deg, rgba(226, 232, 240, 0.72), rgba(241, 245, 249, 0.54)),
    rgba(226, 232, 240, 0.45);
  padding: 0.3rem;
  box-shadow:
    inset 0 1px 2px rgba(15, 23, 42, 0.1),
    inset 0 -1px 0 rgba(255, 255, 255, 0.72);
}

.image-studio-ratio-picker {
  grid-template-columns: repeat(auto-fit, minmax(5.2rem, 1fr));
}

.image-studio-quality-picker {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.image-studio-choice-picker button {
  min-height: 3.25rem;
  border-radius: 0.68rem;
  border: 1px solid transparent;
  background: transparent;
  color: rgb(51, 65, 85);
  padding: 0.48rem 0.42rem;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background 180ms ease,
    color 180ms ease,
    transform 180ms ease;
}

.image-studio-choice-picker button:hover,
.image-studio-choice-picker button:focus-visible {
  color: var(--brand-700);
  background: rgba(255, 255, 255, 0.58);
  box-shadow: inset 0 0 0 1px rgba(var(--brand-rgb), 0.12);
}

.image-studio-choice-picker button.active {
  border-color: rgba(255, 255, 255, 0.92);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(248, 250, 252, 0.82)),
    white;
  color: var(--brand-700);
  box-shadow:
    0 10px 22px rgba(37, 99, 235, 0.14),
    0 0 0 1px rgba(var(--brand-rgb), 0.24),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
  transform: translateY(-0.5px);
}

.image-studio-choice-picker strong,
.image-studio-choice-picker small {
  display: block;
}

.image-studio-choice-picker strong {
  font-weight: 800;
}

.image-studio-choice-picker small {
  margin-top: 0.15rem;
  color: rgb(100, 116, 139);
  font-size: 0.67rem;
  line-height: 1.25;
}

.image-studio-choice-picker button.active small {
  color: rgba(29, 78, 216, 0.72);
}

.image-studio-disabled,
.image-studio-alert {
  margin-top: 1rem;
  display: flex;
  align-items: center;
  gap: 0.85rem;
  border-radius: var(--studio-radius);
  border: 1px solid rgba(245, 158, 11, 0.28);
  background: linear-gradient(135deg, rgba(255, 251, 235, 0.92), rgba(255, 255, 255, 0.96));
  padding: 0.9rem 1rem;
  color: rgb(146, 64, 14);
}

.image-studio-disabled div {
  display: grid;
  gap: 0.15rem;
}

.image-studio-disabled span,
.image-studio-alert span {
  font-size: 0.88rem;
}

.image-studio-alert {
  margin-top: 0;
  border-color: rgba(239, 68, 68, 0.3);
  color: rgb(185, 28, 28);
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.98), rgba(255, 255, 255, 0.94));
}

.image-studio-link-button {
  margin-left: auto;
  color: var(--brand-700);
  font-weight: 750;
}

.image-studio-upload-panel {
  display: grid;
  gap: 0.75rem;
  border-top: 1px solid rgba(203, 213, 225, 0.56);
  padding-top: 0.85rem;
}

.image-studio-dropzone {
  display: grid;
  min-height: 7.5rem;
  place-items: center;
  gap: 0.35rem;
  border-radius: var(--studio-radius);
  border: 1px dashed rgba(var(--brand-rgb), 0.42);
  background: rgba(239, 246, 255, 0.44);
  color: var(--brand-700);
  text-align: center;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background 180ms ease;
}

.image-studio-dropzone.is-dragging,
.image-studio-dropzone:hover,
.image-studio-dropzone:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.72);
  box-shadow: 0 0 0 3px rgba(var(--brand-cyan-rgb), 0.14);
}

.image-studio-dropzone span {
  font-weight: 800;
}

.image-studio-dropzone small {
  color: rgb(100, 116, 139);
}

.image-studio-reference-list {
  display: grid;
  gap: 0.5rem;
}

.image-studio-reference-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 0.7rem;
  border-radius: var(--studio-radius-sm);
  border: 1px solid var(--studio-border-soft);
  background: rgba(255, 255, 255, 0.78);
  padding: 0.65rem 0.8rem;
  color: rgb(51, 65, 85);
}

.image-studio-reference-item span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.86rem;
  font-weight: 700;
}

.image-studio-reference-item small {
  color: rgb(100, 116, 139);
  font-size: 0.75rem;
}

.image-studio-action-bar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.7);
  border-radius: 1rem;
  background:
    linear-gradient(135deg, rgba(var(--brand-rgb), 0.06), rgba(var(--brand-cyan-rgb), 0.1)),
    rgba(239, 246, 255, 0.62);
  padding: 0.78rem;
}

.image-studio-cost-preview {
  display: grid;
  gap: 0.1rem;
}

.image-studio-cost-preview span {
  color: rgb(100, 116, 139);
  font-size: 0.75rem;
  font-weight: 700;
}

.image-studio-cost-preview strong {
  color: rgb(15, 23, 42);
  font-size: 1.25rem;
  font-weight: 850;
}

.image-studio-cost-preview small {
  color: rgb(71, 85, 105);
  font-size: 0.78rem;
}

.image-studio-submit-button {
  min-height: 3rem;
  min-width: 12.5rem;
  justify-content: center;
  gap: 0.55rem;
  border-radius: 0.85rem;
}

.image-studio-icon-button {
  display: inline-flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: var(--studio-radius-sm);
  border: 1px solid var(--studio-border-soft);
  color: var(--brand-700);
  background: rgba(239, 246, 255, 0.66);
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease;
}

.image-studio-icon-button:hover,
.image-studio-icon-button:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.54);
  box-shadow: 0 0 0 3px rgba(var(--brand-rgb), 0.12);
}

.image-studio-preview-frame {
  margin-top: 0.95rem;
  display: grid;
  min-height: 0;
  aspect-ratio: 1 / 1;
  place-items: center;
  overflow: hidden;
  border-radius: 1rem;
  border: 1px solid rgba(203, 213, 225, 0.6);
  background:
    linear-gradient(45deg, rgba(219, 234, 254, 0.38) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(219, 234, 254, 0.38) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(219, 234, 254, 0.38) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(219, 234, 254, 0.38) 75%),
    rgba(248, 250, 252, 0.82);
  background-position: 0 0, 0 10px, 10px -10px, -10px 0;
  background-size: 20px 20px;
}

.image-studio-preview-frame img {
  display: block;
  max-height: 100%;
  max-width: 100%;
  object-fit: contain;
}

.image-studio-generating-state,
.image-studio-empty-preview,
.image-studio-empty-gallery {
  display: grid;
  place-items: center;
  gap: 0.55rem;
  padding: 2rem;
  text-align: center;
  color: rgb(71, 85, 105);
}

.image-studio-generating-state strong,
.image-studio-empty-preview strong,
.image-studio-empty-gallery strong {
  color: rgb(15, 23, 42);
  font-weight: 800;
}

.image-studio-loader {
  height: 3rem;
  width: 3rem;
  border-radius: 9999px;
  background: conic-gradient(from 0deg, var(--brand-600), var(--brand-cyan), transparent 72%);
  animation: image-studio-spin 1s linear infinite;
  mask: radial-gradient(circle, transparent 44%, #000 46%);
}

.image-studio-current-actions,
.image-studio-image-card-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.image-studio-current-actions {
  margin-top: 0.78rem;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.image-studio-current-actions button,
.image-studio-image-card-actions button {
  display: inline-flex;
  min-height: 2.35rem;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  border-radius: var(--studio-radius-sm);
  border: 1px solid var(--studio-border-soft);
  background: rgba(239, 246, 255, 0.66);
  color: var(--brand-700);
  padding: 0.45rem 0.7rem;
  font-size: 0.8rem;
  font-weight: 750;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease;
}

.image-studio-current-actions button:hover,
.image-studio-image-card-actions button:hover,
.image-studio-current-actions button:focus-visible,
.image-studio-image-card-actions button:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.54);
  box-shadow: 0 0 0 3px rgba(var(--brand-rgb), 0.12);
  transform: translateY(-1px);
}

.image-studio-gallery-count {
  border-radius: 9999px;
  background: rgba(var(--brand-rgb), 0.07);
  color: var(--brand-700);
  padding: 0.35rem 0.7rem;
  font-size: 0.78rem;
  font-weight: 800;
}

.image-studio-gallery-grid,
.image-studio-gallery-loading {
  margin-top: 1rem;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 13.5rem), 1fr));
  gap: 0.85rem;
}

.image-studio-gallery-loading div {
  min-height: 16rem;
  border-radius: 1rem;
  background: linear-gradient(90deg, rgba(226, 232, 240, 0.64), rgba(239, 246, 255, 0.9), rgba(226, 232, 240, 0.64));
  background-size: 220% 100%;
  animation: image-studio-shimmer 1.25s ease-in-out infinite;
}

.image-studio-image-card {
  overflow: hidden;
  border-radius: 0.9rem;
  border: 1px solid rgba(203, 213, 225, 0.58);
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.055);
}

.image-studio-image-card img {
  display: block;
  width: 100%;
  aspect-ratio: 1;
  object-fit: cover;
  background: rgba(239, 246, 255, 0.9);
}

.image-studio-image-card-body {
  display: grid;
  gap: 0.65rem;
  padding: 0.78rem;
}

.image-studio-image-card-body div:first-child {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
}

.image-studio-image-card-body strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(15, 23, 42);
  font-size: 0.86rem;
}

.image-studio-image-card-body span {
  flex-shrink: 0;
  color: rgb(100, 116, 139);
  font-size: 0.75rem;
  font-weight: 700;
}

.image-studio-image-card-body p {
  display: -webkit-box;
  min-height: 2.5rem;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  color: rgb(71, 85, 105);
  font-size: 0.8rem;
  line-height: 1.55;
}

.image-studio-fade-enter-active,
.image-studio-fade-leave-active {
  transition:
    opacity 180ms ease,
    transform 180ms ease;
}

.image-studio-fade-enter-from,
.image-studio-fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.dark .image-studio-workspace,
.dark .image-studio-preview-panel,
.dark .image-studio-gallery {
  --studio-surface: rgba(15, 23, 42, 0.92);
  --studio-surface-soft: rgba(15, 23, 42, 0.52);
  --studio-border: rgba(96, 165, 250, 0.24);
  --studio-border-soft: rgba(96, 165, 250, 0.2);
  --studio-text: rgb(248, 250, 252);
  --studio-muted: rgb(148, 163, 184);
  --studio-subtle: rgb(203, 213, 225);
  border-color: var(--studio-border);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.94), rgba(2, 6, 23, 0.9)),
    var(--studio-surface);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.04),
    0 18px 44px rgba(0, 0, 0, 0.28);
}

.dark .image-studio-panel-header {
  border-color: rgba(96, 165, 250, 0.16);
}

.dark .image-studio-panel-header h2,
.dark .image-studio-cost-preview strong,
.dark .image-studio-generating-state strong,
.dark .image-studio-empty-preview strong,
.dark .image-studio-empty-gallery strong,
.dark .image-studio-image-card-body strong {
  color: white;
}

.dark .image-studio-field > span,
.dark .image-studio-generating-state,
.dark .image-studio-empty-preview,
.dark .image-studio-empty-gallery,
.dark .image-studio-image-card-body p {
  color: rgb(203, 213, 225);
}

.dark .image-studio-field textarea,
.dark .image-studio-select-trigger,
.dark .image-studio-select-menu,
.dark .image-studio-reference-item,
.dark .image-studio-image-card {
  border-color: var(--studio-border-soft);
  background: rgba(15, 23, 42, 0.78);
  color: white;
}

.dark .image-studio-command-surface,
.dark .image-studio-control-dock,
.dark .image-studio-foundation-row,
.dark .image-studio-output-row,
.dark .image-studio-action-bar,
.dark .image-studio-upload-panel {
  border-color: var(--studio-border-soft);
}

.dark .image-studio-control-dock {
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.78), rgba(2, 6, 23, 0.52)),
    rgba(15, 23, 42, 0.58);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.04),
    inset 0 -1px 0 rgba(148, 163, 184, 0.08);
}

.dark .image-studio-action-bar {
  background:
    linear-gradient(135deg, rgba(37, 99, 235, 0.18), rgba(14, 165, 233, 0.12)),
    rgba(15, 23, 42, 0.62);
}

.dark .image-studio-choice-section {
  background: transparent;
}

.dark .image-studio-choice-picker {
  border-color: rgba(96, 165, 250, 0.16);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.86), rgba(2, 6, 23, 0.5)),
    rgba(15, 23, 42, 0.48);
  box-shadow:
    inset 0 1px 2px rgba(0, 0, 0, 0.36),
    inset 0 -1px 0 rgba(148, 163, 184, 0.08);
}

.dark .image-studio-choice-picker button {
  color: rgb(203, 213, 225);
}

.dark .image-studio-choice-picker button:hover,
.dark .image-studio-choice-picker button:focus-visible {
  color: rgb(191, 219, 254);
  background: rgba(30, 41, 59, 0.78);
}

.dark .image-studio-choice-picker button.active {
  border-color: rgba(96, 165, 250, 0.28);
  background:
    linear-gradient(180deg, rgba(37, 99, 235, 0.34), rgba(14, 165, 233, 0.2)),
    rgba(15, 23, 42, 0.92);
  color: white;
  box-shadow:
    0 8px 18px rgba(0, 0, 0, 0.24),
    0 0 0 1px rgba(96, 165, 250, 0.24),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.dark .image-studio-choice-picker small,
.dark .image-studio-choice-picker button.active small {
  color: rgb(148, 163, 184);
}

.dark .image-studio-select-option {
  color: rgb(203, 213, 225);
}

.dark .image-studio-select-option small,
.dark .image-studio-cost-preview small {
  color: rgb(148, 163, 184);
}

.dark .image-studio-select-option:hover,
.dark .image-studio-select-option:focus-visible,
.dark .image-studio-select-option.is-selected {
  color: rgb(191, 219, 254);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.22), rgba(6, 182, 212, 0.16));
}

.dark .image-studio-mode-switch,
.dark .image-studio-icon-button,
.dark .image-studio-current-actions button,
.dark .image-studio-image-card-actions button {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.14);
  color: rgb(191, 219, 254);
}

.dark .image-studio-preview-frame {
  border-color: rgba(96, 165, 250, 0.18);
  background:
    linear-gradient(45deg, rgba(30, 64, 175, 0.2) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(30, 64, 175, 0.2) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(30, 64, 175, 0.2) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(30, 64, 175, 0.2) 75%),
    rgba(2, 6, 23, 0.74);
  background-position: 0 0, 0 10px, 10px -10px, -10px 0;
  background-size: 20px 20px;
}

.dark .image-studio-dropzone {
  border-color: rgba(96, 165, 250, 0.36);
  background: rgba(15, 23, 42, 0.54);
  color: rgb(191, 219, 254);
}

.dark .image-studio-disabled {
  border-color: rgba(245, 158, 11, 0.24);
  background: linear-gradient(135deg, rgba(120, 53, 15, 0.26), rgba(2, 6, 23, 0.88));
  color: rgb(252, 211, 77);
}

.dark .image-studio-alert {
  border-color: rgba(248, 113, 113, 0.24);
  color: rgb(252, 165, 165);
  background: linear-gradient(135deg, rgba(127, 29, 29, 0.24), rgba(2, 6, 23, 0.88));
}

@keyframes image-studio-spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes image-studio-shimmer {
  0% {
    background-position: 180% 0;
  }
  100% {
    background-position: -180% 0;
  }
}

@media (max-width: 1100px) {
  .image-studio-grid {
    grid-template-columns: 1fr;
  }

  .image-studio-preview-panel {
    position: static;
  }
}

@media (max-width: 760px) {
  .image-studio-panel-header {
    align-items: stretch;
    flex-direction: column;
  }

  .image-studio-action-bar {
    align-items: stretch;
    grid-template-columns: 1fr;
  }

  .image-studio-foundation-row,
  .image-studio-output-row {
    grid-template-columns: 1fr;
  }

  .image-studio-current-actions {
    grid-template-columns: 1fr;
  }

  .image-studio-preview-frame {
    min-height: 20rem;
  }

  .image-studio-submit-button {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .image-studio-loader,
  .image-studio-gallery-loading div {
    animation: none;
  }

  .image-studio-choice-picker button,
  .image-studio-current-actions button,
  .image-studio-image-card-actions button {
    transition: none;
  }
}
</style>
