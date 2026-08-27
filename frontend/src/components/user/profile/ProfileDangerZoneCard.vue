<template>
  <section class="brand-surface profile-danger-card">
    <div class="profile-danger-header">
      <div class="profile-danger-icon">
        <Icon name="exclamationTriangle" size="md" />
      </div>
      <div class="min-w-0 flex-1">
        <h2 class="text-base font-semibold text-red-950 dark:text-red-100">
          {{ t('profile.accountDeletion.title') }}
        </h2>
        <p class="mt-1 text-sm text-red-700/80 dark:text-red-200/75">
          {{ t('profile.accountDeletion.description') }}
        </p>
      </div>
    </div>

    <div class="px-5 py-5 sm:px-6 sm:py-6">
      <div v-if="!confirming" class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <p class="text-sm text-slate-600 dark:text-slate-300">
          {{ t('profile.accountDeletion.summary') }}
        </p>
        <button
          type="button"
          class="btn btn-danger shrink-0"
          data-testid="account-delete-open"
          @click="openConfirm"
        >
          {{ t('profile.accountDeletion.open') }}
        </button>
      </div>

      <form
        v-else
        data-testid="account-delete-form"
        class="profile-danger-confirm space-y-4"
        @submit.prevent="submitDeletion"
      >
        <p class="text-sm text-red-800 dark:text-red-100">
          {{ t('profile.accountDeletion.confirmHint') }}
        </p>

        <div>
          <label for="account-delete-password" class="input-label">
            {{ t('profile.currentPassword') }}
          </label>
          <input
            id="account-delete-password"
            v-model="password"
            data-testid="account-delete-password"
            type="password"
            autocomplete="current-password"
            class="input"
            :placeholder="t('profile.accountDeletion.passwordPlaceholder')"
          />
        </div>

        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            @click="cancelConfirm"
          >
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn btn-danger" :disabled="loading">
            {{ loading ? t('common.processing') : t('profile.accountDeletion.confirm') }}
          </button>
        </div>
      </form>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { userAPI } from '@/api'
import { Icon } from '@/components/icons'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const confirming = ref(false)
const loading = ref(false)
const password = ref('')

function openConfirm(): void {
  password.value = ''
  confirming.value = true
}

function cancelConfirm(): void {
  if (loading.value) return
  password.value = ''
  confirming.value = false
}

async function submitDeletion(): Promise<void> {
  if (password.value.trim().length === 0) {
    appStore.showError(t('profile.accountDeletion.passwordRequired'))
    return
  }

  loading.value = true
  try {
    await userAPI.deleteOwnAccount(password.value)
    appStore.showSuccess(t('profile.accountDeletion.success'))
    await authStore.logout()
    await router.push('/login')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('profile.accountDeletion.failed')))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.profile-danger-card {
  overflow: hidden;
  border-radius: 1.25rem;
  border-color: rgba(248, 113, 113, 0.35);
}

.profile-danger-header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  border-bottom: 1px solid rgba(248, 113, 113, 0.28);
  padding: 1.25rem 1.25rem 1rem;
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.92), rgba(255, 247, 237, 0.56));
}

.profile-danger-icon {
  display: inline-flex;
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: rgba(254, 226, 226, 0.95);
  color: rgb(220, 38, 38);
}

.profile-danger-confirm {
  border-radius: 0.875rem;
  border: 1px solid rgba(248, 113, 113, 0.32);
  background: rgba(254, 242, 242, 0.55);
  padding: 1rem;
}

.dark .profile-danger-card {
  border-color: rgba(248, 113, 113, 0.2);
}

.dark .profile-danger-header {
  border-color: rgba(248, 113, 113, 0.2);
  background: linear-gradient(135deg, rgba(127, 29, 29, 0.34), rgba(69, 10, 10, 0.18));
}

.dark .profile-danger-icon {
  background: rgba(127, 29, 29, 0.58);
  color: rgb(252, 165, 165);
}

.dark .profile-danger-confirm {
  border-color: rgba(248, 113, 113, 0.24);
  background: rgba(127, 29, 29, 0.18);
}

@media (max-width: 640px) {
  .profile-danger-card {
    border-radius: 1rem;
  }

  .profile-danger-header {
    padding: 1rem;
  }
}
</style>
