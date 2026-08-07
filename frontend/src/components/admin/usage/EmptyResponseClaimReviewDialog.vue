<template>
  <div v-if="show && claim" class="fixed inset-0 z-[80] flex items-end justify-center bg-slate-950/55 sm:items-center sm:p-4" role="dialog" aria-modal="true">
    <section class="w-full rounded-t-3xl border border-slate-200 bg-white shadow-2xl dark:border-dark-600 dark:bg-dark-800 sm:max-w-lg sm:rounded-2xl">
      <header class="flex items-start justify-between border-b border-slate-100 px-5 py-4 dark:border-dark-700">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-primary-600 dark:text-primary-400">#{{ claim.id }}</p>
          <h3 class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">{{ action === 'approve' ? t('admin.usage.emptyResponseClaims.approve') : t('admin.usage.emptyResponseClaims.reject') }}</h3>
        </div>
        <button type="button" class="h-9 w-9 rounded-xl text-slate-400 hover:bg-slate-100 dark:hover:bg-dark-700" @click="emit('close')">×</button>
      </header>
      <div class="space-y-4 p-5">
        <div class="rounded-2xl bg-slate-50 p-4 text-sm dark:bg-dark-900/60">
          <p class="break-all font-mono font-semibold text-slate-900 dark:text-white">{{ claim.model }}</p>
          <div class="mt-2 flex justify-between text-slate-500 dark:text-slate-400">
            <span>{{ claim.user_email }}</span>
            <span class="font-mono">${{ claim.estimated_refund.toFixed(6) }}</span>
          </div>
        </div>
        <label class="block">
          <span class="mb-2 block text-sm font-medium text-slate-800 dark:text-slate-200">
            {{ action === 'reject' ? t('admin.usage.emptyResponseClaims.rejectionNote') : t('admin.usage.emptyResponseClaims.reviewNote') }}
          </span>
          <textarea v-model="note" rows="4" maxlength="2000" class="input w-full resize-none" />
        </label>
      </div>
      <footer class="flex gap-3 border-t border-slate-100 p-4 dark:border-dark-700">
        <button type="button" class="btn btn-secondary flex-1" :disabled="submitting" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button
          data-testid="submit-claim-review"
          type="button"
          class="btn flex-[1.4]"
          :class="action === 'approve' ? 'btn-primary' : 'bg-rose-600 text-white hover:bg-rose-700'"
          :disabled="submitting || (action === 'reject' && !note.trim())"
          @click="emit('submit', note.trim())"
        >
          {{ submitting ? t('common.processing') : t('common.confirm') }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminEmptyResponseClaim } from '@/api/admin/usage'

const props = defineProps<{
  show: boolean
  claim: AdminEmptyResponseClaim | null
  action: 'approve' | 'reject'
  submitting: boolean
}>()
const emit = defineEmits<{ close: []; submit: [note: string] }>()
const { t } = useI18n()
const note = ref('')
watch(() => props.show, (show) => { if (show) note.value = '' })
</script>
