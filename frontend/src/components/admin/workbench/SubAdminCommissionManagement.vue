<template>
  <div data-test="sub-admin-commission-management" class="flex min-w-0 flex-wrap items-center gap-2">
    <button
      type="button"
      data-test="commission-settings-open"
      class="btn btn-secondary min-h-11 justify-center px-3"
      aria-haspopup="dialog"
      :aria-expanded="showSettings"
      @click="openSettings"
    >
      <Icon name="cog" size="sm" />
      <span>{{ t('adminWorkbench.commission.settings') }}</span>
    </button>
    <button
      type="button"
      data-test="commission-exclusions-open"
      class="btn btn-secondary min-h-11 justify-center px-3"
      aria-haspopup="dialog"
      :aria-expanded="showExclusions"
      @click="showExclusions = true"
    >
      {{ t('adminWorkbench.commission.exclusions') }}
    </button>

    <BaseDialog
      v-if="showSettings"
      :show="true"
      :title="t('adminWorkbench.commission.settings')"
      appearance="neutral"
      width="wide"
      :close-on-escape="!savingSettings && !savingGrants"
      :show-close-button="!savingSettings && !savingGrants"
      @close="closeSettings"
    >
      <div ref="settingsContent" class="min-w-0 space-y-5" tabindex="-1">
        <p v-if="loading" class="text-sm text-gray-500 dark:text-dark-400" role="status">
          {{ t('common.loading') }}
        </p>
        <div v-else-if="loadError" class="space-y-2">
          <p class="text-sm text-red-600 dark:text-red-400" role="alert">{{ loadError }}</p>
          <button type="button" class="btn btn-secondary min-h-11" @click="loadManagementData">
            {{ t('common.refresh') }}
          </button>
        </div>

        <section class="min-w-0">
          <div data-test="commission-rate-controls" class="workbench-commission-controls">
            <label class="block min-w-0">
              <span class="input-label">{{ t('adminWorkbench.commission.commissionRate') }}</span>
              <input
                v-model.number="commissionRate"
                data-test="sub-admin-commission-rate"
                type="number"
                min="0"
                max="1"
                step="0.01"
                class="input"
                :disabled="!managementLoaded || loading || savingSettings"
              />
            </label>
            <button
              type="button"
              data-test="sub-admin-commission-save-settings"
              class="btn btn-primary min-h-11 justify-center"
              :disabled="!managementLoaded || loading || savingSettings"
              @click="saveSettings"
            >
              <Icon v-if="savingSettings" name="refresh" size="sm" class="animate-spin" />
              <span>{{ t('adminWorkbench.commission.saveSettings') }}</span>
            </button>
          </div>
        </section>

        <section class="workbench-commission-grants min-w-0 space-y-3 border-t pt-4">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h3 class="min-w-0 text-sm font-semibold text-gray-950 dark:text-white">
              {{ t('adminWorkbench.commission.assignedGroups') }}
            </h3>
            <button
              type="button"
              data-test="sub-admin-commission-save-grants"
              class="btn btn-secondary min-h-11 justify-center"
              :disabled="!managementLoaded || loading || savingGrants"
              @click="saveGrants"
            >
              <Icon v-if="savingGrants" name="refresh" size="sm" class="animate-spin" />
              <span>{{ t('adminWorkbench.commission.saveGrants') }}</span>
            </button>
          </div>
          <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
            {{ t('adminWorkbench.commission.sharedGrantsHint') }}
          </p>
          <div v-if="managementLoaded && groups.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.noGroupsAvailable') }}
          </div>
          <div v-else-if="managementLoaded" class="grid max-h-72 gap-2 overflow-y-auto pr-1 sm:grid-cols-2">
            <label
              v-for="group in groups"
              :key="group.id"
              class="workbench-commission-group flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-sm"
            >
              <input
                v-model="assignedGroupIDs"
                type="checkbox"
                class="sub-admin-group-checkbox rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :value="group.id"
                :disabled="savingGrants"
                :data-test="`sub-admin-commission-group-${group.id}`"
              />
              <span class="min-w-0 break-words text-gray-900 dark:text-white">{{ group.name }}</span>
            </label>
          </div>
        </section>
      </div>
      <template #footer>
        <button
          type="button"
          class="btn btn-secondary min-h-11"
          :disabled="savingSettings || savingGrants"
          @click="closeSettings"
        >
          {{ t('common.close') }}
        </button>
      </template>
    </BaseDialog>
    <SubAdminCommissionExclusionsDialog
      v-if="showExclusions"
      @close="showExclusions = false"
      @changed="emit('changed')"
    />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import type { SubAdminCommissionGrant } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import SubAdminCommissionExclusionsDialog from './SubAdminCommissionExclusionsDialog.vue'

const emit = defineEmits<{ (event: 'changed'): void }>()
const showExclusions = ref(false)
const showSettings = ref(false)
const settingsContent = ref<HTMLElement | null>(null)
const { t } = useI18n()
const appStore = useAppStore()

const commissionRate = ref(0)
const groups = ref<AdminGroup[]>([])
const grants = ref<SubAdminCommissionGrant[]>([])
const assignedGroupIDs = ref<number[]>([])
const loading = ref(false)
const managementLoaded = ref(false)
const loadError = ref('')
const savingSettings = ref(false)
const savingGrants = ref(false)

function syncSelectedGrants() {
  assignedGroupIDs.value = grants.value
    .filter((grant) => grant.enabled)
    .map((grant) => grant.group_id)
}

function openSettings() {
  showSettings.value = true
  if (!managementLoaded.value) void loadManagementData()
}

function closeSettings() {
  if (!savingSettings.value && !savingGrants.value) showSettings.value = false
}

function keepSettingsFocus(event: KeyboardEvent) {
  if (!showSettings.value || event.key !== 'Tab' || event.defaultPrevented) return
  const dialog = settingsContent.value?.closest<HTMLElement>('[role="dialog"]')
  if (!dialog) return
  const active = document.activeElement
  const activeDialog = active?.closest('[role="dialog"]')
  if (activeDialog && activeDialog !== dialog) return
  const focusable = Array.from(dialog.querySelectorAll<HTMLElement>(
    'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  )).filter((element) => element.tabIndex >= 0 && !element.closest('[hidden], [inert], [aria-hidden="true"]'))
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (!first) {
    event.preventDefault()
    settingsContent.value?.focus()
  } else if (!dialog.contains(active) || (event.shiftKey && active === first) || (!event.shiftKey && active === last)) {
    event.preventDefault()
    const target = event.shiftKey ? last : first
    target.focus()
  }
}

watch(showSettings, (open) => {
  if (open) document.addEventListener('keydown', keepSettingsFocus)
  else document.removeEventListener('keydown', keepSettingsFocus)
})
onBeforeUnmount(() => document.removeEventListener('keydown', keepSettingsFocus))

async function loadManagementData() {
  if (loading.value || managementLoaded.value) return
  loading.value = true
  loadError.value = ''
  try {
    const [settings, groupList, grantList] = await Promise.all([
      adminAPI.subAdminCommission.getSettings(),
      adminAPI.groups.getAll(),
      adminAPI.subAdminCommission.listGrants()
    ])
    commissionRate.value = settings.commission_rate
    groups.value = groupList
    grants.value = grantList
    syncSelectedGrants()
    managementLoaded.value = true
  } catch (error: any) {
    loadError.value = extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed'))
    appStore.showError(loadError.value)
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  if (!managementLoaded.value || loading.value || savingSettings.value) return
  savingSettings.value = true
  try {
    const saved = await adminAPI.subAdminCommission.updateSettings({
      commission_rate: commissionRate.value
    })
    commissionRate.value = saved.commission_rate
    appStore.showSuccess(t('adminWorkbench.commission.saveSuccess'))
    emit('changed')
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.commission.saveFailed')))
  } finally {
    savingSettings.value = false
  }
}

async function saveGrants() {
  if (!managementLoaded.value || loading.value || savingGrants.value) return
  savingGrants.value = true
  try {
    const updated = await adminAPI.subAdminCommission.replaceGrants({
      group_ids: [...assignedGroupIDs.value]
    })
    grants.value = updated
    syncSelectedGrants()
    appStore.showSuccess(t('adminWorkbench.commission.saveSuccess'))
    emit('changed')
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.commission.saveFailed')))
  } finally {
    savingGrants.value = false
  }
}
</script>

<style scoped>
.workbench-commission-controls {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: 0.5rem;
}
.workbench-commission-group:hover { background: var(--workspace-hover); }
.workbench-commission-group:has(:checked) { background: var(--workspace-hover); }

.workbench-commission-grants { border-color: var(--workspace-divider); }

@media (max-width: 359px) {
  .workbench-commission-controls { grid-template-columns: minmax(0, 1fr); }
}

.sub-admin-group-checkbox {
  width: 16px !important;
  height: 16px !important;
  min-width: 16px;
  min-height: 16px;
  flex: 0 0 16px;
  margin: 0;
  accent-color: rgb(37 99 235);
}
</style>
