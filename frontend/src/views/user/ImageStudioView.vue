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

      <div class="image-studio-grid image-studio-workbench" data-testid="image-studio-workbench">
        <div class="image-studio-control-console" data-testid="image-studio-control-console">
          <form class="image-studio-workspace" data-testid="image-studio-submit" @submit.prevent="handleSubmit">
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
              <div class="image-studio-step-heading">
                <span>00</span>
                <strong>{{ t('imageStudio.stepPrompt') }}</strong>
              </div>
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
              <transition name="image-studio-fade">
                <div v-if="mode === 'edit'" class="image-studio-upload-panel">
                  <div class="image-studio-step-heading">
                    <span>01</span>
                    <strong>{{ t('imageStudio.stepReference') }}</strong>
                  </div>
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
                      <img
                        class="image-studio-reference-thumb"
                        data-testid="image-studio-reference-preview"
                        :src="referencePreviewURL(file)"
                        alt=""
                      />
                      <small>{{ formatBytes(file.size) }}</small>
                      <button type="button" :aria-label="t('imageStudio.removeReference')" @click="removeReference(file)">
                        <Icon name="x" size="sm" />
                      </button>
                    </div>
                  </div>
                </div>
              </transition>

              <div class="image-studio-output-row">
                <div class="image-studio-step-heading image-studio-step-heading-full">
                  <span>{{ mode === 'edit' ? '02' : '01' }}</span>
                  <strong>{{ t('imageStudio.stepOutput') }}</strong>
                </div>
                <div class="image-studio-field image-studio-choice-section">
                  <span>{{ t('imageStudio.outputCount') }}</span>
                  <div class="image-studio-choice-picker image-studio-count-picker">
                    <button
                      v-for="item in outputCountOptions"
                      :key="item"
                      type="button"
                      :data-testid="`image-studio-output-count-${item}`"
                      :class="{ active: outputCount === item }"
                      :disabled="submitting || !canUse"
                      @click="chooseOutputCount(item)"
                    >
                      <strong>{{ item }}</strong>
                      <small>×{{ item }}</small>
                    </button>
                  </div>
                </div>

                <div class="image-studio-field image-studio-choice-section">
                  <span>{{ t('imageStudio.outputFormat') }}</span>
                  <div class="image-studio-choice-picker image-studio-format-picker">
                    <button
                      v-for="item in outputFormatOptions"
                      :key="item.value"
                      type="button"
                      :data-testid="`image-studio-output-format-${item.value}`"
                      :class="{ active: outputFormat === item.value }"
                      :disabled="submitting || !canUse"
                      @click="chooseOutputFormat(item.value)"
                    >
                      <strong>{{ item.label }}</strong>
                    </button>
                  </div>
                </div>

                <div class="image-studio-field image-studio-choice-section">
                  <span>{{ t('imageStudio.outputBackground') }}</span>
                  <div class="image-studio-choice-picker image-studio-background-picker">
                    <button
                      v-for="item in outputBackgroundOptions"
                      :key="item.value"
                      type="button"
                      :data-testid="`image-studio-output-background-${item.value}`"
                      :class="{ active: outputBackground === item.value }"
                      :disabled="submitting || !canUse || isOutputBackgroundDisabled(item.value)"
                      @click="chooseOutputBackground(item.value)"
                    >
                      <strong>{{ t(item.labelKey) }}</strong>
                    </button>
                  </div>
                </div>

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
                      <em v-if="item.quality === '4K'">{{ t('imageStudio.quality4KRisk') }}</em>
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

              <div class="image-studio-foundation-row">
                <div class="image-studio-step-heading image-studio-step-heading-full">
                  <span>{{ mode === 'edit' ? '03' : '02' }}</span>
                  <strong>{{ t('imageStudio.stepConnection') }}</strong>
                </div>
                <div class="image-studio-field image-studio-select-field">
                  <span>{{ t('imageStudio.apiKey') }}</span>
                  <div
                    class="image-studio-select"
                    data-testid="image-studio-api-key-select-root"
                    :class="{
                      'is-open': openSelect === 'apiKey',
                      'is-disabled': submitting || !canUse,
                      'is-drop-up': selectPlacement.apiKey === 'up',
                    }"
                    @click.stop
                  >
                    <button
                      type="button"
                      class="image-studio-select-trigger"
                      data-testid="image-studio-api-key-select"
                      :disabled="submitting || !canUse"
                      aria-haspopup="listbox"
                      :aria-expanded="openSelect === 'apiKey'"
                      @click="toggleSelect('apiKey', $event)"
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
                    data-testid="image-studio-model-select-root"
                    :class="{
                      'is-open': openSelect === 'model',
                      'is-disabled': submitting || !canUse,
                      'is-drop-up': selectPlacement.model === 'up',
                    }"
                    @click.stop
                  >
                    <button
                      type="button"
                      class="image-studio-select-trigger"
                      data-testid="image-studio-model-select"
                      :disabled="submitting || !canUse"
                      aria-haspopup="listbox"
                      :aria-expanded="openSelect === 'model'"
                      @click="toggleSelect('model', $event)"
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
            </section>
          </div>

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
          </form>
        </div>

        <div class="image-studio-canvas-stage" data-testid="image-studio-canvas-stage">
          <aside class="image-studio-preview-panel image-studio-stage-panel">
            <div class="image-studio-panel-header">
              <div>
                <p class="image-studio-section-label">{{ t('imageStudio.resultPanel') }}</p>
                <h2>{{ t('imageStudio.latestResult') }}</h2>
              </div>
              <div class="image-studio-stage-meta">
                <span>{{ model || '-' }}</span>
                <span>{{ aspectRatio }}</span>
                <button type="button" class="image-studio-icon-button" :disabled="loadingHistory" @click="loadHistory">
                  <Icon name="refresh" size="md" :class="loadingHistory ? 'animate-spin' : ''" />
                </button>
              </div>
            </div>

            <div class="image-studio-preview-frame">
              <div v-if="submitting" class="image-studio-generating-state">
                <div class="image-studio-loader"></div>
                <strong>{{ t('imageStudio.generatingTitle') }}</strong>
                <span>{{ activeTaskHint }}</span>
              </div>
              <div
                v-else-if="generationFailure"
                class="image-studio-failure-state"
                data-testid="image-studio-failure-panel"
                role="alert"
              >
                <div class="image-studio-failure-icon">
                  <Icon name="exclamationCircle" size="lg" />
                </div>
                <strong>{{ t('imageStudio.failureTitle') }}</strong>
                <span>{{ failureDescription }}</span>
                <small>{{ t('imageStudio.failureNotCharged') }}</small>
                <div class="image-studio-failure-actions">
                  <button
                    type="button"
                    data-testid="image-studio-retry-button"
                    :disabled="submitting || !canSubmit"
                    @click="retryGeneration"
                  >
                    <Icon name="refresh" size="sm" />
                    {{ t('imageStudio.retry') }}
                  </button>
                  <button
                    type="button"
                    data-testid="image-studio-retry-1k-button"
                    :disabled="submitting || !canUse || !qualityOptions.some((item) => item.quality === '1K')"
                    @click="retryAtLowQuality"
                  >
                    <Icon name="sparkles" size="sm" />
                    {{ t('imageStudio.retryAt1K') }}
                  </button>
                </div>
              </div>
              <button
                v-else-if="currentImage"
                type="button"
                class="image-studio-preview-open"
                data-testid="image-studio-current-preview"
                :aria-label="t('imageStudio.openPreview')"
                @click="openImagePreview(currentImage)"
              >
                <img :src="currentImage.image_url" :alt="currentImage.prompt" />
                <span>
                  <Icon name="eye" size="sm" />
                  {{ t('imageStudio.previewImage') }}
                </span>
              </button>
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
                <button
                  type="button"
                  class="image-studio-image-thumb"
                  :aria-label="t('imageStudio.openPreview')"
                  @click="openImagePreview(item)"
                >
                  <img :src="item.image_url" :alt="item.prompt" loading="lazy" />
                  <span>
                    <Icon name="eye" size="sm" />
                    {{ t('imageStudio.previewImage') }}
                  </span>
                </button>
              </article>
            </div>
            <div v-else class="image-studio-empty-gallery">
              <Icon name="inbox" size="xl" />
              <strong>{{ t('imageStudio.emptyGalleryTitle') }}</strong>
              <span>{{ t('imageStudio.emptyGalleryHint') }}</span>
            </div>
          </section>
        </div>
      </div>

      <div
        v-if="previewImage"
        class="image-studio-preview-modal"
        data-testid="image-studio-image-preview-dialog"
        role="dialog"
        aria-modal="true"
        :aria-label="t('imageStudio.previewDialogTitle')"
        @click.self="closeImagePreview"
      >
        <div class="image-studio-preview-dialog">
          <div class="image-studio-preview-toolbar">
            <div>
              <p class="image-studio-section-label">{{ t('imageStudio.previewDialogTitle') }}</p>
              <h2>{{ previewImage.model }}</h2>
            </div>
            <button
              type="button"
              class="image-studio-preview-close"
              :aria-label="t('common.close')"
              @click="closeImagePreview"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <div class="image-studio-preview-canvas">
            <img :src="previewImage.image_url" :alt="previewImage.prompt" />
          </div>

          <div class="image-studio-preview-details">
            <div>
              <span>{{ t('imageStudio.promptLabel') }}</span>
              <strong>{{ previewImage.prompt }}</strong>
            </div>
            <div class="image-studio-preview-meta-grid">
              <span>{{ previewImage.aspect_ratio }} / {{ previewImage.size }}</span>
              <span>{{ formatCurrency(previewImage.cost || 0) }}</span>
              <span>{{ formatDateTime(previewImage.created_at) || '-' }}</span>
            </div>
          </div>

          <div class="image-studio-preview-actions">
            <button type="button" data-testid="image-studio-preview-copy" @click="copyImageURL(previewImage)">
              <Icon name="copy" size="sm" />
              {{ t('imageStudio.copyLink') }}
            </button>
            <button type="button" data-testid="image-studio-preview-download" @click="downloadImage(previewImage)">
              <Icon name="download" size="sm" />
              {{ t('imageStudio.download') }}
            </button>
            <button type="button" data-testid="image-studio-preview-reference" @click="usePreviewAsReference">
              <Icon name="edit" size="sm" />
              {{ t('imageStudio.useAsReference') }}
            </button>
            <button
              type="button"
              class="image-studio-preview-delete-action"
              data-testid="image-studio-preview-delete"
              @click="requestDeletePreviewImage"
            >
              <Icon name="trash" size="sm" />
              {{ t('imageStudio.deleteAction') }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="pendingDeleteImage"
        class="image-studio-delete-modal"
        data-testid="image-studio-delete-dialog"
        role="dialog"
        aria-modal="true"
        :aria-label="t('imageStudio.deleteTitle')"
        @click.self="closeDeleteDialog"
      >
        <div class="image-studio-delete-dialog">
          <div class="image-studio-delete-visual">
            <img :src="pendingDeleteImage.image_url" :alt="pendingDeleteImage.prompt" />
          </div>
          <div class="image-studio-delete-content">
            <div class="image-studio-delete-icon">
              <Icon name="trash" size="md" />
            </div>
            <p class="image-studio-section-label">{{ t('imageStudio.deleteKicker') }}</p>
            <h2>{{ t('imageStudio.deleteTitle') }}</h2>
            <p>{{ t('imageStudio.deleteDescription') }}</p>
            <div class="image-studio-delete-summary">
              <strong>{{ pendingDeleteImage.prompt }}</strong>
              <span>{{ pendingDeleteImage.model }} / {{ pendingDeleteImage.aspect_ratio }} / {{ pendingDeleteImage.size }}</span>
            </div>
            <div class="image-studio-delete-actions">
              <button type="button" class="image-studio-delete-cancel" :disabled="deletingImage" @click="closeDeleteDialog">
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="image-studio-delete-confirm"
                data-testid="image-studio-delete-confirm"
                :disabled="deletingImage"
                @click="confirmDeleteImage"
              >
                <svg v-if="deletingImage" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
                <Icon v-else name="trash" size="sm" />
                {{ t('imageStudio.deleteAction') }}
              </button>
            </div>
          </div>
        </div>
      </div>
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
  type ImageStudioGeneratePayload,
  type ImageStudioImage,
  type ImageStudioMode,
  type ImageStudioModelOption,
  type ImageStudioOptions,
  type ImageStudioQualityOption,
  type ImageStudioTask,
} from '@/api/images'
import { keysAPI } from '@/api/keys'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { ApiKey } from '@/types'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { formatBytes, formatCurrency, formatDateTime } from '@/utils/format'

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
type ImageStudioOutputCount = 1 | 2 | 3 | 4
type ImageStudioOutputFormat = 'png' | 'jpeg' | 'webp'
type ImageStudioBackground = 'auto' | 'opaque' | 'transparent'
const outputCount = ref<ImageStudioOutputCount>(1)
const outputFormat = ref<ImageStudioOutputFormat>('png')
const outputBackground = ref<ImageStudioBackground>('auto')
const referenceFiles = ref<File[]>([])
const referencePreviewUrls = new Map<File, string>()
const loadingConfig = ref(false)
const loadingHistory = ref(false)
const submitting = ref(false)
const dragging = ref(false)
const loadError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const totalImages = ref(0)
type ImageStudioSelectMenu = 'apiKey' | 'model'
type ImageStudioSelectPlacement = 'down' | 'up'
const openSelect = ref<ImageStudioSelectMenu | null>(null)
const selectPlacement = ref<Record<ImageStudioSelectMenu, ImageStudioSelectPlacement>>({
  apiKey: 'down',
  model: 'down',
})
const activeTask = ref<ImageStudioTask | null>(null)
const taskPollTimer = ref<number | null>(null)
const previewImage = ref<ImageStudioImage | null>(null)
const pendingDeleteImage = ref<ImageStudioImage | null>(null)
const deletingImage = ref(false)
const generationFailure = ref<{
  message: string
  reason?: string
} | null>(null)
const outputCountOptions = [1, 2, 3, 4] as const
const outputFormatOptions: Array<{ value: ImageStudioOutputFormat; label: string }> = [
  { value: 'png', label: 'PNG' },
  { value: 'jpeg', label: 'JPEG' },
  { value: 'webp', label: 'WebP' },
]
const outputBackgroundOptions: Array<{ value: ImageStudioBackground; labelKey: string }> = [
  { value: 'auto', labelKey: 'imageStudio.backgroundAuto' },
  { value: 'opaque', labelKey: 'imageStudio.backgroundOpaque' },
  { value: 'transparent', labelKey: 'imageStudio.backgroundTransparent' },
]

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
const selectedUnitCost = computed(() => selectedPricePreview.value?.estimated_cost ?? selectedQuality.value?.estimated_cost)
const estimatedCostLabel = computed(() => {
  const cost = selectedUnitCost.value
  return typeof cost === 'number' ? formatCurrency(cost * outputCount.value) : '-'
})
const selectedPreviewMeta = computed(() => {
  const size = selectedPricePreview.value?.size ?? selectedRatio.value?.size ?? '-'
  const tier =
    selectedPricePreview.value?.billing_tier ??
    selectedQuality.value?.billing_tier ??
    selectedRatio.value?.billing_tier ??
    '-'
  if (tier === '4K') {
    return `${size} / ${tier} ×${outputCount.value} · ${t('imageStudio.quality4KInlineHint')}`
  }
  return `${size} / ${tier} ×${outputCount.value}`
})
const failureDescription = computed(() => {
  if (!generationFailure.value) return ''
  if (generationFailure.value.reason === 'IMAGE_PROVIDER_TIMEOUT_OR_DISCONNECT') {
    return t('imageStudio.failureTimeoutDescription')
  }
  return generationFailure.value.message || t('imageStudio.failureGenericDescription')
})
const activeTaskHint = computed(() => {
  if (!activeTask.value) return t('imageStudio.generatingHint')
  if (activeTask.value.status === 'queued') return t('imageStudio.taskQueuedHint')
  if (activeTask.value.status === 'running') {
    if (activeTask.value.quality === '4K') {
      return t('imageStudio.taskRunningHighQualityHint')
    }
    return t('imageStudio.taskRunningHint')
  }
  return t('imageStudio.generatingHint')
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

watch([outputFormat, model], () => {
  if (isOutputBackgroundDisabled('transparent') && outputBackground.value === 'transparent') {
    outputBackground.value = 'auto'
  }
}, { immediate: true })

function toggleSelect(menu: ImageStudioSelectMenu, event?: MouseEvent) {
  if (openSelect.value === menu) {
    openSelect.value = null
    return
  }
  selectPlacement.value = {
    ...selectPlacement.value,
    [menu]: resolveSelectPlacement(event?.currentTarget),
  }
  openSelect.value = menu
}

function closeSelectMenus() {
  openSelect.value = null
}

function resolveSelectPlacement(target: EventTarget | null | undefined): ImageStudioSelectPlacement {
  if (!(target instanceof HTMLElement)) return 'down'
  const rect = target.getBoundingClientRect()
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight
  const preferredMenuHeight = 240
  const availableBelow = viewportHeight - rect.bottom
  const availableAbove = rect.top
  return availableBelow < preferredMenuHeight && availableAbove > availableBelow ? 'up' : 'down'
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
  generationFailure.value = null
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
    await resumeLatestUnfinishedGenerationTask()
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
  stopTaskPolling()
  activeTask.value = null
  generationFailure.value = null
  try {
    const normalizedPrompt = prompt.value.trim()
    const payload = {
      api_key_id: selectedAPIKeyID.value,
      group_id: selectedGroup.value?.id ?? null,
      model: model.value,
      prompt: normalizedPrompt,
      aspect_ratio: aspectRatio.value,
      quality: quality.value,
      ...selectedOutputPayload(),
    }
    const batch = await runImageBatch(outputCount.value, async () => {
      const created =
        mode.value === 'edit'
          ? await imageStudioAPI.edit({
              ...payload,
              images: referenceFiles.value,
            })
          : await createAndPollGenerationTask(payload)
      applyGeneratedImage(created)
      return created
    })

    if (batch.created.length > 0) {
      await authStore.refreshUser().catch(() => undefined)
      appStore.showSuccess(t('imageStudio.generateSuccess'))
    }
    if (batch.errors.length > 0) {
      const error = batch.errors[0]
      const message = extractApiErrorMessage(error, t('imageStudio.generateFailed'))
      if (batch.created.length === 0) {
        generationFailure.value = {
          message,
          reason: extractApiErrorCode(error),
        }
      }
      appStore.showError(message)
    }
  } catch (error) {
    const message = extractApiErrorMessage(error, t('imageStudio.generateFailed'))
    generationFailure.value = {
      message,
      reason: extractApiErrorCode(error),
    }
    appStore.showError(message)
  } finally {
    submitting.value = false
  }
}

async function runImageBatch(
  count: ImageStudioOutputCount,
  worker: () => Promise<ImageStudioImage>,
): Promise<{ created: ImageStudioImage[]; errors: unknown[] }> {
  const created: ImageStudioImage[] = []
  const errors: unknown[] = []
  let next = 0
  const concurrency = Math.min(2, count)
  async function runWorker() {
    while (next < count) {
      next += 1
      try {
        created.push(await worker())
      } catch (error) {
        errors.push(error)
      }
    }
  }
  await Promise.all(Array.from({ length: concurrency }, runWorker))
  return { created, errors }
}

async function resumeLatestUnfinishedGenerationTask() {
  if (submitting.value || !canUse.value) return
  try {
    const response = await imageStudioAPI.listTasks({ page: 1, page_size: 5 })
    const task = (response.items ?? []).find(isUnfinishedGenerationTask)
    if (!task) return

    restoreTaskFormState(task)
    generationFailure.value = null
    activeTask.value = task
    submitting.value = true
    void finishRecoveredGenerationTask(task.id)
  } catch {
    // Task recovery is opportunistic; config/history load should stay usable if it fails.
  }
}

function isUnfinishedGenerationTask(task: ImageStudioTask) {
  return task.mode === 'generation' && (task.status === 'queued' || task.status === 'running')
}

function restoreTaskFormState(task: ImageStudioTask) {
  mode.value = 'generation'
  prompt.value = task.prompt || prompt.value
  if (task.api_key_id && availableAPIKeys.value.some((item) => item.id === task.api_key_id)) {
    selectedAPIKeyID.value = task.api_key_id
  }
  if (modelOptions.value.some((item) => item.model === task.model)) {
    model.value = task.model
  }
  if (aspectRatioOptions.value.some((item) => item.ratio === task.aspect_ratio)) {
    aspectRatio.value = task.aspect_ratio
  }
  if (qualityOptions.value.some((item) => item.quality === task.quality)) {
    quality.value = task.quality
  }
  if (isImageStudioOutputFormat(task.output_format)) {
    outputFormat.value = task.output_format
  }
  if (isImageStudioBackground(task.background)) {
    outputBackground.value = task.background
  }
}

async function finishRecoveredGenerationTask(taskID: number) {
  try {
    const created = await waitForImageTask(taskID)
    applyGeneratedImage(created)
    await authStore.refreshUser().catch(() => undefined)
    appStore.showSuccess(t('imageStudio.generateSuccess'))
  } catch (error) {
    const message = extractApiErrorMessage(error, t('imageStudio.generateFailed'))
    generationFailure.value = {
      message,
      reason: extractApiErrorCode(error),
    }
    appStore.showError(message)
  } finally {
    submitting.value = false
  }
}

function applyGeneratedImage(created: ImageStudioImage) {
  const alreadyListed = images.value.some((item) => item.id === created.id)
  currentImage.value = created
  images.value = [created, ...images.value.filter((item) => item.id !== created.id)].slice(0, 12)
  if (!alreadyListed) {
    totalImages.value += 1
  }
}

async function createAndPollGenerationTask(payload: {
  api_key_id: number | null
  group_id: number | null
  model: string
  prompt: string
  aspect_ratio: string
  quality: string
  output_format?: string
  background?: string
}): Promise<ImageStudioImage> {
  const task = await imageStudioAPI.createTask({
    mode: 'generation',
    ...payload,
  })
  activeTask.value = task
  return await waitForImageTask(task.id)
}

async function waitForImageTask(taskID: number): Promise<ImageStudioImage> {
  const deadline = Date.now() + 10 * 60 * 1000
  let delay = 1200
  while (Date.now() < deadline) {
    await sleep(delay)
    const task = await imageStudioAPI.getTask(taskID)
    activeTask.value = task
    if (task.status === 'succeeded') {
      if (task.image) {
        return task.image
      }
      throw new Error(t('imageStudio.taskMissingImage'))
    }
    if (task.status === 'failed') {
      throw {
        reason: task.error_reason,
        message: task.error_message || t('imageStudio.generateFailed'),
      }
    }
    delay = Math.min(3000, delay + 400)
  }
  throw {
    reason: 'IMAGE_TASK_POLL_TIMEOUT',
    message: t('imageStudio.taskPollTimeout'),
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    taskPollTimer.value = window.setTimeout(() => {
      taskPollTimer.value = null
      resolve()
    }, ms)
  })
}

function stopTaskPolling() {
  if (taskPollTimer.value != null) {
    window.clearTimeout(taskPollTimer.value)
    taskPollTimer.value = null
  }
}

async function retryGeneration() {
  await handleSubmit()
}

async function retryAtLowQuality() {
  if (!qualityOptions.value.some((item) => item.quality === '1K')) return
  quality.value = '1K'
  await handleSubmit()
}

function addReferenceFiles(files: File[]) {
  const maxBytes = maxReferenceImageMB.value * 1024 * 1024
  const accepted = files.filter((file) => file.type.startsWith('image/') && file.size <= maxBytes)
  const rejected = files.length - accepted.length
  if (rejected > 0) {
    appStore.showError(t('imageStudio.referenceRejected', { count: rejected, mb: maxReferenceImageMB.value }))
  }
  const nextFiles = [...referenceFiles.value, ...accepted].slice(0, 4)
  pruneReferencePreviewURLs(nextFiles)
  referenceFiles.value = nextFiles
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
  revokeReferencePreviewURL(file)
}

function referencePreviewURL(file: File): string {
  const existing = referencePreviewUrls.get(file)
  if (existing) return existing
  if (typeof URL.createObjectURL !== 'function') return ''
  const url = URL.createObjectURL(file)
  referencePreviewUrls.set(file, url)
  return url
}

function revokeReferencePreviewURL(file: File) {
  const url = referencePreviewUrls.get(file)
  if (!url) return
  URL.revokeObjectURL(url)
  referencePreviewUrls.delete(file)
}

function pruneReferencePreviewURLs(nextFiles: File[]) {
  const nextSet = new Set(nextFiles)
  for (const file of Array.from(referencePreviewUrls.keys())) {
    if (!nextSet.has(file)) {
      revokeReferencePreviewURL(file)
    }
  }
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

function chooseOutputCount(count: number) {
  if (count === 1 || count === 2 || count === 3 || count === 4) {
    outputCount.value = count
  }
}

function chooseOutputFormat(format: ImageStudioOutputFormat) {
  outputFormat.value = format
}

function chooseOutputBackground(background: ImageStudioBackground) {
  if (isOutputBackgroundDisabled(background)) return
  outputBackground.value = background
}

function isOutputBackgroundDisabled(background: ImageStudioBackground) {
  return background === 'transparent' && (outputFormat.value === 'jpeg' || !supportsTransparentBackground(model.value))
}

function selectedOutputPayload(): Pick<ImageStudioGeneratePayload, 'output_format' | 'background'> {
  const payload: Pick<ImageStudioGeneratePayload, 'output_format' | 'background'> = {}
  if (outputFormat.value !== 'png') {
    payload.output_format = outputFormat.value
  }
  if (outputBackground.value !== 'auto') {
    payload.background = outputBackground.value
  }
  return payload
}

function isImageStudioOutputFormat(value: unknown): value is ImageStudioOutputFormat {
  return value === 'png' || value === 'jpeg' || value === 'webp'
}

function isImageStudioBackground(value: unknown): value is ImageStudioBackground {
  return value === 'auto' || value === 'opaque' || value === 'transparent'
}

function supportsTransparentBackground(candidateModel: string) {
  return candidateModel.trim().toLowerCase() !== 'gpt-image-2'
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
  anchor.download = `passion-api-image-${image.id}${imageFileExtension(image, image.mime_type || '')}`
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
    const nextFiles = [file, ...referenceFiles.value.filter((item) => item.name !== file.name)].slice(0, 4)
    pruneReferencePreviewURLs(nextFiles)
    referenceFiles.value = nextFiles
    appStore.showSuccess(t('imageStudio.referenceModeHint'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('imageStudio.referenceLoadFailed')))
  }
}

async function usePreviewAsReference() {
  if (!previewImage.value) return
  const image = previewImage.value
  closeImagePreview()
  await useAsReference(image)
}

function requestDeletePreviewImage() {
  if (!previewImage.value) return
  const image = previewImage.value
  closeImagePreview()
  requestDeleteImage(image)
}

function openImagePreview(image: ImageStudioImage) {
  previewImage.value = image
}

function closeImagePreview() {
  previewImage.value = null
}

function handlePreviewKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (pendingDeleteImage.value) {
    closeDeleteDialog()
    return
  }
  if (previewImage.value) {
    closeImagePreview()
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

function requestDeleteImage(image: ImageStudioImage) {
  pendingDeleteImage.value = image
}

function closeDeleteDialog() {
  if (deletingImage.value) return
  pendingDeleteImage.value = null
}

async function confirmDeleteImage() {
  if (!pendingDeleteImage.value) return
  const image = pendingDeleteImage.value

  deletingImage.value = true
  try {
    await imageStudioAPI.delete(image.id)
    images.value = images.value.filter((item) => item.id !== image.id)
    if (currentImage.value?.id === image.id) {
      currentImage.value = images.value[0] ?? null
    }
    if (previewImage.value?.id === image.id) {
      previewImage.value = null
    }
    totalImages.value = Math.max(0, totalImages.value - 1)
    pendingDeleteImage.value = null
    appStore.showSuccess(t('imageStudio.deleteSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('imageStudio.deleteFailed')))
  } finally {
    deletingImage.value = false
  }
}

onMounted(() => {
  void loadInitialData()
  document.addEventListener('click', closeSelectMenus)
  document.addEventListener('keydown', handlePreviewKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeSelectMenus)
  document.removeEventListener('keydown', handlePreviewKeydown)
  stopTaskPolling()
  pruneReferencePreviewURLs([])
})
</script>

<style scoped>
.image-studio-shell {
  display: grid;
  max-width: min(100%, 118rem);
  gap: clamp(0.9rem, 1.35vw, 1.25rem);
  height: var(--studio-viewport-height);
  min-height: 0;
  overflow: hidden;
  padding-bottom: 0;
  --studio-viewport-height: calc(100dvh - 4rem - var(--app-content-padding-total-y, 2rem));
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
  gap: clamp(1rem, 1.6vw, 1.45rem);
  min-height: 0;
}

.image-studio-workbench {
  height: 100%;
  min-height: 0;
  align-items: stretch;
  grid-template-columns: minmax(18.5rem, 0.56fr) minmax(38rem, 1.44fr);
}

.image-studio-control-console {
  z-index: 3;
  height: 100%;
  min-width: 0;
  min-height: 0;
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
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr) auto;
  overflow: hidden;
  overscroll-behavior: contain;
  scrollbar-color: rgba(var(--brand-rgb), 0.32) transparent;
  scrollbar-width: thin;
}

.image-studio-preview-panel {
  display: grid;
  height: 100%;
  min-width: 0;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr) auto;
}

.image-studio-workspace::before,
.image-studio-preview-panel::before,
.image-studio-gallery::before {
  display: none;
}

.image-studio-workspace,
.image-studio-preview-panel {
  padding: clamp(0.9rem, 1.25vw, 1.15rem);
}

.image-studio-gallery {
  display: grid;
  min-height: 0;
  max-height: clamp(11rem, 22dvh, 15rem);
  grid-template-rows: auto minmax(0, 1fr);
  padding: clamp(0.82rem, 1.1vw, 1rem);
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

.image-studio-control-console .image-studio-panel-header {
  align-items: stretch;
  flex-direction: column;
}

.image-studio-control-console .image-studio-mode-switch {
  width: 100%;
}

.image-studio-control-console .image-studio-mode-switch button {
  flex: 1 1 0;
  justify-content: center;
}

.image-studio-section-label {
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0;
  text-transform: uppercase;
  color: var(--brand-700);
}

.image-studio-step-heading {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  color: var(--studio-text);
}

.image-studio-step-heading-full {
  grid-column: 1 / -1;
}

.image-studio-step-heading span {
  display: inline-grid;
  width: 1.62rem;
  height: 1.62rem;
  place-items: center;
  border-radius: 0.5rem;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(15, 23, 42, 0.04);
  color: rgb(71, 85, 105);
  font-size: 0.68rem;
  font-weight: 850;
}

.image-studio-step-heading strong {
  font-size: 0.9rem;
  font-weight: 820;
}

.image-studio-mode-switch {
  display: inline-flex;
  flex-wrap: nowrap;
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
  padding: 0.42rem 0.62rem;
  color: var(--studio-subtle);
  font-size: 0.78rem;
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
  height: 100%;
  min-height: clamp(9rem, 24dvh, 15rem);
  resize: none;
  padding: 0.92rem;
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
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  gap: 0;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding-right: 0.12rem;
  scrollbar-color: rgba(var(--brand-rgb), 0.32) transparent;
  scrollbar-width: thin;
}

.image-studio-prompt-section,
.image-studio-action-bar {
  margin-top: 0.95rem;
}

.image-studio-prompt-section {
  margin-top: 0;
  display: grid;
  min-height: 0;
  flex: 0 0 auto;
  gap: 0.7rem;
}

.image-studio-prompt-field {
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr);
}

.image-studio-control-dock {
  flex: 0 0 auto;
  margin-top: 0.95rem;
  display: grid;
  gap: 0.8rem;
  border-top: 1px solid rgba(203, 213, 225, 0.62);
  background: transparent;
  padding-top: 0.85rem;
}

.image-studio-foundation-row {
  order: 3;
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.75rem;
  align-items: start;
}

.image-studio-output-row {
  order: 2;
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.8rem;
  align-items: start;
  border-top: 1px solid rgba(203, 213, 225, 0.5);
  padding-top: 0.8rem;
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

.image-studio-select.is-drop-up .image-studio-select-menu {
  top: auto;
  bottom: calc(100% + 0.45rem);
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

.image-studio-select.is-drop-up .image-studio-select-pop-enter-from,
.image-studio-select.is-drop-up .image-studio-select-pop-leave-to {
  transform: translateY(4px);
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
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.image-studio-quality-picker {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.image-studio-count-picker {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.image-studio-format-picker,
.image-studio-background-picker {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.image-studio-choice-picker button {
  min-height: 3.05rem;
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

.image-studio-choice-picker button:disabled {
  cursor: not-allowed;
  opacity: 0.46;
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
.image-studio-choice-picker small,
.image-studio-choice-picker em {
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

.image-studio-choice-picker em {
  margin-top: 0.18rem;
  color: rgb(217, 119, 6);
  font-size: 0.63rem;
  font-style: normal;
  font-weight: 780;
  line-height: 1.2;
}

.image-studio-choice-picker button.active small {
  color: rgba(29, 78, 216, 0.72);
}

.image-studio-choice-picker button.active em {
  color: rgb(180, 83, 9);
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
  order: 1;
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
  margin-bottom: 1rem;
}

.image-studio-reference-item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.65rem;
  border-radius: var(--studio-radius-sm);
  border: 1px solid var(--studio-border-soft);
  background: rgba(255, 255, 255, 0.78);
  padding: 0.42rem 0.5rem;
  color: rgb(51, 65, 85);
}

.image-studio-reference-thumb {
  width: 3.25rem;
  height: 3.25rem;
  border-radius: 0.85rem;
  border: 1px solid rgba(191, 219, 254, 0.88);
  background: rgba(239, 246, 255, 0.72);
  object-fit: cover;
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.72) inset,
    0 10px 24px rgba(15, 23, 42, 0.1);
}

.image-studio-reference-item small {
  color: rgb(100, 116, 139);
  font-size: 0.75rem;
}

.image-studio-reference-item button {
  display: grid;
  width: 1.9rem;
  height: 1.9rem;
  place-items: center;
  border-radius: 999px;
  color: rgb(71, 85, 105);
  transition:
    background 180ms ease,
    color 180ms ease,
    transform 180ms ease;
}

.image-studio-reference-item button:hover,
.image-studio-reference-item button:focus-visible {
  background: rgba(239, 68, 68, 0.1);
  color: rgb(220, 38, 38);
  transform: translateY(-1px);
}

.image-studio-action-bar {
  position: relative;
  z-index: 1;
  display: grid;
  flex: 0 0 auto;
  align-self: end;
  grid-template-columns: 1fr;
  align-items: stretch;
  gap: 0.72rem;
  border: 1px solid rgba(var(--brand-rgb), 0.2);
  border-radius: 1rem;
  background:
    linear-gradient(
      180deg,
      rgba(255, 255, 255, 0.98) 0%,
      rgba(248, 250, 252, 0.98) 52%,
      rgba(239, 246, 255, 0.98) 100%
    ),
    rgb(255, 255, 255);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.9) inset,
    0 14px 34px rgba(37, 99, 235, 0.12),
    0 4px 12px rgba(15, 23, 42, 0.06);
  margin-top: 0.85rem;
  padding: 0.85rem;
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
  width: 100%;
  min-width: 0;
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

.image-studio-canvas-stage {
  display: grid;
  height: 100%;
  min-width: 0;
  min-height: 0;
  gap: clamp(0.85rem, 1.2vw, 1rem);
  grid-template-rows: minmax(0, 1fr) auto;
}

.image-studio-stage-meta {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.4rem;
}

.image-studio-stage-meta > span {
  max-width: 12rem;
  overflow: hidden;
  border-radius: 9999px;
  background:
    linear-gradient(135deg, rgba(var(--brand-rgb), 0.08), rgba(var(--brand-cyan-rgb), 0.12)),
    rgba(239, 246, 255, 0.7);
  color: var(--brand-700);
  padding: 0.38rem 0.62rem;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.74rem;
  font-weight: 820;
}

.image-studio-preview-frame {
  margin-top: 0.95rem;
  display: grid;
  height: 100%;
  min-height: 0;
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

.image-studio-stage-panel .image-studio-preview-open img {
  height: auto;
  width: auto;
  max-height: min(100%, clamp(20rem, 54dvh, 36rem));
  max-width: min(100%, clamp(26rem, 56vw, 56rem));
  border-radius: 0.8rem;
  object-fit: contain;
  object-position: center;
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.16);
}

.image-studio-preview-open,
.image-studio-image-thumb {
  position: relative;
  display: grid;
  height: 100%;
  width: 100%;
  place-items: center;
  overflow: hidden;
  color: white;
}

.image-studio-stage-panel .image-studio-preview-open {
  display: flex;
  height: 100%;
  width: 100%;
  max-height: 100%;
  max-width: 100%;
  align-items: center;
  justify-content: center;
  border-radius: 0.9rem;
  background: rgba(255, 255, 255, 0.46);
  padding: clamp(0.9rem, 1.4vw, 1.2rem);
  box-shadow:
    0 18px 42px rgba(15, 23, 42, 0.09),
    inset 0 1px 0 rgba(255, 255, 255, 0.64);
}

.image-studio-preview-open img,
.image-studio-image-thumb img {
  transition:
    filter 200ms ease,
    transform 240ms ease;
}

.image-studio-preview-open > span,
.image-studio-image-thumb > span {
  position: absolute;
  left: 50%;
  top: 50%;
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
  border: 1px solid rgba(255, 255, 255, 0.42);
  border-radius: 9999px;
  background:
    linear-gradient(135deg, rgba(var(--brand-rgb), 0.9), rgba(var(--brand-cyan-rgb), 0.84)),
    rgba(15, 23, 42, 0.62);
  padding: 0.48rem 0.72rem;
  font-size: 0.8rem;
  font-weight: 800;
  opacity: 0;
  pointer-events: none;
  transform: translate(-50%, -44%) scale(0.96);
  transition:
    opacity 180ms ease,
    transform 180ms ease;
  white-space: nowrap;
  box-shadow: 0 14px 28px rgba(15, 23, 42, 0.24);
}

.image-studio-preview-open:hover img,
.image-studio-preview-open:focus-visible img,
.image-studio-image-thumb:hover img,
.image-studio-image-thumb:focus-visible img {
  filter: saturate(1.08) brightness(0.78);
  transform: scale(1.025);
}

.image-studio-stage-panel .image-studio-preview-open:hover img,
.image-studio-stage-panel .image-studio-preview-open:focus-visible img {
  filter: saturate(1.04) brightness(0.9);
  transform: none;
}

.image-studio-preview-open:hover > span,
.image-studio-preview-open:focus-visible > span,
.image-studio-image-thumb:hover > span,
.image-studio-image-thumb:focus-visible > span {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1);
}

.image-studio-preview-open:focus-visible,
.image-studio-image-thumb:focus-visible {
  outline: 3px solid rgba(var(--brand-rgb), 0.35);
  outline-offset: -3px;
}

.image-studio-generating-state,
.image-studio-failure-state,
.image-studio-empty-preview,
.image-studio-empty-gallery {
  display: grid;
  place-items: center;
  gap: 0.55rem;
  padding: 2rem;
  text-align: center;
  color: rgb(71, 85, 105);
}

.image-studio-empty-preview {
  width: min(100%, 30rem);
  border-radius: 1.1rem;
  border: 1px solid rgba(var(--brand-rgb), 0.16);
  background:
    radial-gradient(circle at 50% 0%, rgba(var(--brand-cyan-rgb), 0.18), transparent 62%),
    rgba(255, 255, 255, 0.58);
  box-shadow:
    0 20px 48px rgba(37, 99, 235, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.image-studio-empty-gallery {
  min-height: 5.75rem;
  border-radius: 0.85rem;
  border: 1px dashed rgba(var(--brand-rgb), 0.22);
  background: rgba(239, 246, 255, 0.36);
}

.image-studio-generating-state strong,
.image-studio-failure-state strong,
.image-studio-empty-preview strong,
.image-studio-empty-gallery strong {
  color: rgb(15, 23, 42);
  font-weight: 800;
}

.image-studio-failure-state {
  width: min(100%, 26rem);
  justify-self: center;
  border-radius: 1rem;
  border: 1px solid rgba(var(--brand-rgb), 0.22);
  background:
    radial-gradient(circle at top left, rgba(var(--brand-cyan-rgb), 0.2), transparent 44%),
    linear-gradient(135deg, rgba(255, 255, 255, 0.94), rgba(239, 246, 255, 0.86));
  box-shadow:
    0 18px 44px rgba(37, 99, 235, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.image-studio-failure-state > span {
  max-width: 22rem;
  color: rgb(51, 65, 85);
  font-size: 0.88rem;
  line-height: 1.55;
}

.image-studio-failure-state > small {
  border-radius: 9999px;
  background: rgba(16, 185, 129, 0.1);
  color: rgb(4, 120, 87);
  padding: 0.34rem 0.62rem;
  font-size: 0.75rem;
  font-weight: 780;
}

.image-studio-failure-icon {
  display: inline-flex;
  height: 3rem;
  width: 3rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: linear-gradient(135deg, rgba(var(--brand-rgb), 0.14), rgba(var(--brand-cyan-rgb), 0.22));
  color: var(--brand-700);
  box-shadow: inset 0 0 0 1px rgba(var(--brand-rgb), 0.18);
}

.image-studio-failure-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.5rem;
  padding-top: 0.2rem;
}

.image-studio-failure-actions button {
  display: inline-flex;
  min-height: 2.35rem;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  border-radius: var(--studio-radius-sm);
  border: 1px solid rgba(var(--brand-rgb), 0.2);
  background: rgba(255, 255, 255, 0.76);
  color: var(--brand-700);
  padding: 0.48rem 0.75rem;
  font-size: 0.8rem;
  font-weight: 780;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease,
    background 180ms ease;
}

.image-studio-failure-actions button:hover:not(:disabled),
.image-studio-failure-actions button:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.5);
  background: white;
  box-shadow: 0 0 0 3px rgba(var(--brand-rgb), 0.12);
  transform: translateY(-1px);
}

.image-studio-failure-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.56;
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
  margin-top: clamp(1.35rem, 2vh, 1.8rem);
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
  margin-top: 0.72rem;
  display: grid;
  min-height: 0;
  grid-auto-columns: minmax(11rem, 13rem);
  grid-auto-flow: column;
  grid-template-columns: none;
  gap: 0.62rem;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 0.05rem 0.1rem 0.35rem;
  scroll-snap-type: x proximity;
  scrollbar-color: rgba(var(--brand-rgb), 0.32) transparent;
  scrollbar-width: thin;
}

.image-studio-gallery-loading div {
  min-height: 8.25rem;
  border-radius: 0.86rem;
  background: linear-gradient(90deg, rgba(226, 232, 240, 0.64), rgba(239, 246, 255, 0.9), rgba(226, 232, 240, 0.64));
  background-size: 220% 100%;
  animation: image-studio-shimmer 1.25s ease-in-out infinite;
}

.image-studio-image-card {
  position: relative;
  overflow: hidden;
  height: 100%;
  min-width: 0;
  border-radius: 0.9rem;
  border: 1px solid rgba(203, 213, 225, 0.58);
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.055);
  scroll-snap-align: start;
}

.image-studio-image-thumb {
  height: auto;
  aspect-ratio: 4 / 3;
  background: rgba(239, 246, 255, 0.9);
}

.image-studio-image-thumb img {
  display: block;
  width: 100%;
  aspect-ratio: 4 / 3;
  object-fit: cover;
}

.image-studio-image-card-body {
  display: grid;
  gap: 0.55rem;
  padding: 0.7rem;
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
  min-height: 2.35rem;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  color: rgb(71, 85, 105);
  font-size: 0.8rem;
  line-height: 1.55;
}

.image-studio-gallery-rail .image-studio-image-card-actions {
  flex-wrap: nowrap;
}

.image-studio-gallery-rail .image-studio-image-card-body {
  position: absolute;
  inset-inline: auto 0.42rem;
  top: 0.42rem;
  bottom: auto;
  display: block;
  width: max-content;
  max-width: calc(100% - 0.84rem);
  border-radius: 0.72rem;
  background: rgba(255, 255, 255, 0.82);
  padding: 0.25rem;
  backdrop-filter: blur(12px);
  box-shadow: 0 10px 22px rgba(15, 23, 42, 0.14);
}

.image-studio-gallery-rail .image-studio-image-card-body > div:first-child,
.image-studio-gallery-rail .image-studio-image-card-body p {
  display: none;
}

.image-studio-gallery-rail .image-studio-image-card-actions button {
  flex: 1 1 0;
  height: 2rem;
  min-height: 2rem;
  min-width: 0;
  padding-inline: 0.38rem;
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

.image-studio-preview-modal {
  position: fixed;
  z-index: 80;
  inset: 0;
  display: grid;
  place-items: center;
  background:
    radial-gradient(circle at 18% 12%, rgba(var(--brand-cyan-rgb), 0.2), transparent 28rem),
    radial-gradient(circle at 82% 20%, rgba(var(--brand-rgb), 0.22), transparent 30rem),
    rgba(15, 23, 42, 0.72);
  padding: clamp(0.75rem, 2vw, 1.5rem);
  backdrop-filter: blur(18px);
  animation: image-studio-preview-backdrop 160ms ease-out;
}

.image-studio-preview-dialog {
  display: grid;
  width: min(100%, 74rem);
  max-height: min(92vh, 58rem);
  overflow: hidden;
  border: 1px solid rgba(219, 234, 254, 0.34);
  border-radius: 1.2rem;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.88)),
    white;
  box-shadow:
    0 34px 90px rgba(15, 23, 42, 0.32),
    inset 0 1px 0 rgba(255, 255, 255, 0.92);
  animation: image-studio-preview-dialog-in 180ms ease-out;
}

.image-studio-preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgba(203, 213, 225, 0.56);
  padding: 0.95rem 1rem;
}

.image-studio-preview-toolbar h2 {
  margin-top: 0.1rem;
  color: var(--studio-text);
  font-size: 1.02rem;
  font-weight: 850;
}

.image-studio-preview-close {
  display: inline-flex;
  height: 2.45rem;
  width: 2.45rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  border: 1px solid rgba(203, 213, 225, 0.66);
  background: rgba(255, 255, 255, 0.74);
  color: var(--brand-700);
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease;
}

.image-studio-preview-close:hover,
.image-studio-preview-close:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.54);
  box-shadow: 0 0 0 3px rgba(var(--brand-rgb), 0.12);
  transform: translateY(-1px);
}

.image-studio-preview-canvas {
  display: grid;
  min-height: min(58vh, 38rem);
  overflow: auto;
  place-items: center;
  background:
    linear-gradient(45deg, rgba(219, 234, 254, 0.36) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(219, 234, 254, 0.36) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(219, 234, 254, 0.36) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(219, 234, 254, 0.36) 75%),
    rgba(241, 245, 249, 0.68);
  background-position: 0 0, 0 12px, 12px -12px, -12px 0;
  background-size: 24px 24px;
  padding: clamp(0.8rem, 2vw, 1.3rem);
}

.image-studio-preview-canvas img {
  display: block;
  max-height: min(58vh, 38rem);
  max-width: 100%;
  object-fit: contain;
  border-radius: 0.75rem;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.18);
}

.image-studio-preview-details {
  display: grid;
  gap: 0.7rem;
  border-top: 1px solid rgba(203, 213, 225, 0.56);
  padding: 0.9rem 1rem;
}

.image-studio-preview-details > div:first-child {
  display: grid;
  gap: 0.18rem;
}

.image-studio-preview-details span {
  color: rgb(100, 116, 139);
  font-size: 0.76rem;
  font-weight: 760;
}

.image-studio-preview-details strong {
  color: var(--studio-text);
  font-size: 0.9rem;
  line-height: 1.55;
}

.image-studio-preview-meta-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.image-studio-preview-meta-grid span {
  border-radius: 9999px;
  background: linear-gradient(135deg, rgba(var(--brand-rgb), 0.08), rgba(var(--brand-cyan-rgb), 0.12));
  color: var(--brand-700);
  padding: 0.36rem 0.64rem;
}

.image-studio-preview-actions {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.55rem;
  border-top: 1px solid rgba(203, 213, 225, 0.56);
  background: rgba(248, 250, 252, 0.72);
  padding: 0.85rem 1rem;
}

.image-studio-preview-actions button {
  display: inline-flex;
  min-height: 2.65rem;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  border-radius: var(--studio-radius-sm);
  border: 1px solid rgba(var(--brand-rgb), 0.22);
  background: rgba(255, 255, 255, 0.76);
  color: var(--brand-700);
  font-size: 0.84rem;
  font-weight: 800;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease,
    background 180ms ease;
}

.image-studio-preview-actions button:hover,
.image-studio-preview-actions button:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.52);
  background: white;
  box-shadow: 0 0 0 3px rgba(var(--brand-rgb), 0.12);
  transform: translateY(-1px);
}

.image-studio-preview-actions .image-studio-preview-delete-action {
  border-color: rgba(239, 68, 68, 0.24);
  color: rgb(220, 38, 38);
}

.image-studio-preview-actions .image-studio-preview-delete-action:hover,
.image-studio-preview-actions .image-studio-preview-delete-action:focus-visible {
  border-color: rgba(239, 68, 68, 0.42);
  background: rgba(254, 242, 242, 0.92);
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.12);
}

.image-studio-delete-modal {
  position: fixed;
  z-index: 90;
  inset: 0;
  display: grid;
  place-items: center;
  background:
    radial-gradient(circle at 22% 16%, rgba(239, 68, 68, 0.14), transparent 24rem),
    radial-gradient(circle at 78% 12%, rgba(var(--brand-rgb), 0.2), transparent 30rem),
    rgba(15, 23, 42, 0.66);
  padding: clamp(0.8rem, 2vw, 1.35rem);
  backdrop-filter: blur(16px);
  animation: image-studio-preview-backdrop 160ms ease-out;
}

.image-studio-delete-dialog {
  display: grid;
  grid-template-columns: minmax(11rem, 0.72fr) minmax(0, 1fr);
  width: min(100%, 42rem);
  overflow: hidden;
  border-radius: 1.15rem;
  border: 1px solid rgba(219, 234, 254, 0.42);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.92)),
    white;
  box-shadow:
    0 30px 80px rgba(15, 23, 42, 0.3),
    inset 0 1px 0 rgba(255, 255, 255, 0.92);
  animation: image-studio-preview-dialog-in 180ms ease-out;
}

.image-studio-delete-visual {
  min-height: 17rem;
  background:
    linear-gradient(45deg, rgba(219, 234, 254, 0.36) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(219, 234, 254, 0.36) 25%, transparent 25%),
    rgba(239, 246, 255, 0.72);
  background-size: 22px 22px;
}

.image-studio-delete-visual img {
  display: block;
  height: 100%;
  width: 100%;
  object-fit: cover;
}

.image-studio-delete-content {
  display: grid;
  align-content: start;
  gap: 0.72rem;
  padding: clamp(1rem, 2vw, 1.35rem);
}

.image-studio-delete-icon {
  display: inline-flex;
  height: 2.75rem;
  width: 2.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background:
    linear-gradient(135deg, rgba(239, 68, 68, 0.12), rgba(var(--brand-rgb), 0.1)),
    rgba(255, 255, 255, 0.86);
  color: rgb(220, 38, 38);
  box-shadow:
    inset 0 0 0 1px rgba(239, 68, 68, 0.18),
    0 10px 20px rgba(239, 68, 68, 0.1);
}

.image-studio-delete-content h2 {
  color: var(--studio-text);
  font-size: 1.25rem;
  font-weight: 850;
}

.image-studio-delete-content > p:not(.image-studio-section-label) {
  color: rgb(71, 85, 105);
  font-size: 0.9rem;
  line-height: 1.6;
}

.image-studio-delete-summary {
  display: grid;
  gap: 0.28rem;
  border-radius: 0.82rem;
  border: 1px solid rgba(203, 213, 225, 0.62);
  background: rgba(248, 250, 252, 0.8);
  padding: 0.72rem 0.8rem;
}

.image-studio-delete-summary strong {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  color: rgb(15, 23, 42);
  font-size: 0.88rem;
  line-height: 1.48;
}

.image-studio-delete-summary span {
  color: rgb(100, 116, 139);
  font-size: 0.75rem;
  font-weight: 760;
}

.image-studio-delete-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.55rem;
  padding-top: 0.25rem;
}

.image-studio-delete-actions button {
  display: inline-flex;
  min-height: 2.75rem;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  border-radius: var(--studio-radius-sm);
  font-size: 0.84rem;
  font-weight: 820;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    transform 180ms ease,
    background 180ms ease;
}

.image-studio-delete-cancel {
  border: 1px solid rgba(203, 213, 225, 0.78);
  background: rgba(255, 255, 255, 0.72);
  color: rgb(51, 65, 85);
}

.image-studio-delete-confirm {
  border: 1px solid rgba(37, 99, 235, 0.18);
  background:
    linear-gradient(135deg, var(--brand-600), var(--brand-500), var(--brand-cyan)),
    var(--brand-600);
  color: white;
  box-shadow: 0 12px 24px rgba(37, 99, 235, 0.18);
}

.image-studio-delete-actions button:hover:not(:disabled),
.image-studio-delete-actions button:focus-visible {
  box-shadow: 0 0 0 3px rgba(var(--brand-rgb), 0.12);
  transform: translateY(-1px);
}

.image-studio-delete-cancel:hover:not(:disabled),
.image-studio-delete-cancel:focus-visible {
  border-color: rgba(var(--brand-rgb), 0.38);
  background: white;
  color: var(--brand-700);
}

.image-studio-delete-confirm:hover:not(:disabled),
.image-studio-delete-confirm:focus-visible {
  border-color: rgba(255, 255, 255, 0.56);
  box-shadow:
    0 0 0 3px rgba(var(--brand-rgb), 0.14),
    0 16px 30px rgba(37, 99, 235, 0.22);
}

.image-studio-delete-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.62;
  transform: none;
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
.dark .image-studio-failure-state strong,
.dark .image-studio-empty-preview strong,
.dark .image-studio-empty-gallery strong,
.dark .image-studio-image-card-body strong {
  color: white;
}

.dark .image-studio-field > span,
.dark .image-studio-generating-state,
.dark .image-studio-failure-state,
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

.dark .image-studio-reference-thumb {
  border-color: rgba(96, 165, 250, 0.32);
  background: rgba(15, 23, 42, 0.92);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.04) inset,
    0 12px 26px rgba(0, 0, 0, 0.26);
}

.dark .image-studio-reference-item button {
  color: rgb(203, 213, 225);
}

.dark .image-studio-reference-item button:hover,
.dark .image-studio-reference-item button:focus-visible {
  background: rgba(248, 113, 113, 0.13);
  color: rgb(254, 202, 202);
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
    linear-gradient(
      180deg,
      rgba(15, 23, 42, 0.98) 0%,
      rgba(15, 23, 42, 0.98) 54%,
      rgba(23, 37, 84, 0.98) 100%
    ),
    rgb(15, 23, 42);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.06) inset,
    0 16px 38px rgba(0, 0, 0, 0.28),
    0 0 0 1px rgba(96, 165, 250, 0.06);
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

.dark .image-studio-choice-picker em,
.dark .image-studio-choice-picker button.active em {
  color: rgb(251, 191, 36);
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

.dark .image-studio-gallery-rail .image-studio-image-card-body {
  background: rgba(15, 23, 42, 0.82);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.28);
}

.dark .image-studio-mode-switch,
.dark .image-studio-icon-button,
.dark .image-studio-stage-meta > span,
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

.dark .image-studio-failure-state {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    radial-gradient(circle at top left, rgba(6, 182, 212, 0.18), transparent 44%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.94), rgba(2, 6, 23, 0.82));
  box-shadow:
    0 18px 44px rgba(0, 0, 0, 0.28),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.dark .image-studio-failure-state > span {
  color: rgb(203, 213, 225);
}

.dark .image-studio-failure-state > small {
  background: rgba(16, 185, 129, 0.14);
  color: rgb(110, 231, 183);
}

.dark .image-studio-failure-icon,
.dark .image-studio-failure-actions button {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.14);
  color: rgb(191, 219, 254);
}

.dark .image-studio-failure-actions button:hover:not(:disabled),
.dark .image-studio-failure-actions button:focus-visible {
  border-color: rgba(96, 165, 250, 0.42);
  background: rgba(30, 41, 59, 0.88);
}

.dark .image-studio-preview-dialog {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(2, 6, 23, 0.92)),
    rgb(15, 23, 42);
  box-shadow:
    0 34px 90px rgba(0, 0, 0, 0.48),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.dark .image-studio-preview-toolbar,
.dark .image-studio-preview-details,
.dark .image-studio-preview-actions {
  border-color: rgba(96, 165, 250, 0.16);
}

.dark .image-studio-preview-toolbar h2,
.dark .image-studio-preview-details strong {
  color: white;
}

.dark .image-studio-preview-close,
.dark .image-studio-preview-actions button {
  border-color: rgba(96, 165, 250, 0.2);
  background: rgba(37, 99, 235, 0.14);
  color: rgb(191, 219, 254);
}

.dark .image-studio-preview-canvas {
  background:
    linear-gradient(45deg, rgba(30, 64, 175, 0.2) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(30, 64, 175, 0.2) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(30, 64, 175, 0.2) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(30, 64, 175, 0.2) 75%),
    rgba(2, 6, 23, 0.74);
  background-position: 0 0, 0 12px, 12px -12px, -12px 0;
  background-size: 24px 24px;
}

.dark .image-studio-preview-details span {
  color: rgb(148, 163, 184);
}

.dark .image-studio-preview-meta-grid span {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.22), rgba(6, 182, 212, 0.16));
  color: rgb(191, 219, 254);
}

.dark .image-studio-preview-actions {
  background: rgba(2, 6, 23, 0.42);
}

.dark .image-studio-preview-actions .image-studio-preview-delete-action {
  border-color: rgba(248, 113, 113, 0.28);
  background: rgba(127, 29, 29, 0.18);
  color: rgb(252, 165, 165);
}

.dark .image-studio-preview-actions .image-studio-preview-delete-action:hover,
.dark .image-studio-preview-actions .image-studio-preview-delete-action:focus-visible {
  border-color: rgba(248, 113, 113, 0.46);
  background: rgba(127, 29, 29, 0.28);
  box-shadow: 0 0 0 3px rgba(248, 113, 113, 0.14);
}

.dark .image-studio-delete-dialog {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(2, 6, 23, 0.92)),
    rgb(15, 23, 42);
  box-shadow:
    0 30px 80px rgba(0, 0, 0, 0.48),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.dark .image-studio-delete-visual {
  background:
    linear-gradient(45deg, rgba(30, 64, 175, 0.2) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(30, 64, 175, 0.2) 25%, transparent 25%),
    rgba(2, 6, 23, 0.74);
  background-size: 22px 22px;
}

.dark .image-studio-delete-content h2,
.dark .image-studio-delete-summary strong {
  color: white;
}

.dark .image-studio-delete-content > p:not(.image-studio-section-label) {
  color: rgb(203, 213, 225);
}

.dark .image-studio-delete-summary {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(15, 23, 42, 0.62);
}

.dark .image-studio-delete-summary span {
  color: rgb(148, 163, 184);
}

.dark .image-studio-delete-cancel {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.12);
  color: rgb(191, 219, 254);
}

.dark .image-studio-delete-cancel:hover:not(:disabled),
.dark .image-studio-delete-cancel:focus-visible {
  background: rgba(30, 41, 59, 0.88);
  color: white;
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

@keyframes image-studio-preview-backdrop {
  from {
    opacity: 0;
  }
}

@keyframes image-studio-preview-dialog-in {
  from {
    opacity: 0;
    transform: translateY(12px) scale(0.98);
  }
}

@media (max-width: 1100px) {
  .image-studio-shell {
    height: auto;
    overflow: visible;
    padding-bottom: 1.25rem;
  }

  .image-studio-grid {
    grid-template-columns: 1fr;
  }

  .image-studio-workbench,
  .image-studio-control-console {
    height: auto;
    min-height: 0;
  }

  .image-studio-workspace {
    height: auto;
    grid-template-rows: auto;
    overflow: visible;
  }

  .image-studio-command-surface {
    overflow: visible;
  }

  .image-studio-prompt-section {
    flex: none;
  }

  .image-studio-canvas-stage,
  .image-studio-preview-panel {
    height: auto;
    position: static;
  }

  .image-studio-canvas-stage {
    grid-template-rows: auto auto;
  }

  .image-studio-gallery {
    max-height: none;
  }

  .image-studio-preview-frame {
    min-height: clamp(28rem, 58dvh, 42rem);
    height: auto;
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
    min-height: 24rem;
  }

  .image-studio-gallery-grid,
  .image-studio-gallery-loading {
    grid-auto-columns: minmax(10.5rem, 78vw);
  }

  .image-studio-stage-meta {
    flex-wrap: wrap;
  }

  .image-studio-preview-modal {
    align-items: end;
    padding: 0;
  }

  .image-studio-preview-dialog {
    width: 100%;
    max-height: 96vh;
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
  }

  .image-studio-preview-canvas {
    min-height: 48vh;
  }

  .image-studio-preview-canvas img {
    max-height: 48vh;
  }

  .image-studio-preview-actions {
    grid-template-columns: 1fr;
  }

  .image-studio-delete-modal {
    align-items: end;
    padding: 0;
  }

  .image-studio-delete-dialog {
    grid-template-columns: 1fr;
    width: 100%;
    border-bottom-left-radius: 0;
    border-bottom-right-radius: 0;
  }

  .image-studio-delete-visual {
    min-height: 12rem;
    max-height: 32vh;
  }

  .image-studio-delete-actions {
    grid-template-columns: 1fr;
  }

  .image-studio-submit-button {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .image-studio-loader,
  .image-studio-gallery-loading div,
  .image-studio-preview-modal,
  .image-studio-preview-dialog,
  .image-studio-delete-modal,
  .image-studio-delete-dialog {
    animation: none;
  }

  .image-studio-choice-picker button,
  .image-studio-current-actions button,
  .image-studio-image-card-actions button,
  .image-studio-preview-open img,
  .image-studio-image-thumb img,
  .image-studio-preview-open > span,
  .image-studio-image-thumb > span,
  .image-studio-preview-actions button,
  .image-studio-delete-actions button {
    transition: none;
  }
}
</style>
