<template>
  <div class="account-table-actions">
    <slot name="before"></slot>
    <button type="button" @click="$emit('refresh')" :disabled="loading" class="btn btn-secondary account-refresh-action" :aria-label="t('common.refresh')" :title="t('common.refresh')">
      <Icon name="refresh" size="sm" :class="[loading ? 'animate-spin' : '']" />
    </button>
    <slot name="after"></slot>
    <slot name="beforeCreate"></slot>
    <button type="button" @click="$emit('create')" class="btn btn-primary account-create-action" :aria-label="t('admin.accounts.createAccount')" :title="t('admin.accounts.createAccount')">
      <Icon name="plus" size="sm" />
      <span class="sm:hidden">{{ t('common.create') }}</span>
      <span class="hidden sm:inline">{{ t('admin.accounts.createAccount') }}</span>
    </button>
    <slot name="afterCreate"></slot>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps<{ loading?: boolean }>()
defineEmits(['refresh', 'create'])

const { t } = useI18n()
</script>

<style scoped>
.account-table-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  min-width: 0;
}

.account-refresh-action,
.account-create-action {
  min-height: 2.25rem;
  border-radius: 6px;
}

.account-refresh-action {
  width: 2.25rem;
  padding: 0;
}

.account-create-action { gap: 0.375rem; }

@media (prefers-reduced-motion: reduce) {
  .account-table-actions :deep(.animate-spin) { animation: none; }
}
</style>
