<template>
  <div>
    <div
      class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700/50 sm:px-6"
    >
      <p class="text-xs text-gray-400 dark:text-gray-500">
        {{ t("admin.usage.tokenRanking.subtitle") }}
      </p>
      <div class="flex items-center gap-3">
        <select v-model="sortBy" class="input h-9 w-auto text-xs md:hidden" aria-label="排序指标" @change="page = 1; load()">
          <option v-for="column in sortableColumns" :key="column.key" :value="column.key">{{ t(column.label) }}</option>
        </select>
        <button class="btn btn-secondary btn-sm md:hidden" type="button" aria-label="切换排序方向" @click="setSort(sortBy)">{{ sortOrder === 'desc' ? '↓' : '↑' }}</button>
        <span v-if="!loading && total" class="text-xs text-gray-400">{{
          t("admin.usage.tokenRanking.userCount", { count: total })
        }}</span>
        <div class="w-28">
          <Select
            v-model="pageSize"
            :options="limitOptions"
            @change="(v) => changePageSize(v as number)"
          />
        </div>
      </div>
    </div>
    <div
      v-if="error"
      role="alert"
      class="m-4 flex items-center justify-between rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300"
    >
      <span>{{ t("admin.usage.tokenRanking.loadFailed") }}</span
      ><button
        data-test="ranking-retry"
        class="btn btn-secondary btn-sm"
        @click="load"
      >
        {{ t("common.retry") }}
      </button>
    </div>
    <div v-if="loading" class="py-12 text-center"><LoadingSpinner /></div>
    <template v-else-if="!error"
      ><div class="grid grid-cols-2 gap-3 p-4 sm:grid-cols-4">
        <div class="rounded-lg bg-blue-50 p-3 dark:bg-blue-500/10">
          <p class="text-xs text-gray-500">
            {{ t("admin.usage.tokenRanking.summaryUsers") }}
          </p>
          <strong>{{ summary.users.toLocaleString() }}</strong>
        </div>
        <div class="rounded-lg bg-blue-50 p-3 dark:bg-blue-500/10">
          <p class="text-xs text-gray-500">
            {{ t("admin.usage.tokenRanking.summaryRequests") }}
          </p>
          <strong>{{ summary.requests.toLocaleString() }}</strong>
        </div>
        <div class="rounded-lg bg-blue-50 p-3 dark:bg-blue-500/10">
          <p class="text-xs text-gray-500">
            {{ t("admin.usage.tokenRanking.summaryTokens") }}
          </p>
          <strong>{{ fmtTokens(summary.total_tokens) }}</strong>
        </div>
        <div class="rounded-lg bg-green-50 p-3 dark:bg-green-500/10">
          <p class="text-xs text-gray-500">
            {{ t("admin.usage.tokenRanking.summaryCost") }}
          </p>
          <strong>${{ fmtCost(summary.balance_deducted) }}</strong>
        </div>
      </div>
      <div class="hidden overflow-x-auto md:block">
        <table
          class="w-full min-w-[900px] divide-y divide-gray-200 dark:divide-dark-700"
        >
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="w-16 px-4 py-3 text-left text-xs text-gray-500">#</th>
              <th class="px-4 py-3 text-left text-xs text-gray-500">
                {{ t("admin.usage.tokenRanking.columns.user") }}
              </th>
              <th
                v-for="col in sortableColumns"
                :key="col.key"
                :data-sort="col.key"
                class="cursor-pointer whitespace-nowrap px-4 py-3 text-right text-xs text-gray-500 hover:bg-gray-100"
                :class="sortBy === col.key ? 'text-primary-600' : ''"
                @click="setSort(col.key)"
              >
                {{ t(col.label) }}
                <span v-if="sortBy === col.key">{{
                  sortOrder === "asc" ? "↑" : "↓"
                }}</span>
              </th>
              <th class="px-4 py-3 text-right text-xs text-gray-500">
                {{ t("admin.usage.tokenRanking.columns.balance") }}
              </th>
              <th class="px-4 py-3 text-right text-xs text-gray-500">
                {{ t("admin.usage.tokenRanking.columns.lastRequest") }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-900">
            <tr v-if="items.length === 0">
              <td colspan="12" class="py-12 text-center text-sm text-gray-400">
                {{ t("admin.dashboard.noDataAvailable") }}
              </td>
            </tr>
            <tr
              v-for="(item, index) in items"
              v-else
              :key="item.user_id"
              class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-700/40"
              :title="t('admin.usage.tokenRanking.rowHint')"
              @click="$emit('select-user', item.user_id, item.email)"
            >
              <td class="px-4 py-3">
                <span
                  :class="
                    index < 3 ? RANK_BADGE_CLASSES[index] : 'text-gray-400'
                  "
                  class="inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold"
                  >{{ (page - 1) * pageSize + index + 1 }}</span
                >
              </td>
              <td
                class="max-w-[240px] truncate px-4 py-3 text-sm font-medium"
                :title="item.email"
              >
                {{ item.email || `User #${item.user_id}` }}
                <span class="text-xs text-gray-400">#{{ item.user_id }}</span>
              </td>
              <td class="px-4 py-3 text-right text-sm tabular-nums">
                {{ item.requests.toLocaleString() }}
              </td>
              <td class="px-4 py-3 text-right text-sm tabular-nums">
                {{ fmtTokens(item.input_tokens) }}
              </td>
              <td class="px-4 py-3 text-right text-sm tabular-nums">
                {{ fmtTokens(item.output_tokens) }}
              </td>
              <td class="px-4 py-3 text-right text-sm tabular-nums">
                {{ fmtTokens(item.total_tokens) }}
              </td>
              <td
                class="px-4 py-3 text-right text-sm font-medium text-green-600 tabular-nums"
              >
                ${{ fmtCost(item.balance_deducted) }}
              </td>
              <td class="px-4 py-3 text-right text-sm tabular-nums">
                ${{ fmtCost(item.balance ?? 0) }}
              </td>
              <td class="px-4 py-3 text-right text-sm text-gray-500">
                {{ formatDateTime(item.last_request_at) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="space-y-3 p-4 md:hidden">
        <div
          v-if="items.length === 0"
          class="py-10 text-center text-sm text-gray-400"
        >
          {{ t("admin.dashboard.noDataAvailable") }}
        </div>
        <article
          v-for="(item, index) in items"
          v-else
          :key="item.user_id"
          class="rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900"
          @click="$emit('select-user', item.user_id, item.email)"
        >
          <div class="flex items-center justify-between">
            <div class="flex min-w-0 items-center gap-2">
              <span
                :class="
                  index < 3
                    ? RANK_BADGE_CLASSES[index]
                    : 'bg-gray-100 text-gray-500'
                "
                class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
                >{{ (page - 1) * pageSize + index + 1 }}</span
              ><span class="truncate text-sm font-medium">{{
                item.email || `User #${item.user_id}`
              }}</span>
            </div>
            <span class="font-semibold text-green-600"
              >${{ fmtCost(item.balance_deducted) }}</span
            >
          </div>
          <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-500">
            <span
              >{{ t("admin.usage.tokenRanking.columns.requests") }}
              {{ item.requests.toLocaleString() }}</span
            ><span
              >{{ t("admin.usage.tokenRanking.columns.totalTokens") }}
              {{ fmtTokens(item.total_tokens) }}</span
            ><span
              >{{ t("admin.usage.tokenRanking.columns.balance") }} ${{
                fmtCost(item.balance ?? 0)
              }}</span
            ><span>{{ formatDateTime(item.last_request_at) }}</span>
          </div>
        </article>
      </div>
      <Pagination
        v-if="total > pageSize"
        :page="page"
        :total="total"
        :page-size="pageSize"
        @update:page="setPage"
        @update:page-size="changePageSize"
      />
    </template>
  </div>
</template>
<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  getUserBreakdown,
  type UserBreakdownParams,
  type UserBreakdownResponse,
} from "@/api/admin/dashboard";
import {
  formatCompactNumber,
  formatCostFixed,
  formatDateTime as formatDate,
} from "@/utils/format";
import type { UserBreakdownItem } from "@/types";
import Select from "@/components/common/Select.vue";
import Pagination from "@/components/common/Pagination.vue";
import LoadingSpinner from "@/components/common/LoadingSpinner.vue";
const props = defineProps<{
  startDate: string;
  endDate: string;
  filters: Record<string, unknown>;
  model?: string;
}>();
defineEmits<{ (e: "select-user", userId: number, email: string): void }>();
const { t } = useI18n();
type SortKey = NonNullable<UserBreakdownParams["sort_by"]>;
const sortableColumns: { key: SortKey; label: string }[] = [
  { key: "requests", label: "admin.usage.tokenRanking.columns.requests" },
  {
    key: "input_tokens",
    label: "admin.usage.tokenRanking.columns.inputTokens",
  },
  {
    key: "output_tokens",
    label: "admin.usage.tokenRanking.columns.outputTokens",
  },
  {
    key: "total_tokens",
    label: "admin.usage.tokenRanking.columns.totalTokens",
  },
  { key: "balance_deducted", label: "admin.usage.tokenRanking.columns.cost" },
];
const limitOptions = [
  { value: 20, label: "20" },
  { value: 50, label: "50" },
  { value: 100, label: "100" },
];
const RANK_BADGE_CLASSES = [
  "bg-amber-100 text-amber-700",
  "bg-gray-200 text-gray-600",
  "bg-orange-100 text-orange-700",
];
const items = ref<UserBreakdownItem[]>([]);
const loading = ref(false);
const error = ref(false);
const sortBy = ref<SortKey>("balance_deducted");
const sortOrder = ref<"asc" | "desc">("desc");
const page = ref(1);
const pageSize = ref(50);
const total = ref(0);
const summary = ref({
  users: 0,
  requests: 0,
  total_tokens: 0,
  balance_deducted: 0,
});
let seq = 0;
const fmtTokens = (v: number) => formatCompactNumber(v || 0);
const fmtCost = (v: number | undefined) => formatCostFixed(v || 0, 2);
const formatDateTime = (v?: string | null) => (v ? formatDate(v) : "-");
const setSort = (key: SortKey) => {
  if (sortBy.value === key)
    sortOrder.value = sortOrder.value === "desc" ? "asc" : "desc";
  else {
    sortBy.value = key;
    sortOrder.value = "desc";
  }
  page.value = 1;
  load();
};
const setPage = (v: number) => {
  page.value = v;
  load();
};
const changePageSize = (v: number) => {
  pageSize.value = Number(v) || 50;
  page.value = 1;
  load();
};
const load = async () => {
  const current = ++seq;
  loading.value = true;
  error.value = false;
  try {
    const params: UserBreakdownParams = {
      ...props.filters,
      start_date: props.startDate,
      end_date: props.endDate,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      page: page.value,
      page_size: pageSize.value,
    };
    if (props.model) params.model = props.model;
    const res: UserBreakdownResponse = await getUserBreakdown(params);
    if (current !== seq) return;
    items.value = res.users || [];
    total.value = Number(res.total ?? res.users?.length ?? 0);
    summary.value = {
      ...summary.value,
      ...(res.summary || {}),
      users: Number(res.summary?.users ?? res.total ?? res.users?.length ?? 0),
    };
  } catch {
    if (current === seq) {
      items.value = [];
      error.value = true;
    }
  } finally {
    if (current === seq) loading.value = false;
  }
};
watch(
  () => [
    props.startDate,
    props.endDate,
    props.model,
    JSON.stringify(props.filters),
  ],
  () => {
    page.value = 1;
    load();
  },
  { immediate: true },
);
defineExpose({ reload: load });
</script>
