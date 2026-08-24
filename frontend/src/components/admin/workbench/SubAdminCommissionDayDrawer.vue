<template>
  <section
    v-if="date"
    data-test="sub-admin-commission-day-drawer"
    class="min-w-0 rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-5 xl:max-h-[calc(100vh-8rem)] xl:overflow-y-auto"
  >
    <div class="mb-4 flex items-start justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-950 dark:text-white">
          {{ t('adminWorkbench.commission.dayDetails') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ date }}</p>
      </div>
      <button type="button" class="btn btn-secondary px-3" @click="emit('close')">
        <Icon name="x" size="sm" />
      </button>
    </div>

    <div v-if="loadingGroups" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="errorMessage" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300">
      {{ errorMessage }}
    </div>
    <div v-else-if="groups.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.noData') }}
    </div>
    <div v-else class="space-y-3">
      <article
        v-for="group in groups"
        :key="group.group_id"
        :data-test="`commission-day-group-${group.group_id}`"
        class="commission-day-group-card min-w-0 rounded-lg border border-gray-200 p-3 dark:border-dark-700 sm:p-4"
      >
        <div class="grid min-w-0 gap-3">
          <div class="min-w-0">
            <h3
              :data-test="`commission-day-group-${group.group_id}-name`"
              class="break-words text-sm font-semibold leading-5 text-gray-950 dark:text-white"
            >
              {{ group.group_name }}
            </h3>
            <p
              :data-test="`commission-day-group-${group.group_id}-metrics`"
              class="mt-1 text-xs tabular-nums text-gray-500 dark:text-dark-400"
            >
              {{ group.requests }} req · {{ group.total_tokens }} tokens
            </p>
          </div>
          <div
            :data-test="`commission-day-group-${group.group_id}-amounts`"
            class="grid min-w-0 grid-cols-1 gap-2 min-[480px]:grid-cols-2"
          >
            <span class="min-w-0 rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-800">
              <span class="block text-[10px] font-medium text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.commission.actualCost') }}
              </span>
              <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-semibold tabular-nums text-gray-900 [scrollbar-width:none] dark:text-white" :title="`$${group.actual_cost.toFixed(2)}`">
                ${{ group.actual_cost.toFixed(2) }}
              </span>
            </span>
            <span class="min-w-0 rounded-md bg-emerald-50 px-2.5 py-2 dark:bg-emerald-500/10">
              <span class="block text-[10px] font-medium text-emerald-700 dark:text-emerald-300">
                {{ t('adminWorkbench.commission.commissionAmount') }}
              </span>
              <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-semibold tabular-nums text-emerald-700 [scrollbar-width:none] dark:text-emerald-300" :title="`$${group.commission_amount.toFixed(2)}`">
                ${{ group.commission_amount.toFixed(2) }}
              </span>
            </span>
          </div>
          <button
            type="button"
            class="btn btn-secondary w-full justify-center px-3"
            :data-test="`commission-day-group-${group.group_id}-toggle`"
            @click="toggleGroup(group.group_id)"
          >
            <span>{{ expandedGroupID === group.group_id ? t('common.collapse') : t('common.expand') }}</span>
          </button>
        </div>

        <div v-if="expandedGroupID === group.group_id" class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-800">
          <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('adminWorkbench.commission.requestLogs') }}
          </h4>
          <div v-if="loadingLogs" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="logs.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.noData') }}
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="log in logs"
              :key="log.id"
              :data-test="`commission-log-${log.id}`"
              class="grid gap-3 rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800/70 sm:gap-2 md:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_auto]"
            >
              <div class="min-w-0">
                <p
                  :data-test="`commission-log-request-${log.id}`"
                  class="break-all font-mono font-semibold text-gray-950 dark:text-white sm:truncate"
                >
                  {{ log.request_id }}
                </p>
                <p
                  :data-test="`commission-log-user-${log.id}`"
                  class="break-all text-xs text-gray-500 dark:text-dark-400 sm:truncate"
                >
                  {{ log.user_email }} · {{ log.api_key_name }}
                </p>
              </div>
              <div class="min-w-0 text-xs text-gray-600 dark:text-dark-300">
                <p
                  :data-test="`commission-log-model-${log.id}`"
                  class="break-all sm:truncate"
                >
                  {{ log.requested_model || log.model }}
                </p>
                <p class="tabular-nums">{{ log.total_tokens }} tokens</p>
              </div>
              <div class="text-left font-semibold tabular-nums text-gray-950 dark:text-white md:text-right">
                ${{ log.actual_cost.toFixed(2) }}
              </div>
            </div>
          </div>
          <Pagination
            v-if="pagination.total > pagination.page_size"
            class="mt-4"
            :page="pagination.page"
            :page-size="pagination.page_size"
            :total="pagination.total"
            :page-size-options="[10, 20, 50]"
            @update:page="handleLogPageChange"
            @update:page-size="handleLogPageSizeChange"
          />
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SubAdminCommissionDayGroup, SubAdminCommissionUsageLog } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'

const props = defineProps<{
  date: string | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()

const groups = ref<SubAdminCommissionDayGroup[]>([])
const logs = ref<SubAdminCommissionUsageLog[]>([])
const expandedGroupID = ref<number | null>(null)
const loadingGroups = ref(false)
const loadingLogs = ref(false)
const errorMessage = ref('')
const pagination = reactive({
  page: 1,
  page_size: 10,
  total: 0
})

async function fetchGroups() {
  if (!props.date) {
    groups.value = []
    return
  }
  loadingGroups.value = true
  errorMessage.value = ''
  try {
    groups.value = await adminAPI.subAdminCommission.getWorkbenchDayGroups(props.date)
  } catch (error: any) {
    errorMessage.value = extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed'))
  } finally {
    loadingGroups.value = false
  }
}

async function fetchLogs() {
  if (!props.date || expandedGroupID.value == null) {
    logs.value = []
    return
  }
  loadingLogs.value = true
  try {
    const response = await adminAPI.subAdminCommission.getWorkbenchDayGroupLogs(
      props.date,
      expandedGroupID.value,
      { page: pagination.page, page_size: pagination.page_size }
    )
    logs.value = response.items
    pagination.total = response.total
    pagination.page = response.page
    pagination.page_size = response.page_size
  } catch (error: any) {
    errorMessage.value = extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed'))
  } finally {
    loadingLogs.value = false
  }
}

function toggleGroup(groupID: number) {
  if (expandedGroupID.value === groupID) {
    expandedGroupID.value = null
    logs.value = []
    return
  }
  expandedGroupID.value = groupID
  pagination.page = 1
  void fetchLogs()
}

function handleLogPageChange(page: number) {
  pagination.page = page
  void fetchLogs()
}

function handleLogPageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void fetchLogs()
}

watch(
  () => props.date,
  () => {
    expandedGroupID.value = null
    logs.value = []
    pagination.page = 1
    void fetchGroups()
  },
  { immediate: true }
)
</script>
