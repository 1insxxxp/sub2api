<template>
  <section class="admin-form-section" data-test="support-access-control">
    <label class="flex items-start gap-3 rounded-lg border border-gray-200 bg-white/70 p-3 dark:border-dark-700 dark:bg-dark-900/40">
      <input v-model="enabled" type="checkbox" :disabled="loading || saving || !loaded" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
      <span class="min-w-0">
        <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('userSupport.permission.title') }}</span>
        <span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('userSupport.permission.description') }}</span>
      </span>
    </label>
    <div class="mt-2 flex flex-wrap items-center justify-end gap-3">
      <span v-if="message" :role="failed ? 'alert' : 'status'" class="text-xs" :class="failed ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'">{{ t(message) }}</span>
      <button v-if="!loaded && !loading" type="button" class="btn btn-secondary min-h-10 text-xs" @click="load">{{ t('userSupport.permission.retry') }}</button>
      <button v-if="loaded" data-test="support-access-save" type="button" class="btn btn-secondary min-h-10 text-xs" :disabled="loading || saving || enabled === saved" @click="save">
        {{ t(saving ? 'userSupport.permission.saving' : 'userSupport.permission.save') }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUserAccess, setUserAccess } from '@/api/admin/userSupport'

const props = defineProps<{ userId: number }>()
const { t } = useI18n()
const enabled = ref(false)
const saved = ref(false)
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const message = ref('')
const failed = ref(false)
let version = 0

async function load() {
  const request = ++version
  loaded.value = false; enabled.value = false; message.value = ''; loading.value = true
  try {
    const result = await getUserAccess(props.userId)
    if (request !== version) return
    enabled.value = saved.value = result.enabled === true
    loaded.value = true
  } catch {
    if (request !== version) return
    failed.value = true; message.value = 'userSupport.permission.loadFailed'
  } finally { if (request === version) loading.value = false }
}
async function save() {
  if (!loaded.value || saving.value || loading.value) return
  const request = version
  const userID = props.userId
  saving.value = true; message.value = ''
  try {
    const result = await setUserAccess(userID, enabled.value)
    if (request !== version) return
    enabled.value = saved.value = result.enabled === true
    failed.value = false; message.value = 'userSupport.permission.saved'
  } catch {
    if (request !== version) return
    failed.value = true; message.value = 'userSupport.permission.saveFailed'
  } finally { if (request === version) saving.value = false }
}
watch(() => props.userId, () => { saving.value = false; void load() }, { immediate: true })
onBeforeUnmount(() => { version++ })
</script>
