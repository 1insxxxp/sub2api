<template>
  <AppLayout>
    <div class="user-workspace balance-transfer-workspace">
      <BalanceTransferPanel scope="user" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import '@/styles/user-workspace.css'
import AppLayout from '@/components/layout/AppLayout.vue'
import BalanceTransferPanel from '@/components/redeem/BalanceTransferPanel.vue'
</script>
