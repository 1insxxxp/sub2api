<template>
  <div>
    <div
      class="flex min-w-0 flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700/50"
    >
      <p
        class="min-w-0 flex-1 basis-full text-xs text-gray-400 dark:text-gray-500 sm:basis-auto"
      >
        {{ t("admin.usage.rechargeRanking.subtitle") }}
        <span class="ml-1">{{ t("admin.usage.rechargeRanking.countDescription") }}</span>
      </p>
      <div class="flex w-full min-w-0 flex-wrap items-center gap-2 sm:w-auto">
        <select
          v-model="source"
          class="input h-9 min-w-0 flex-1 text-xs sm:w-auto sm:flex-none"
          @change="reloadFirstPage"
        >
          <option value="">
            {{ t("admin.usage.rechargeRanking.allSources") }}
          </option>
          <option
            v-for="option in sourceOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ t(option.label) }}
          </option>
        </select>
        <select
          v-model="sortBy"
          class="input h-9 min-w-0 flex-1 text-xs sm:w-auto sm:flex-none"
          @change="reloadFirstPage"
        >
          <option value="total_amount">
            {{ t("admin.usage.rechargeRanking.columns.total") }}
          </option>
          <option value="recharge_count">
            {{ t("admin.usage.rechargeRanking.columns.count") }}
          </option>
          <option value="last_recharged_at">
            {{ t("admin.usage.rechargeRanking.columns.last") }}
          </option>
        </select>
        <button
          type="button"
          class="btn btn-secondary btn-sm shrink-0"
          :aria-label="
            sortOrder === 'desc'
              ? t('admin.usage.rechargeRanking.sortAsc')
              : t('admin.usage.rechargeRanking.sortDesc')
          "
          @click="toggleSortOrder"
        >
          {{ sortOrder === "desc" ? "↓" : "↑" }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm w-full sm:w-auto"
          @click="exportCsv"
        >
          {{ t("admin.usage.rechargeRanking.exportCurrentPage") }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="py-12 text-center"><LoadingSpinner /></div>
    <div
      v-else-if="error"
      role="alert"
      class="m-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300"
    >
      {{ t("admin.usage.rechargeRanking.loadFailed") }}
      <button type="button" class="btn btn-secondary btn-sm ml-2" @click="load">
        {{ t("common.retry") }}
      </button>
    </div>
    <template v-else>
      <div class="grid grid-cols-2 gap-3 p-4 sm:grid-cols-4">
        <SummaryCard
          :label="t('admin.usage.rechargeRanking.summaryUsers')"
          :value="String(summary.recharge_users || 0)"
        />
        <SummaryCard
          :label="t('admin.usage.rechargeRanking.summaryCount')"
          :value="String(summary.recharge_count || 0)"
        />
        <SummaryCard
          :label="t('admin.usage.rechargeRanking.summaryTotal')"
          :value="`$${money(summary.total_amount)}`"
          tone="green"
        />
        <SummaryCard
          :label="t('admin.usage.rechargeRanking.summaryOnline')"
          :value="`$${money(summary.online_amount)}`"
          tone="green"
        />
      </div>
      <div class="hidden overflow-x-auto md:block">
        <table
          class="w-full min-w-[1100px] divide-y divide-gray-200 dark:divide-dark-700"
        >
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th
                v-for="column in tableColumns"
                :key="column.key"
                class="px-4 py-3 text-right text-xs text-gray-500 first:text-left"
              >
                {{ t(column.label) }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-900">
            <tr v-if="!items.length">
              <td
                :colspan="tableColumns.length"
                class="py-12 text-center text-sm text-gray-400"
              >
                {{ t("admin.dashboard.noDataAvailable") }}
              </td>
            </tr>
            <tr
              v-for="(item, index) in items"
              v-else
              :key="item.user_id"
              class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-700/40"
              @click="$emit('select-user', item.user_id, item.email)"
            >
              <td class="px-4 py-3 text-sm">
                {{ (page - 1) * pageSize + index + 1 }}
              </td>
              <td class="px-4 py-3 text-sm font-medium">
                {{ item.email || item.username || `User #${item.user_id}` }}
              </td>
              <td class="px-4 py-3 text-sm">
                {{ inviterLabel(item) }}
              </td>
              <td
                v-for="column in amountColumns"
                :key="column.key"
                class="px-4 py-3 text-right text-sm tabular-nums"
                :class="
                  column.key === 'total_amount'
                    ? 'font-semibold text-green-600'
                    : ''
                "
              >
                ${{ money(item[column.key] as number) }}
              </td>
              <td class="px-4 py-3 text-right text-sm tabular-nums">
                {{ item.recharge_count }}
              </td>
              <td class="px-4 py-3 text-right text-xs text-gray-500">
                {{ item.last_recharged_at || "-" }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="space-y-3 p-4 md:hidden">
        <div
          v-if="!items.length"
          class="py-10 text-center text-sm text-gray-400"
        >
          {{ t("admin.dashboard.noDataAvailable") }}
        </div>
        <article
          v-for="(item, index) in items"
          v-else
          :key="item.user_id"
          class="min-w-0 rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900"
          @click="$emit('select-user', item.user_id, item.email)"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="truncate text-sm font-medium"
              >#{{ (page - 1) * pageSize + index + 1 }}
              {{ item.email || item.username }}</span
            ><strong class="text-green-600"
              >${{ money(item.total_amount) }}</strong
            >
          </div>
          <dl class="mt-3 grid min-w-0 grid-cols-2 gap-2 text-xs text-gray-500">
            <template v-for="column in mobileColumns" :key="column.key"
              ><dt>{{ t(column.label) }}</dt>
              <dd class="min-w-0 break-words text-right">
                {{ mobileValue(item, column) }}
              </dd></template
            >
          </dl>
        </article>
      </div>
      <Pagination
        v-if="total > pageSize"
        :page="page"
        :total="total"
        :page-size="pageSize"
        @update:page="setPage"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, defineComponent, h } from "vue";
import { useI18n } from "vue-i18n";
import { saveAs } from "file-saver";
import {
  getRechargeRanking,
  type RechargeRankingItem,
  type RechargeRankingParams,
  type RechargeRankingSummary,
} from "@/api/admin/dashboard";
import LoadingSpinner from "@/components/common/LoadingSpinner.vue";
import Pagination from "@/components/common/Pagination.vue";

function csvEscape(value: unknown): string {
  const raw = String(value ?? "");
  const safe = /^[\t\r\n ]*[=+\-@]/.test(raw) ? `'${raw}` : raw;
  return `"${safe.replace(/"/g, '""')}"`;
}

const SummaryCard = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    tone: String,
  },
  setup(props) {
    return () =>
      h(
        "div",
        {
          class: [
            "rounded-lg p-3",
            props.tone === "green"
              ? "bg-green-50 dark:bg-green-500/10"
              : "bg-blue-50 dark:bg-blue-500/10",
          ],
        },
        [
          h("p", { class: "text-xs text-gray-500" }, props.label),
          h(
            "strong",
            props.tone === "green" ? { class: "text-green-600" } : {},
            props.value,
          ),
        ],
      );
  },
});

const props = defineProps<{
  startDate: string;
  endDate: string;
  filters: Record<string, unknown>;
}>();
defineEmits<{ (e: "select-user", userId: number, email: string): void }>();
const { t } = useI18n();
const items = ref<RechargeRankingItem[]>([]);
const summary = ref<RechargeRankingSummary>({
  total_amount: 0,
  online_amount: 0,
  redeem_amount: 0,
  affiliate_amount: 0,
  admin_amount: 0,
  reward_amount: 0,
  refund_amount: 0,
  other_amount: 0,
  recharge_count: 0,
  recharge_users: 0,
});
const loading = ref(false);
const error = ref(false);
const total = ref(0);
const page = ref(1);
const pageSize = ref(50);
const sortBy = ref("total_amount");
const sortOrder = ref<"asc" | "desc">("desc");
const source = ref("");
let sequence = 0;
const sourceOptions = [
  { value: "online", label: "admin.usage.rechargeRanking.sources.online" },
  { value: "redeem", label: "admin.usage.rechargeRanking.sources.redeem" },
  {
    value: "affiliate",
    label: "admin.usage.rechargeRanking.sources.affiliate",
  },
  { value: "admin", label: "admin.usage.rechargeRanking.sources.admin" },
  { value: "reward", label: "admin.usage.rechargeRanking.sources.reward" },
  { value: "refund", label: "admin.usage.rechargeRanking.sources.refund" },
  { value: "other", label: "admin.usage.rechargeRanking.sources.other" },
];
const amountColumns: Array<{
  key: keyof RechargeRankingItem & string;
  label: string;
}> = [
  { key: "total_amount", label: "admin.usage.rechargeRanking.columns.total" },
  { key: "online_amount", label: "admin.usage.rechargeRanking.columns.online" },
  { key: "redeem_amount", label: "admin.usage.rechargeRanking.columns.redeem" },
  {
    key: "affiliate_amount",
    label: "admin.usage.rechargeRanking.columns.affiliate",
  },
  { key: "admin_amount", label: "admin.usage.rechargeRanking.columns.admin" },
  { key: "reward_amount", label: "admin.usage.rechargeRanking.columns.reward" },
  { key: "refund_amount", label: "admin.usage.rechargeRanking.columns.refund" },
  { key: "other_amount", label: "admin.usage.rechargeRanking.columns.other" },
];
const tableColumns = [
  { key: "rank", label: "admin.usage.rechargeRanking.columns.rank" },
  { key: "user", label: "admin.usage.rechargeRanking.columns.user" },
  { key: "inviter", label: "admin.usage.rechargeRanking.columns.inviter" },
  ...amountColumns,
  { key: "recharge_count", label: "admin.usage.rechargeRanking.columns.count" },
  {
    key: "last_recharged_at",
    label: "admin.usage.rechargeRanking.columns.last",
  },
];
const mobileColumns: Array<{ key: string; label: string; amount: boolean }> = (
  [
    ...amountColumns.slice(1),
    {
      key: "inviter",
      label: "admin.usage.rechargeRanking.columns.inviter",
    },
    {
      key: "recharge_count",
      label: "admin.usage.rechargeRanking.columns.count",
    },
  ] as Array<{ key: string; label: string }>)
  .map((column) => ({ ...column, amount: column.key !== "recharge_count" && column.key !== "inviter" }));
const money = (value: number | undefined) => Number(value || 0).toFixed(2);
const mobileValue = (
  item: RechargeRankingItem,
  column: { key: string; amount: boolean },
): string => {
  if (column.key === "inviter") return inviterLabel(item);
  const value = item[column.key as keyof RechargeRankingItem];
  return column.amount ? `$${money(value as number)}` : String(value ?? "-");
};
const inviterLabel = (item: RechargeRankingItem): string => {
  const id = item.inviter_id;
  if (id === null || id === undefined || !id) return t("admin.usage.rechargeRanking.unbound");
  const identity = item.inviter_email || item.inviter_username;
  return identity ? `${identity} (#${id})` : `#${id}`;
};
const reloadFirstPage = () => {
  page.value = 1;
  load();
};
const toggleSortOrder = () => {
  sortOrder.value = sortOrder.value === "desc" ? "asc" : "desc";
  reloadFirstPage();
};
const setPage = (value: number) => {
  page.value = value;
  load();
};
const load = async () => {
  const current = ++sequence;
  loading.value = true;
  error.value = false;
  try {
    const params: RechargeRankingParams = {
      start_date: props.startDate,
      end_date: props.endDate,
      ...props.filters,
      source: source.value || undefined,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      page: page.value,
      page_size: pageSize.value,
    };
    const response = await getRechargeRanking(params);
    if (current !== sequence) return;
    items.value = response.items || [];
    total.value = response.total || 0;
    if (response.summary) summary.value = response.summary;
  } catch {
    if (current === sequence) {
      error.value = true;
      items.value = [];
    }
  } finally {
    if (current === sequence) loading.value = false;
  }
};
const exportCsv = () => {
  const rows = [
    [
      "用户",
      t("admin.usage.rechargeRanking.columns.inviterId"),
      t("admin.usage.rechargeRanking.columns.inviterEmail"),
      t("admin.usage.rechargeRanking.columns.inviterUsername"),
      "总充值",
      "在线充值",
      "兑换码",
      "邀请返利",
      "管理员调整",
      "活动奖励",
      "退款",
      "其他",
      "次数",
      "最后充值时间",
    ],
    ...items.value.map((item) => [
      item.email || item.username || `User #${item.user_id}`,
      item.inviter_id ?? "",
      item.inviter_email || "",
      item.inviter_username || "",
      item.total_amount,
      item.online_amount,
      item.redeem_amount,
      item.affiliate_amount,
      item.admin_amount,
      item.reward_amount,
      item.refund_amount,
      item.other_amount,
      item.recharge_count,
      item.last_recharged_at || "",
    ]),
  ];
  saveAs(
    new Blob(
      ["\ufeff" + rows.map((row) => row.map(csvEscape).join(",")).join("\r\n")],
      { type: "text/csv;charset=utf-8" },
    ),
    "recharge-ranking.csv",
  );
};
watch(
  () => [props.startDate, props.endDate, JSON.stringify(props.filters)],
  reloadFirstPage,
  { immediate: true },
);
defineExpose({ reload: load });
</script>
