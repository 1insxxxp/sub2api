<template>
  <div :class="props.embedded ? 'space-y-4' : 'brand-surface profile-action-card'">
    <div
      v-if="!props.embedded"
      class="profile-action-header"
    >
      <div class="brand-floating-icon profile-action-icon">
        <Icon name="lock" size="md" />
      </div>
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-slate-950 dark:text-white">
          {{ t('profile.changePassword') }}
        </h2>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
          {{ t('profile.passwordHint') }}
        </p>
      </div>
    </div>
    <div :class="props.embedded ? '' : 'px-5 py-5 sm:px-6 sm:py-6'">
      <form @submit.prevent="handleChangePassword" class="space-y-4">
        <div v-if="props.embedded">
          <p class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('profile.changePassword') }}
          </p>
        </div>
        <div>
          <label for="old_password" class="input-label">
            {{ t('profile.currentPassword') }}
          </label>
          <input
            id="old_password"
            v-model="form.old_password"
            type="password"
            required
            autocomplete="current-password"
            class="input"
          />
        </div>

        <div>
          <label for="new_password" class="input-label">
            {{ t('profile.newPassword') }}
          </label>
          <input
            id="new_password"
            v-model="form.new_password"
            type="password"
            required
            autocomplete="new-password"
            class="input"
          />
          <p class="input-hint">
            {{ t('profile.passwordHint') }}
          </p>
        </div>

        <div>
          <label for="confirm_password" class="input-label">
            {{ t('profile.confirmNewPassword') }}
          </label>
          <input
            id="confirm_password"
            v-model="form.confirm_password"
            type="password"
            required
            autocomplete="new-password"
            class="input"
          />
        </div>

        <div class="flex justify-end pt-4">
          <button type="submit" :disabled="loading" class="btn btn-primary">
            {{ loading ? t('profile.changingPassword') : t('profile.changePasswordButton') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { userAPI } from '@/api'

const { t } = useI18n()
const appStore = useAppStore()
const props = withDefaults(defineProps<{
  embedded?: boolean
}>(), {
  embedded: false,
})

const loading = ref(false)
const form = ref({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const handleChangePassword = async () => {
  if (form.value.new_password !== form.value.confirm_password) {
    appStore.showError(t('profile.passwordsNotMatch'))
    return
  }

  if (form.value.new_password.length < 8) {
    appStore.showError(t('profile.passwordTooShort'))
    return
  }

  loading.value = true
  try {
    await userAPI.changePassword(form.value.old_password, form.value.new_password)
    form.value = { old_password: '', new_password: '', confirm_password: '' }
    appStore.showSuccess(t('profile.passwordChangeSuccess'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('profile.passwordChangeFailed'))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.profile-action-card {
  overflow: hidden;
  border-radius: 1.25rem;
}

.profile-action-header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  border-bottom: 1px solid rgba(191, 219, 254, 0.5);
  padding: 1.25rem 1.25rem 1rem;
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.66), rgba(236, 254, 255, 0.42));
}

.profile-action-icon {
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
}

.dark .profile-action-header {
  border-color: rgba(96, 165, 250, 0.16);
  background: linear-gradient(135deg, rgba(30, 64, 175, 0.16), rgba(8, 145, 178, 0.08));
}

@media (max-width: 640px) {
  .profile-action-card {
    border-radius: 1rem;
  }

  .profile-action-header {
    padding: 1rem;
  }
}
</style>
