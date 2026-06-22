<template>
  <div class="brand-surface profile-totp-card">
    <div class="profile-totp-header">
      <div class="brand-floating-icon profile-totp-icon">
        <Icon name="shield" size="md" />
      </div>
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-slate-950 dark:text-white">
          {{ t('profile.totp.title') }}
        </h2>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          {{ t('profile.totp.description') }}
        </p>
      </div>
    </div>
    <div class="px-6 py-6">
      <!-- Loading state -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
      </div>

      <!-- Feature disabled globally -->
      <div v-else-if="status && !status.feature_enabled" class="brand-floating-card profile-totp-state">
        <div class="flex-shrink-0 rounded-full bg-gray-100 p-3 dark:bg-dark-700">
          <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
          </svg>
        </div>
        <div>
          <p class="font-medium text-gray-700 dark:text-gray-300">
            {{ t('profile.totp.featureDisabled') }}
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.totp.featureDisabledHint') }}
          </p>
        </div>
      </div>

      <!-- 2FA Enabled -->
      <div v-else-if="status?.enabled" class="brand-floating-card profile-totp-state justify-between">
        <div class="flex items-center gap-4">
          <div class="flex-shrink-0 rounded-full bg-green-100 p-3 dark:bg-green-900/30">
            <svg class="h-6 w-6 text-green-600 dark:text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
            </svg>
          </div>
          <div>
            <p class="font-medium text-gray-900 dark:text-white">
              {{ t('profile.totp.enabled') }}
            </p>
            <p v-if="status.enabled_at" class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('profile.totp.enabledAt') }}: {{ formatDate(status.enabled_at) }}
            </p>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-outline-danger"
          @click="showDisableDialog = true"
        >
          {{ t('profile.totp.disable') }}
        </button>
      </div>

      <!-- 2FA Not Enabled -->
      <div v-else class="brand-floating-card profile-totp-state justify-between">
        <div class="flex items-center gap-4">
          <div class="flex-shrink-0 rounded-full bg-gray-100 p-3 dark:bg-dark-700">
            <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
            </svg>
          </div>
          <div>
            <p class="font-medium text-gray-700 dark:text-gray-300">
              {{ t('profile.totp.notEnabled') }}
            </p>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('profile.totp.notEnabledHint') }}
            </p>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-primary"
          @click="showSetupModal = true"
        >
          {{ t('profile.totp.enable') }}
        </button>
      </div>
    </div>

    <!-- Setup Modal -->
    <TotpSetupModal
      v-if="showSetupModal"
      @close="showSetupModal = false"
      @success="handleSetupSuccess"
    />

    <!-- Disable Dialog -->
    <TotpDisableDialog
      v-if="showDisableDialog"
      @close="showDisableDialog = false"
      @success="handleDisableSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { totpAPI } from '@/api'
import type { TotpStatus } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import TotpSetupModal from './TotpSetupModal.vue'
import TotpDisableDialog from './TotpDisableDialog.vue'

const { t } = useI18n()

const loading = ref(true)
const status = ref<TotpStatus | null>(null)
const showSetupModal = ref(false)
const showDisableDialog = ref(false)

const loadStatus = async () => {
  loading.value = true
  try {
    status.value = await totpAPI.getStatus()
  } catch (error) {
    console.error('Failed to load TOTP status:', error)
  } finally {
    loading.value = false
  }
}

const handleSetupSuccess = () => {
  showSetupModal.value = false
  loadStatus()
}

const handleDisableSuccess = () => {
  showDisableDialog.value = false
  loadStatus()
}

const formatDate = (timestamp: number) => {
  // Backend returns Unix timestamp in seconds, convert to milliseconds
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

onMounted(() => {
  loadStatus()
})
</script>

<style scoped>
.profile-totp-card {
  overflow: hidden;
  border-radius: 1.25rem;
}

.profile-totp-header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  border-bottom: 1px solid rgba(191, 219, 254, 0.5);
  padding: 1.25rem 1.25rem 1rem;
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.66), rgba(236, 254, 255, 0.42));
}

.profile-totp-icon {
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
}

.profile-totp-state {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.dark .profile-totp-header {
  border-color: rgba(96, 165, 250, 0.16);
  background: linear-gradient(135deg, rgba(30, 64, 175, 0.16), rgba(8, 145, 178, 0.08));
}

@media (max-width: 640px) {
  .profile-totp-card {
    border-radius: 1rem;
  }

  .profile-totp-header {
    padding: 1rem;
  }

  .profile-totp-state {
    align-items: flex-start;
    flex-direction: column;
  }

  .profile-totp-state > .flex {
    width: 100%;
  }
}
</style>
