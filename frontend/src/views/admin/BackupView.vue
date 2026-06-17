<template>
    <div class="backup-admin-page space-y-6">
      <section class="admin-toolbar-surface backup-overview-panel" data-test="backup-overview-surface">
        <div class="admin-toolbar">
          <div class="flex min-w-0 flex-1 items-start gap-3">
            <div class="backup-icon-shell bg-blue-100 text-blue-600 dark:bg-blue-500/15 dark:text-blue-300">
              <Icon name="database" size="md" />
            </div>
            <div class="min-w-0">
              <p class="text-xs font-semibold uppercase text-blue-600 dark:text-blue-300">{{ t('admin.settings.tabs.backup') }}</p>
              <h3 class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">{{ t('admin.backup.title') }}</h3>
              <p class="mt-1 max-w-3xl text-sm leading-6 text-slate-500 dark:text-slate-400">{{ t('admin.backup.description') }}</p>
            </div>
          </div>
          <div class="admin-toolbar-group justify-end">
            <span class="admin-page-meta-chip">
              <span>S3</span>
              <strong>{{ s3SecretConfigured ? t('common.enabled') : t('common.disabled') }}</strong>
            </span>
            <span class="admin-page-meta-chip">
              <span>{{ t('admin.backup.schedule.title') }}</span>
              <strong>{{ scheduleForm.enabled ? t('common.enabled') : t('common.disabled') }}</strong>
            </span>
            <span class="admin-page-meta-chip">
              <span>{{ t('common.total') }}</span>
              <strong>{{ backups.length }}</strong>
            </span>
          </div>
        </div>
      </section>

      <!-- S3 Storage Config -->
      <section data-test="backup-s3-surface" class="admin-surface overflow-hidden">
        <div class="admin-panel-header">
          <div class="flex min-w-0 items-start gap-3">
            <div class="backup-icon-shell bg-cyan-100 text-cyan-600 dark:bg-cyan-500/15 dark:text-cyan-300">
              <Icon name="cloud" size="sm" />
            </div>
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.backup.s3.title') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.backup.s3.descriptionPrefix') }}
                <button type="button" class="font-medium text-primary-600 underline decoration-primary-300 underline-offset-4 transition-colors hover:text-primary-700 dark:text-primary-300 dark:decoration-primary-500/50 dark:hover:text-primary-200" @click="showR2Guide = true">Cloudflare R2</button>
                {{ t('admin.backup.s3.descriptionSuffix') }}
              </p>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 p-5 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.backup.s3.endpoint') }}</label>
            <input v-model="s3Form.endpoint" class="input w-full" placeholder="https://<account_id>.r2.cloudflarestorage.com" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.s3.region') }}</label>
            <input v-model="s3Form.region" class="input w-full" placeholder="auto" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.s3.bucket') }}</label>
            <input v-model="s3Form.bucket" class="input w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.s3.prefix') }}</label>
            <input v-model="s3Form.prefix" class="input w-full" placeholder="backups/" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.s3.accessKeyId') }}</label>
            <input v-model="s3Form.access_key_id" class="input w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.s3.secretAccessKey') }}</label>
            <input v-model="s3Form.secret_access_key" type="password" class="input w-full" :placeholder="s3SecretConfigured ? t('admin.backup.s3.secretConfigured') : ''" />
          </div>
          <label class="backup-check-row md:col-span-2">
            <input v-model="s3Form.force_path_style" type="checkbox" />
            <span>{{ t('admin.backup.s3.forcePathStyle') }}</span>
          </label>
          <div class="admin-toolbar-surface md:col-span-2">
            <div class="admin-toolbar">
              <div class="admin-toolbar-group justify-end">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="testingS3" @click="testS3">
                  {{ testingS3 ? t('common.loading') : t('admin.backup.s3.testConnection') }}
                </button>
                <button type="button" class="btn btn-primary btn-sm" :disabled="savingS3" @click="saveS3Config">
                  {{ savingS3 ? t('common.loading') : t('common.save') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Schedule Config -->
      <section data-test="backup-schedule-surface" class="admin-surface overflow-hidden">
        <div class="admin-panel-header">
          <div class="flex min-w-0 items-start gap-3">
            <div class="backup-icon-shell bg-blue-100 text-blue-600 dark:bg-blue-500/15 dark:text-blue-300">
              <Icon name="clock" size="sm" />
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.backup.schedule.title') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.backup.schedule.description') }}
              </p>
            </div>
          </div>
          <span class="admin-page-meta-chip w-fit text-xs">
            {{ scheduleForm.enabled ? t('common.enabled') : t('common.disabled') }}
          </span>
        </div>
        <div class="grid grid-cols-1 gap-4 p-5 md:grid-cols-2">
          <label class="backup-check-row md:col-span-2">
            <input v-model="scheduleForm.enabled" type="checkbox" />
            <span>{{ t('admin.backup.schedule.enabled') }}</span>
          </label>
          <div>
            <label class="input-label">{{ t('admin.backup.schedule.cronExpr') }}</label>
            <input v-model="scheduleForm.cron_expr" class="input w-full" placeholder="0 2 * * *" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.backup.schedule.cronHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.schedule.retainDays') }}</label>
            <input v-model.number="scheduleForm.retain_days" type="number" min="0" class="input w-full" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.backup.schedule.retainDaysHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.backup.schedule.retainCount') }}</label>
            <input v-model.number="scheduleForm.retain_count" type="number" min="0" class="input w-full" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.backup.schedule.retainCountHint') }}</p>
          </div>
          <div class="admin-toolbar-surface md:col-span-2">
            <div class="admin-toolbar">
              <div class="admin-toolbar-group justify-end">
                <button type="button" class="btn btn-primary btn-sm" :disabled="savingSchedule" @click="saveSchedule">
                  {{ savingSchedule ? t('common.loading') : t('common.save') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Backup Operations -->
      <section data-test="backup-operations-surface" class="admin-surface overflow-hidden">
        <div class="admin-panel-header">
          <div class="flex min-w-0 items-start gap-3">
            <div class="backup-icon-shell bg-emerald-100 text-emerald-600 dark:bg-emerald-500/15 dark:text-emerald-300">
              <Icon name="server" size="sm" />
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.backup.operations.title') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.backup.operations.description') }}
              </p>
            </div>
          </div>
        </div>
        <div class="border-b border-blue-100/50 p-4 dark:border-blue-500/10">
          <div class="admin-toolbar-surface" data-test="backup-operations-toolbar">
            <div class="admin-toolbar">
              <div class="admin-toolbar-group flex-1">
                <label class="text-xs font-medium text-gray-600 dark:text-gray-400">{{ t('admin.backup.operations.expireDays') }}</label>
                <input v-model.number="manualExpireDays" type="number" min="0" class="input w-24 text-xs" />
              </div>
              <div class="admin-toolbar-group justify-end">
                <button type="button" class="btn btn-primary btn-sm" :disabled="creatingBackup" @click="createBackup">
                  {{ creatingBackup ? t('admin.backup.operations.backing') : t('admin.backup.operations.createBackup') }}
                </button>
                <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingBackups" @click="loadBackups">
                  <Icon name="refresh" size="sm" :class="loadingBackups ? 'animate-spin' : ''" />
                  {{ loadingBackups ? t('common.loading') : t('common.refresh') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="overflow-x-auto">
          <table class="backup-records-table w-full min-w-[800px] text-sm">
            <thead>
              <tr class="text-left text-xs uppercase text-gray-500 dark:text-gray-400">
                <th class="py-2 pr-4">ID</th>
                <th class="py-2 pr-4">{{ t('admin.backup.columns.status') }}</th>
                <th class="py-2 pr-4">{{ t('admin.backup.columns.fileName') }}</th>
                <th class="py-2 pr-4">{{ t('admin.backup.columns.size') }}</th>
                <th class="py-2 pr-4">{{ t('admin.backup.columns.expiresAt') }}</th>
                <th class="py-2 pr-4">{{ t('admin.backup.columns.triggeredBy') }}</th>
                <th class="py-2 pr-4">{{ t('admin.backup.columns.startedAt') }}</th>
                <th class="py-2">{{ t('admin.backup.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-blue-100/50 dark:divide-blue-500/10">
              <tr v-for="record in backups" :key="record.id" class="align-top transition-colors hover:bg-blue-50/35 dark:hover:bg-blue-500/5">
                <td class="py-3 pr-4 font-mono text-xs">{{ record.id }}</td>
                <td class="py-3 pr-4">
                  <span
                    class="rounded px-2 py-0.5 text-xs"
                    :class="statusClass(record.status)"
                  >
                    {{ record.status === 'running' && record.progress
                      ? t(`admin.backup.progress.${record.progress}`)
                      : t(`admin.backup.status.${record.status}`) }}
                  </span>
                </td>
                <td class="py-3 pr-4 text-xs">{{ record.file_name }}</td>
                <td class="py-3 pr-4 text-xs">{{ formatSize(record.size_bytes) }}</td>
                <td class="py-3 pr-4 text-xs">
                  {{ record.expires_at ? formatDate(record.expires_at) : t('admin.backup.neverExpire') }}
                </td>
                <td class="py-3 pr-4 text-xs">
                  {{ record.triggered_by === 'scheduled' ? t('admin.backup.trigger.scheduled') : t('admin.backup.trigger.manual') }}
                </td>
                <td class="py-3 pr-4 text-xs">{{ formatDate(record.started_at) }}</td>
                <td class="py-3 text-xs">
                  <div class="flex flex-wrap gap-1">
                    <button
                      v-if="record.status === 'completed'"
                      type="button"
                      class="btn btn-secondary btn-xs"
                      @click="downloadBackup(record.id)"
                    >
                      {{ t('admin.backup.actions.download') }}
                    </button>
                    <button
                      v-if="record.status === 'completed'"
                      type="button"
                      class="btn btn-secondary btn-xs"
                      :disabled="restoringId === record.id"
                      @click="restoreBackup(record.id)"
                    >
                      {{ restoringId === record.id ? t('common.loading') : t('admin.backup.actions.restore') }}
                    </button>
                    <button
                      type="button"
                      class="btn btn-danger btn-xs"
                      @click="removeBackup(record.id)"
                    >
                      {{ t('common.delete') }}
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="backups.length === 0">
                <td colspan="8" class="px-4 py-8">
                  <div class="admin-empty-state">
                    {{ t('admin.backup.empty') }}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <!-- Cloudflare R2 Setup Guide Modal -->
    <teleport to="body">
      <transition name="modal">
        <div v-if="showR2Guide" class="brand-overlay z-50 flex items-start justify-center overflow-y-auto p-4 pt-[7vh]" @mousedown.self="showR2Guide = false">
          <div class="brand-floating-panel w-full max-w-2xl">
            <div class="brand-floating-header flex items-start justify-between gap-4">
              <div class="flex min-w-0 items-start gap-3">
                <div class="brand-floating-icon h-11 w-11 rounded-2xl">
                  <Icon name="cloud" size="md" />
                </div>
                <div class="min-w-0">
                  <span class="brand-floating-chip">Cloudflare R2</span>
                  <h2 class="mt-3 text-lg font-semibold text-slate-950 dark:text-white">{{ t('admin.backup.r2Guide.title') }}</h2>
                  <p class="mt-1 text-sm leading-6 text-slate-500 dark:text-slate-400">{{ t('admin.backup.r2Guide.intro') }}</p>
                </div>
              </div>
              <button type="button" class="brand-floating-close flex-shrink-0" @click="showR2Guide = false" :aria-label="t('common.close')">
                <Icon name="x" size="md" :stroke-width="2" />
              </button>
            </div>

            <div class="max-h-[calc(85vh-9rem)] overflow-y-auto px-6 py-5">
              <!-- Step 1 -->
              <div class="mb-5">
                <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                  <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">1</span>
                  {{ t('admin.backup.r2Guide.step1.title') }}
                </h3>
                <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                  <li>{{ t('admin.backup.r2Guide.step1.line1') }}</li>
                  <li>{{ t('admin.backup.r2Guide.step1.line2') }}</li>
                  <li>{{ t('admin.backup.r2Guide.step1.line3') }}</li>
                </ol>
              </div>

              <!-- Step 2 -->
              <div class="mb-5">
                <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                  <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">2</span>
                  {{ t('admin.backup.r2Guide.step2.title') }}
                </h3>
                <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                  <li>{{ t('admin.backup.r2Guide.step2.line1') }}</li>
                  <li>{{ t('admin.backup.r2Guide.step2.line2') }}</li>
                  <li>{{ t('admin.backup.r2Guide.step2.line3') }}</li>
                  <li>{{ t('admin.backup.r2Guide.step2.line4') }}</li>
                </ol>
                <div class="mt-2 rounded-lg bg-amber-50 p-3 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                  {{ t('admin.backup.r2Guide.step2.warning') }}
                </div>
              </div>

              <!-- Step 3 -->
              <div class="mb-5">
                <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                  <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">3</span>
                  {{ t('admin.backup.r2Guide.step3.title') }}
                </h3>
                <p class="ml-8 text-sm text-gray-600 dark:text-gray-300">{{ t('admin.backup.r2Guide.step3.desc') }}</p>
                <code class="admin-form-section ml-8 mt-2 block !space-y-0 !rounded-xl !p-3 text-xs text-gray-800 dark:text-gray-200">https://&lt;{{ t('admin.backup.r2Guide.step3.accountId') }}&gt;.r2.cloudflarestorage.com</code>
              </div>

              <!-- Step 4: Fill form -->
              <div class="mb-5">
                <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                  <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">4</span>
                  {{ t('admin.backup.r2Guide.step4.title') }}
                </h3>
                <div class="admin-list-surface ml-8 !rounded-xl">
                  <table class="w-full text-sm">
                    <tbody>
                      <tr v-for="(row, i) in r2ConfigRows" :key="i" class="border-b border-gray-100 dark:border-dark-700 last:border-0">
                        <td class="whitespace-nowrap bg-primary-50/70 px-3 py-2 font-medium text-primary-700 dark:bg-primary-500/10 dark:text-primary-200">{{ row.field }}</td>
                        <td class="px-3 py-2 text-gray-600 dark:text-gray-400"><code class="text-xs">{{ row.value }}</code></td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <!-- Free tier note -->
              <div class="rounded-lg bg-green-50 p-3 text-xs text-green-700 dark:bg-green-900/20 dark:text-green-300">
                {{ t('admin.backup.r2Guide.freeTier') }}
              </div>

              <div class="mt-5 flex justify-end border-t border-blue-100/70 pt-4 dark:border-blue-500/10">
                <button type="button" class="btn btn-primary btn-sm" @click="showR2Guide = false">{{ t('common.close') }}</button>
              </div>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import type { BackupS3Config, BackupScheduleConfig, BackupRecord } from '@/api/admin/backup'

const { t } = useI18n()
const appStore = useAppStore()

// S3 config
const s3Form = ref<BackupS3Config>({
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  prefix: 'backups/',
  force_path_style: false,
})
const s3SecretConfigured = ref(false)
const savingS3 = ref(false)
const testingS3 = ref(false)

// Schedule config
const scheduleForm = ref<BackupScheduleConfig>({
  enabled: false,
  cron_expr: '0 2 * * *',
  retain_days: 14,
  retain_count: 10,
})
const savingSchedule = ref(false)

// Backups
const backups = ref<BackupRecord[]>([])
const loadingBackups = ref(false)
const creatingBackup = ref(false)
const restoringId = ref('')
const manualExpireDays = ref(14)

// Polling
const pollingTimer = ref<ReturnType<typeof setInterval> | null>(null)
const restoringPollingTimer = ref<ReturnType<typeof setInterval> | null>(null)
const MAX_POLL_COUNT = 900

function updateRecordInList(updated: BackupRecord) {
  const idx = backups.value.findIndex(r => r.id === updated.id)
  if (idx >= 0) {
    backups.value[idx] = updated
  }
}

function startPolling(backupId: string) {
  stopPolling()
  let count = 0
  pollingTimer.value = setInterval(async () => {
    if (count++ >= MAX_POLL_COUNT) {
      stopPolling()
      creatingBackup.value = false
      appStore.showWarning(t('admin.backup.operations.backupRunning'))
      return
    }
    try {
      const record = await adminAPI.backup.getBackup(backupId)
      updateRecordInList(record)
      if (record.status === 'completed' || record.status === 'failed') {
        stopPolling()
        creatingBackup.value = false
        if (record.status === 'completed') {
          appStore.showSuccess(t('admin.backup.operations.backupCreated'))
        } else {
          appStore.showError(record.error_message || t('admin.backup.operations.backupFailed'))
        }
        await loadBackups()
      }
    } catch {
      // 轮询失败时不中断
    }
  }, 2000)
}

function stopPolling() {
  if (pollingTimer.value) {
    clearInterval(pollingTimer.value)
    pollingTimer.value = null
  }
}

function startRestorePolling(backupId: string) {
  stopRestorePolling()
  let count = 0
  restoringPollingTimer.value = setInterval(async () => {
    if (count++ >= MAX_POLL_COUNT) {
      stopRestorePolling()
      restoringId.value = ''
      appStore.showWarning(t('admin.backup.operations.restoreRunning'))
      return
    }
    try {
      const record = await adminAPI.backup.getBackup(backupId)
      updateRecordInList(record)
      if (record.restore_status === 'completed' || record.restore_status === 'failed') {
        stopRestorePolling()
        restoringId.value = ''
        if (record.restore_status === 'completed') {
          appStore.showSuccess(t('admin.backup.actions.restoreSuccess'))
        } else {
          appStore.showError(record.restore_error || t('admin.backup.operations.restoreFailed'))
        }
        await loadBackups()
      }
    } catch {
      // 轮询失败时不中断
    }
  }, 2000)
}

function stopRestorePolling() {
  if (restoringPollingTimer.value) {
    clearInterval(restoringPollingTimer.value)
    restoringPollingTimer.value = null
  }
}

function handleVisibilityChange() {
  if (document.hidden) {
    stopPolling()
    stopRestorePolling()
  } else {
    // 标签页恢复时刷新列表，检查是否仍有活跃操作
    loadBackups().then(() => {
      const running = backups.value.find(r => r.status === 'running')
      if (running) {
        creatingBackup.value = true
        startPolling(running.id)
      }
      const restoring = backups.value.find(r => r.restore_status === 'running')
      if (restoring) {
        restoringId.value = restoring.id
        startRestorePolling(restoring.id)
      }
    })
  }
}

// R2 guide
const showR2Guide = ref(false)
const r2ConfigRows = computed(() => [
  { field: t('admin.backup.s3.endpoint'), value: 'https://<account_id>.r2.cloudflarestorage.com' },
  { field: t('admin.backup.s3.region'), value: 'auto' },
  { field: t('admin.backup.s3.bucket'), value: t('admin.backup.r2Guide.step4.bucketValue') },
  { field: t('admin.backup.s3.prefix'), value: 'backups/' },
  { field: 'Access Key ID', value: t('admin.backup.r2Guide.step4.fromStep2') },
  { field: 'Secret Access Key', value: t('admin.backup.r2Guide.step4.fromStep2') },
  { field: t('admin.backup.s3.forcePathStyle'), value: t('admin.backup.r2Guide.step4.unchecked') },
])

async function loadS3Config() {
  try {
    const cfg = await adminAPI.backup.getS3Config()
    s3Form.value = {
      endpoint: cfg.endpoint || '',
      region: cfg.region || 'auto',
      bucket: cfg.bucket || '',
      access_key_id: cfg.access_key_id || '',
      secret_access_key: '',
      prefix: cfg.prefix || 'backups/',
      force_path_style: cfg.force_path_style,
    }
    s3SecretConfigured.value = Boolean(cfg.access_key_id)
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveS3Config() {
  savingS3.value = true
  try {
    await adminAPI.backup.updateS3Config(s3Form.value)
    appStore.showSuccess(t('admin.backup.s3.saved'))
    await loadS3Config()
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingS3.value = false
  }
}

async function testS3() {
  testingS3.value = true
  try {
    const result = await adminAPI.backup.testS3Connection(s3Form.value)
    if (result.ok) {
      appStore.showSuccess(result.message || t('admin.backup.s3.testSuccess'))
    } else {
      appStore.showError(result.message || t('admin.backup.s3.testFailed'))
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    testingS3.value = false
  }
}

async function loadSchedule() {
  try {
    const cfg = await adminAPI.backup.getSchedule()
    scheduleForm.value = {
      enabled: cfg.enabled,
      cron_expr: cfg.cron_expr || '0 2 * * *',
      retain_days: cfg.retain_days || 14,
      retain_count: cfg.retain_count || 10,
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveSchedule() {
  savingSchedule.value = true
  try {
    await adminAPI.backup.updateSchedule(scheduleForm.value)
    appStore.showSuccess(t('admin.backup.schedule.saved'))
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingSchedule.value = false
  }
}

async function loadBackups() {
  loadingBackups.value = true
  try {
    const result = await adminAPI.backup.listBackups()
    backups.value = result.items || []
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    loadingBackups.value = false
  }
}

async function createBackup() {
  creatingBackup.value = true
  try {
    const record = await adminAPI.backup.createBackup({ expire_days: manualExpireDays.value })
    // 插入到列表顶部
    backups.value.unshift(record)
    startPolling(record.id)
  } catch (error: any) {
    if (error?.response?.status === 409) {
      appStore.showWarning(t('admin.backup.operations.alreadyInProgress'))
    } else {
      appStore.showError(error?.message || t('errors.networkError'))
    }
    creatingBackup.value = false
  }
}

async function downloadBackup(id: string) {
  try {
    const result = await adminAPI.backup.getDownloadURL(id)
    window.open(result.url, '_blank')
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function restoreBackup(id: string) {
  if (!window.confirm(t('admin.backup.actions.restoreConfirm'))) return
  const password = window.prompt(t('admin.backup.actions.restorePasswordPrompt'))
  if (!password) return
  restoringId.value = id
  try {
    const record = await adminAPI.backup.restoreBackup(id, password)
    updateRecordInList(record)
    startRestorePolling(id)
  } catch (error: any) {
    if (error?.response?.status === 409) {
      appStore.showWarning(t('admin.backup.operations.restoreRunning'))
    } else {
      appStore.showError(error?.message || t('errors.networkError'))
    }
    restoringId.value = ''
  }
}

async function removeBackup(id: string) {
  if (!window.confirm(t('admin.backup.actions.deleteConfirm'))) return
  try {
    await adminAPI.backup.deleteBackup(id)
    appStore.showSuccess(t('admin.backup.actions.deleted'))
    await loadBackups()
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

function statusClass(status: string): string {
  switch (status) {
    case 'completed':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    case 'running':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
    case 'failed':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    default:
      return 'bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-gray-300'
  }
}

function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

onMounted(async () => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  await Promise.all([loadS3Config(), loadSchedule(), loadBackups()])

  // 如果有正在 running 的备份，恢复轮询
  const runningBackup = backups.value.find(r => r.status === 'running')
  if (runningBackup) {
    creatingBackup.value = true
    startPolling(runningBackup.id)
  }
  const restoringBackup = backups.value.find(r => r.restore_status === 'running')
  if (restoringBackup) {
    restoringId.value = restoringBackup.id
    startRestorePolling(restoringBackup.id)
  }
})

onBeforeUnmount(() => {
  stopPolling()
  stopRestorePolling()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<style scoped>
.backup-overview-panel {
  position: relative;
  overflow: hidden;
}

.backup-overview-panel::after {
  position: absolute;
  right: -4rem;
  bottom: -5rem;
  width: 14rem;
  height: 14rem;
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(34, 211, 238, 0.18), transparent 62%);
  content: "";
  pointer-events: none;
}

.backup-icon-shell {
  @apply flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ring-1 ring-white/70 dark:ring-white/10;
}

.backup-check-row {
  @apply inline-flex items-center gap-2 rounded-xl border border-blue-100/70 bg-blue-50/40 px-3 py-2 text-sm font-medium text-slate-700 dark:border-blue-500/10 dark:bg-blue-500/10 dark:text-slate-200;
}

.backup-check-row input {
  @apply h-4 w-4 rounded border-slate-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500 dark:bg-dark-800;
}

.backup-records-table thead {
  @apply bg-slate-50/80 dark:bg-dark-800/60;
}

.backup-records-table th {
  @apply border-b border-blue-100/60 px-4 py-3 text-xs font-semibold dark:border-blue-500/10;
}

.backup-records-table td {
  @apply px-4 text-slate-700 dark:text-slate-300;
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
