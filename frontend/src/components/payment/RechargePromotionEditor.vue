<template>
  <div class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <label class="input-label mb-0">{{ t('admin.settings.payment.rechargePromotion.title') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.payment.rechargePromotion.hint') }}</p>
      </div>
      <Toggle :model-value="modelValue.enabled" @update:model-value="updateField('enabled', $event)" />
    </div>

    <div v-if="modelValue.enabled" class="space-y-4 rounded-xl border border-blue-100/70 bg-white/60 p-4 shadow-sm dark:border-blue-500/10 dark:bg-white/[0.03]">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <label class="text-xs text-gray-600 dark:text-gray-300">
          {{ t('admin.settings.payment.rechargePromotion.name') }}
          <input class="input mt-1 w-full" type="text" :value="modelValue.name || ''" :placeholder="t('admin.settings.payment.rechargePromotion.namePlaceholder')" @input="updateField('name', ($event.target as HTMLInputElement).value || undefined)" />
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">
          {{ t('admin.settings.payment.rechargePromotion.startAt') }}
          <input class="input mt-1 w-full" type="datetime-local" :value="startDateTime" @input="updateDate('start_at', ($event.target as HTMLInputElement).value)" />
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">
          {{ t('admin.settings.payment.rechargePromotion.endAt') }}
          <input class="input mt-1 w-full" type="datetime-local" :value="endDateTime" @input="updateDate('end_at', ($event.target as HTMLInputElement).value)" />
        </label>
      </div>
      <label class="block max-w-xs text-xs text-gray-600 dark:text-gray-300">
        {{ t('admin.settings.payment.rechargePromotion.multiplier') }}
        <input class="input mt-1 w-full" type="number" min="0" step="0.01" :value="modelValue.multiplier || ''" @input="updateField('multiplier', Number(($event.target as HTMLInputElement).value))" />
        <span class="mt-1 block text-gray-400">{{ t('admin.settings.payment.rechargePromotion.multiplierHint') }}</span>
      </label>

      <div>
        <label class="input-label">{{ t('admin.settings.payment.rechargePromotion.blacklist') }}</label>
        <p class="mb-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.payment.rechargePromotion.blacklistHint') }}</p>
        <div v-if="selectedUserIds.length" class="mb-2 flex flex-wrap gap-2">
          <span v-for="id in selectedUserIds" :key="id" class="inline-flex items-center gap-1.5 rounded-md bg-gray-100 px-2.5 py-1.5 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-200">
            <span class="max-w-64 truncate">{{ selectedUserLabel(id) }}</span><span class="text-gray-400">#{{ id }}</span>
            <button type="button" class="text-gray-400 hover:text-red-600" :aria-label="t('admin.settings.payment.rechargePromotion.removeUser')" @click="removeUser(id)"><Icon name="x" size="xs" /></button>
          </span>
        </div>
        <div ref="containerRef" class="relative">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input v-model="searchQuery" class="input input-sm w-full pl-9" type="text" autocomplete="off" :placeholder="t('admin.settings.payment.rechargePromotion.searchUsers')" @input="debounceSearch" @focus="showDropdown = true" />
          <div v-if="showDropdown && searchQuery.trim()" class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-700">
            <div v-if="searchLoading" class="px-4 py-3 text-sm text-gray-500">{{ t('common.loading') }}</div>
            <div v-else-if="availableResults.length === 0" class="px-4 py-3 text-sm text-gray-500">{{ t('admin.settings.payment.rechargePromotion.noUsers') }}</div>
            <template v-else>
              <button v-for="user in availableResults" :key="user.id" type="button" class="flex w-full items-center justify-between gap-3 px-4 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-600" @click="selectUser(user)">
                <span class="truncate font-medium">{{ user.email || t('admin.settings.payment.rechargePromotion.userFallback', { id: user.id }) }}</span><span class="shrink-0 text-xs text-gray-400">#{{ user.id }}</span>
              </button>
            </template>
          </div>
        </div>
      </div>
      <p v-if="validationError" role="alert" class="text-sm text-red-500">{{ validationMessage }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { SimpleUser } from '@/api/admin/usage'
import type { BalanceRechargePromotionSettings } from '@/api/admin/settings'
import { localDateTimeToRFC3339, rfc3339ToLocalDateTime, validateRechargePromotion } from './rechargePromotion'

const props = defineProps<{ modelValue: BalanceRechargePromotionSettings }>()
const emit = defineEmits<{ 'update:modelValue': [value: BalanceRechargePromotionSettings] }>()
const { t } = useI18n()
const containerRef = ref<HTMLElement | null>(null)
const searchQuery = ref('')
const searchResults = ref<SimpleUser[]>([])
const searchLoading = ref(false)
const showDropdown = ref(false)
const selectedUsers = ref<Record<number, SimpleUser>>({})
let searchTimer: ReturnType<typeof setTimeout> | null = null
let searchSequence = 0

const selectedUserIds = computed(() => Array.from(new Set((props.modelValue.blacklist_user_ids || []).filter((id) => Number.isInteger(id) && id > 0))))
const availableResults = computed(() => searchResults.value.filter((user) => !selectedUserIds.value.includes(user.id)))
const startDateTime = computed(() => rfc3339ToLocalDateTime(props.modelValue.start_at))
const endDateTime = computed(() => rfc3339ToLocalDateTime(props.modelValue.end_at))
const validationError = computed(() => validateRechargePromotion(props.modelValue))
const validationMessage = computed(() => validationError.value === 'range' ? t('admin.settings.payment.rechargePromotion.invalidRange') : t('admin.settings.payment.rechargePromotion.invalidMultiplier'))

function updateField<K extends keyof BalanceRechargePromotionSettings>(field: K, value: BalanceRechargePromotionSettings[K]) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}
function updateDate(field: 'start_at' | 'end_at', value: string) {
  updateField(field, localDateTimeToRFC3339(value))
}
function selectedUserLabel(id: number) { return selectedUsers.value[id]?.email || t('admin.settings.payment.rechargePromotion.userFallback', { id }) }
function clearSearch() { if (searchTimer) clearTimeout(searchTimer); searchTimer = null; searchSequence += 1 }
function debounceSearch() {
  clearSearch(); showDropdown.value = true
  const query = searchQuery.value.trim()
  if (!query) { searchResults.value = []; searchLoading.value = false; return }
  const sequence = searchSequence
  searchTimer = setTimeout(async () => {
    searchLoading.value = true
    try {
      const numericID = /^\d+$/.test(query) ? Number(query) : 0
      const results = numericID > 0
        ? [await adminAPI.users.getById(numericID, true)].map(user => ({ id: user.id, email: user.email, deleted: Boolean(user.deleted_at) } satisfies SimpleUser))
        : await adminAPI.usage.searchUsers(query)
      if (sequence === searchSequence) searchResults.value = results
    }
    catch { if (sequence === searchSequence) searchResults.value = [] }
    finally { if (sequence === searchSequence) searchLoading.value = false }
  }, 300)
}
function selectUser(user: SimpleUser) {
  selectedUsers.value = { ...selectedUsers.value, [user.id]: user }
  updateField('blacklist_user_ids', [...selectedUserIds.value, user.id])
  clearSearch(); searchQuery.value = ''; searchResults.value = []; showDropdown.value = false
}
function removeUser(id: number) { updateField('blacklist_user_ids', selectedUserIds.value.filter((value) => value !== id)) }
async function hydrateSelectedUsers(ids: number[]) {
  const users = await Promise.all(ids.filter((id) => !selectedUsers.value[id]).map(async (id) => {
    try { const user = await adminAPI.users.getById(id, true); return { id: user.id, email: user.email, deleted: Boolean(user.deleted_at) } satisfies SimpleUser } catch { return null }
  }))
  const next = { ...selectedUsers.value }; for (const user of users) if (user) next[user.id] = user; selectedUsers.value = next
}
function handleDocumentClick(event: MouseEvent) { if (event.target && !containerRef.value?.contains(event.target as Node)) showDropdown.value = false }
watch(selectedUserIds, (ids) => { void hydrateSelectedUsers(ids) }, { immediate: true })
onMounted(() => document.addEventListener('click', handleDocumentClick))
onUnmounted(() => { clearSearch(); document.removeEventListener('click', handleDocumentClick) })
</script>
