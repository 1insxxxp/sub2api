<template>
  <div class="profile-info-stack space-y-5 sm:space-y-6">
    <section
      data-testid="profile-overview-hero"
      class="brand-surface profile-overview-card"
    >
      <div class="profile-overview-beam"></div>
      <div class="relative z-10 p-5 sm:p-6 lg:p-7">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-stretch">
          <div
            class="profile-avatar-plate"
          >
            <div class="profile-avatar-frame">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="displayName"
                class="h-full w-full object-cover"
              >
              <DefaultUserAvatar v-else size="xl" />
            </div>
            <div class="profile-avatar-caption">
              <span>{{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}</span>
            </div>
          </div>

          <div class="min-w-0 flex-1 space-y-5">
            <div class="space-y-3">
              <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div class="min-w-0 space-y-2">
                  <h2 class="truncate text-2xl font-semibold text-slate-950 sm:text-3xl dark:text-white">
                  {{ displayName }}
                  </h2>
                  <p class="truncate text-sm font-medium text-slate-600 dark:text-slate-300">
                    {{ primaryEmailDisplay || '-' }}
                  </p>
                </div>

                <div class="flex shrink-0 flex-wrap gap-2">
                  <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
                    {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
                  </span>
                  <span
                    :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']"
                  >
                    {{
                      user?.status === 'active'
                        ? t('common.active')
                        : t('common.disabled')
                    }}
                  </span>
                </div>
              </div>

              <div
                v-if="sourceHints.length"
                class="flex flex-wrap gap-2 text-xs text-slate-500 dark:text-slate-400"
              >
                <span
                  v-for="hint in sourceHints"
                  :key="hint.key"
                  class="brand-floating-chip profile-source-chip"
                >
                  <Icon name="link" size="sm" />
                  {{ hint.text }}
                </span>
              </div>
            </div>

            <div class="grid gap-3 sm:grid-cols-3">
              <div
                data-testid="profile-overview-metric-balance"
                class="brand-floating-card profile-metric-card"
              >
                <div class="profile-metric-icon">
                  <Icon name="dollar" size="sm" />
                </div>
                <div class="min-w-0">
                  <p class="profile-metric-label">
                    {{ t('profile.accountBalance') }}
                  </p>
                  <p class="profile-metric-value">
                    {{ formatCurrency(user?.balance || 0) }}
                  </p>
                </div>
              </div>
              <div
                data-testid="profile-overview-metric-concurrency"
                class="brand-floating-card profile-metric-card"
              >
                <div class="profile-metric-icon">
                  <Icon name="bolt" size="sm" />
                </div>
                <div class="min-w-0">
                  <p class="profile-metric-label">
                    {{ t('profile.concurrencyLimit') }}
                  </p>
                  <p class="profile-metric-value">
                    {{ user?.concurrency || 0 }}
                  </p>
                </div>
              </div>
              <div
                data-testid="profile-overview-metric-member-since"
                class="brand-floating-card profile-metric-card"
              >
                <div class="profile-metric-icon">
                  <Icon name="calendar" size="sm" />
                </div>
                <div class="min-w-0">
                  <p class="profile-metric-label">
                    {{ t('profile.memberSince') }}
                  </p>
                  <p class="profile-metric-value">
                    {{ memberSinceLabel }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <div
      :class="[
        'profile-info-layout grid gap-5 sm:gap-6',
        sourceHints.length ? 'xl:grid-cols-[minmax(0,1fr)_minmax(280px,0.38fr)]' : ''
      ]"
    >
      <div data-testid="profile-main-column" class="space-y-6">
        <section
          data-testid="profile-basics-panel"
          class="brand-surface profile-panel"
        >
          <div class="profile-panel-header">
            <div class="brand-floating-icon profile-panel-icon">
              <Icon name="user" size="md" />
            </div>
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-slate-950 dark:text-white">
                {{ t('profile.basicsTitle') }}
              </h3>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                {{ t('profile.basicsDescription') }}
              </p>
            </div>
          </div>

          <div class="grid gap-4 p-5 sm:p-6 md:grid-cols-2">
            <div class="brand-floating-card profile-embedded-card">
              <ProfileAvatarCard
                :user="user"
                embedded
              />
            </div>

            <div class="brand-floating-card profile-embedded-card">
              <ProfileEditForm
                :initial-username="user?.username || ''"
                embedded
              />
            </div>
          </div>
        </section>

        <section
          data-testid="profile-auth-bindings-panel"
          class="brand-surface profile-panel profile-bindings-panel"
        >
          <div class="p-5 sm:p-6">
            <ProfileIdentityBindingsSection
              :user="user"
              :linuxdo-enabled="linuxdoEnabled"
              :dingtalk-enabled="dingtalkEnabled"
              :oidc-enabled="oidcEnabled"
              :oidc-provider-name="oidcProviderName"
              :wechat-enabled="wechatEnabled"
              :wechat-open-enabled="wechatOpenEnabled"
              :wechat-mp-enabled="wechatMpEnabled"
              embedded
              compact
            />
          </div>
        </section>
      </div>

      <div
        data-testid="profile-side-column"
        :class="sourceHints.length ? 'space-y-5 sm:space-y-6' : 'hidden'"
      >
        <section
          v-if="sourceHints.length"
          class="brand-surface brand-rail profile-source-panel"
        >
          <div class="p-5 pl-7 sm:p-6 sm:pl-8">
            <div class="flex items-start gap-4">
              <div class="profile-source-icon">
                <Icon name="sync" size="md" />
              </div>
              <div class="min-w-0 flex-1">
                <h3 class="text-base font-semibold text-slate-950 dark:text-white">
                  {{ t('profile.linkedProfileSources') }}
                </h3>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                  {{ t('profile.linkedProfileSourcesDescription') }}
                </p>
              </div>
            </div>

            <div class="mt-5 grid gap-3">
              <div
                v-for="hint in sourceHints"
                :key="hint.key"
                class="brand-floating-card profile-source-row"
              >
                <Icon name="link" size="sm" class="mt-0.5 shrink-0 text-primary-500 dark:text-primary-300" />
                <span>{{ hint.text }}</span>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DefaultUserAvatar from '@/components/common/DefaultUserAvatar.vue'
import Icon from '@/components/icons/Icon.vue'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

const props = withDefaults(defineProps<{
  user: User | null
  linuxdoEnabled?: boolean
  dingtalkEnabled?: boolean
  oidcEnabled?: boolean
  oidcProviderName?: string
  wechatEnabled?: boolean
  wechatOpenEnabled?: boolean
  wechatMpEnabled?: boolean
}>(), {
  linuxdoEnabled: false,
  dingtalkEnabled: false,
  oidcEnabled: false,
  oidcProviderName: 'OIDC',
  wechatEnabled: false,
  wechatOpenEnabled: undefined,
  wechatMpEnabled: undefined,
})

const { t } = useI18n()

function normalizeBindingStatus(binding: boolean | UserAuthBindingStatus | undefined): boolean | null {
  if (typeof binding === 'boolean') {
    return binding
  }
  if (!binding) {
    return null
  }
  if (typeof binding.bound === 'boolean') {
    return binding.bound
  }
  return Boolean(binding.provider_subject || binding.issuer || binding.provider_key)
}

function isEmailBound(user: User | null | undefined): boolean {
  if (typeof user?.email_bound === 'boolean') {
    return user.email_bound
  }

  const nested = user?.auth_bindings?.email ?? user?.identity_bindings?.email
  const normalized = normalizeBindingStatus(nested)
  return normalized ?? false
}

const avatarUrl = computed(() => props.user?.avatar_url?.trim() || '')
const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const primaryEmailDisplay = computed(() => {
  const email = props.user?.email?.trim() || ''
  if (!email) {
    return ''
  }
  if (email.endsWith('.invalid') && !isEmailBound(props.user)) {
    return ''
  }
  return email
})
const memberSinceLabel = computed(() => {
  const raw = props.user?.created_at?.trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
  }).format(date)
})

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
  email: t('profile.authBindings.providers.email'),
  linuxdo: t('profile.authBindings.providers.linuxdo'),
  dingtalk: t('profile.authBindings.providers.dingtalk'),
  oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
  wechat: t('profile.authBindings.providers.wechat'),
  github: 'GitHub',
  google: 'Google'
}))

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`
}

function normalizeProvider(value: string): UserAuthProvider | null {
  const normalized = value.trim().toLowerCase()
  if (
    normalized === 'email' ||
    normalized === 'linuxdo' ||
    normalized === 'wechat' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  if (normalized === 'oidc' || normalized.startsWith('oidc:') || normalized.startsWith('oidc/')) {
    return 'oidc'
  }
  return null
}

function readObjectString(source: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = source[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function resolveThirdPartySource(
  rawSource: string | UserProfileSourceContext | null | undefined
): { provider: UserAuthProvider; label: string } | null {
  if (!rawSource) {
    return null
  }

  if (typeof rawSource === 'string') {
    const provider = normalizeProvider(rawSource)
    if (!provider || provider === 'email') {
      return null
    }
    return {
      provider,
      label: providerLabels.value[provider]
    }
  }

  const sourceRecord = rawSource as Record<string, unknown>
  const provider = normalizeProvider(
    readObjectString(sourceRecord, 'provider', 'source', 'provider_type', 'auth_provider')
  )
  if (!provider || provider === 'email') {
    return null
  }

  const explicitLabel = readObjectString(
    sourceRecord,
    'provider_label',
    'label',
    'provider_name',
    'providerName'
  )

  return {
    provider,
    label: explicitLabel || providerLabels.value[provider]
  }
}

const sourceHints = computed(() => {
  const currentUser = props.user
  if (!currentUser) {
    return []
  }

  const hints: Array<{ key: string; text: string }> = []
  const avatarSource = resolveThirdPartySource(
    currentUser.profile_sources?.avatar ?? currentUser.avatar_source
  )
  const usernameSource = resolveThirdPartySource(
    currentUser.profile_sources?.username ??
      currentUser.profile_sources?.display_name ??
      currentUser.profile_sources?.nickname ??
      currentUser.display_name_source ??
      currentUser.username_source ??
      currentUser.nickname_source
  )

  if (avatarSource) {
    hints.push({
      key: 'avatar',
      text: t('profile.authBindings.source.avatar', { providerName: avatarSource.label })
    })
  }

  if (usernameSource) {
    hints.push({
      key: 'username',
      text: t('profile.authBindings.source.username', { providerName: usernameSource.label })
    })
  }

  return hints
})
</script>

<style scoped>
.profile-overview-card,
.profile-panel,
.profile-source-panel {
  border-radius: 1.25rem;
}

.profile-overview-card {
  min-height: 13rem;
}

.profile-overview-beam {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(circle at 0% 0%, rgba(var(--brand-rgb), 0.14), transparent 34%),
    radial-gradient(circle at 100% 0%, rgba(var(--brand-cyan-rgb), 0.14), transparent 32%),
    linear-gradient(135deg, rgba(239, 246, 255, 0.72), transparent 46%);
}

.profile-avatar-plate {
  display: flex;
  min-width: 7.5rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  border-radius: 1.25rem;
  border: 1px solid rgba(191, 219, 254, 0.74);
  background:
    radial-gradient(circle at 20% 0%, rgba(255, 255, 255, 0.9), transparent 42%),
    linear-gradient(135deg, rgba(239, 246, 255, 0.92), rgba(236, 254, 255, 0.78));
  padding: 1rem;
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.9) inset;
}

.profile-avatar-frame {
  display: flex;
  height: 5rem;
  width: 5rem;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 1.35rem;
  background: linear-gradient(135deg, var(--brand-700), var(--brand-500), var(--brand-cyan));
  color: white;
  font-size: 1.75rem;
  font-weight: 800;
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.34) inset,
    0 18px 36px rgba(37, 99, 235, 0.2);
}

.profile-avatar-caption {
  border-radius: 9999px;
  border: 1px solid rgba(191, 219, 254, 0.9);
  background: rgba(255, 255, 255, 0.76);
  padding: 0.3rem 0.65rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--brand-700);
}

.profile-source-chip {
  max-width: 100%;
}

.profile-metric-card {
  display: flex;
  min-height: 5rem;
  align-items: center;
  gap: 0.8rem;
}

.profile-metric-icon {
  display: flex;
  height: 2.35rem;
  width: 2.35rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.9rem;
  border: 1px solid rgba(191, 219, 254, 0.78);
  background: rgba(239, 246, 255, 0.86);
  color: var(--brand-600);
}

.profile-metric-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  font-weight: 700;
  color: rgb(100, 116, 139);
}

.profile-metric-value {
  margin-top: 0.2rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1.05rem;
  font-weight: 800;
  color: rgb(15, 23, 42);
  font-variant-numeric: tabular-nums;
}

.profile-panel {
  overflow: hidden;
}

.profile-panel-header {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  border-bottom: 1px solid rgba(191, 219, 254, 0.5);
  padding: 1.25rem 1.25rem 1rem;
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.66), rgba(236, 254, 255, 0.42));
}

.profile-panel-icon {
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
}

.profile-embedded-card {
  padding: 1.15rem;
}

.profile-source-icon {
  display: flex;
  height: 2.65rem;
  width: 2.65rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.74);
  background: rgba(239, 246, 255, 0.84);
  color: var(--brand-600);
}

.profile-source-row {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  color: rgb(71, 85, 105);
  font-size: 0.875rem;
  line-height: 1.5;
}

.dark .profile-overview-beam {
  background:
    radial-gradient(circle at 0% 0%, rgba(59, 130, 246, 0.18), transparent 34%),
    radial-gradient(circle at 100% 0%, rgba(6, 182, 212, 0.14), transparent 32%),
    linear-gradient(135deg, rgba(30, 64, 175, 0.14), transparent 46%);
}

.dark .profile-avatar-plate {
  border-color: rgba(96, 165, 250, 0.2);
  background:
    radial-gradient(circle at 20% 0%, rgba(96, 165, 250, 0.16), transparent 42%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.86), rgba(8, 13, 28, 0.72));
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

.dark .profile-avatar-caption {
  border-color: rgba(96, 165, 250, 0.22);
  background: rgba(15, 23, 42, 0.76);
  color: rgb(191, 219, 254);
}

.dark .profile-metric-icon,
.dark .profile-source-icon {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.15);
  color: rgb(147, 197, 253);
}

.dark .profile-metric-label {
  color: rgb(148, 163, 184);
}

.dark .profile-metric-value {
  color: white;
}

.dark .profile-panel-header {
  border-color: rgba(96, 165, 250, 0.16);
  background: linear-gradient(135deg, rgba(30, 64, 175, 0.16), rgba(8, 145, 178, 0.08));
}

.dark .profile-source-row {
  color: rgb(203, 213, 225);
}

@media (max-width: 640px) {
  .profile-overview-card,
  .profile-panel,
  .profile-source-panel {
    border-radius: 1rem;
  }

  .profile-avatar-plate {
    min-width: 0;
    flex-direction: row;
    justify-content: flex-start;
  }

  .profile-avatar-frame {
    height: 4.25rem;
    width: 4.25rem;
    border-radius: 1.1rem;
    font-size: 1.45rem;
  }

  .profile-metric-card {
    min-height: 4.4rem;
  }

  .profile-panel-header {
    padding: 1rem;
  }

  .profile-embedded-card {
    padding: 1rem;
  }
}
</style>
