<template>
  <AppLayout>
    <TablePageLayout class="keys-workspace">
      <template #filters>
        <div class="keys-toolbar flex flex-col gap-3" data-test="keys-toolbar">
          <div class="keys-toolbar-main flex flex-wrap items-center justify-between gap-3">
            <div class="keys-filter-controls flex flex-1 flex-wrap items-center gap-3">
              <SearchInput
                v-model="filterSearch"
                :placeholder="t('keys.searchPlaceholder')"
                class="keys-search w-full sm:w-64"
                @search="onFilterChange"
              />
              <div
                id="keys-filter-panel"
                class="keys-filter-selects flex flex-wrap items-center gap-2.5"
                :class="{ 'is-open': showMobileFilters }"
                role="group"
                :aria-label="t('common.filter')"
              >
                <Select
                  :model-value="filterGroupId"
                  class="w-40"
                  :aria-label="t('keys.group')"
                  :options="groupFilterOptions"
                  @update:model-value="onGroupFilterChange"
                />
                <Select
                  :model-value="filterStatus"
                  class="w-40"
                  :aria-label="t('common.status')"
                  :options="statusFilterOptions"
                  @update:model-value="onStatusFilterChange"
                />
                <button
                  v-if="activeFilterCount"
                  type="button"
                  class="keys-clear-filters sm:hidden"
                  data-test="keys-clear-filters"
                  @click="clearMobileFilters"
                >
                  <Icon name="x" size="xs" />
                  {{ t('common.clear') }}
                </button>
              </div>
            </div>
            <div
              class="keys-toolbar-actions flex w-full items-center gap-2 sm:w-auto sm:justify-end sm:gap-3"
              data-test="keys-toolbar-actions"
            >
              <button
                @click="loadApiKeys"
                :disabled="loading"
                data-test="keys-refresh-entry"
                class="keys-refresh-entry btn btn-secondary h-11 w-11 p-0 sm:h-auto sm:w-auto sm:px-4 sm:py-2.5"
                :title="t('common.refresh')"
                :aria-label="t('common.refresh')"
              >
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              </button>
              <div class="keys-desktop-action relative h-11 w-11 sm:h-auto sm:w-auto" ref="columnDropdownRef">
                <button
                  @click="toggleColumnSelector"
                  class="btn btn-secondary h-11 w-11 p-0 sm:h-auto sm:w-auto sm:px-2 sm:py-2.5 md:px-3"
                  :title="t('keys.columnSettings')"
                  :aria-label="t('keys.columnSettings')"
                >
                  <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
                  </svg>
                  <span class="hidden md:inline">{{ t('keys.columnSettings') }}</span>
                </button>
                <div
                  v-if="showColumnDropdown && !isMobileColumnSelector"
                  class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800"
                >
                  <button
                    v-for="col in toggleableColumns"
                    :key="col.key"
                    @click="toggleColumn(col.key)"
                    class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700"
                  >
                    <span>{{ col.label }}</span>
                    <Icon
                      v-if="isColumnVisible(col.key)"
                      name="check"
                      size="sm"
                      class="text-primary-500"
                      :stroke-width="2"
                    />
                  </button>
                </div>
              </div>
              <button
                type="button"
                class="keys-desktop-action btn btn-secondary keys-mobile-secondary-action h-11 min-w-0 gap-1.5 whitespace-nowrap px-2 text-xs sm:h-auto sm:gap-2 sm:px-4 sm:py-2.5 sm:text-sm"
                data-test="custom-groups-entry"
                @click="showCustomGroupsModal = true"
              >
                <Icon name="grid" size="sm" class="shrink-0 max-[359px]:hidden" />
                {{ t('nav.customGroups') }}
              </button>
              <button
                @click="showCreateModal = true"
                data-test="keys-create-entry"
                class="btn btn-primary h-11 min-w-0 gap-1.5 whitespace-nowrap px-2 text-xs sm:h-auto sm:gap-2 sm:px-4 sm:py-2.5 sm:text-sm"
                data-tour="keys-create-btn"
              >
                <Icon name="plus" size="sm" class="shrink-0 max-[359px]:hidden" />
                {{ t('keys.createKey') }}
              </button>
            </div>
            <div class="keys-mobile-utilities">
              <button
                type="button"
                class="keys-filter-toggle"
                data-test="keys-filter-toggle"
                :class="{ 'is-active': activeFilterCount > 0 }"
                :aria-label="t('common.filter')"
                :title="t('common.filter')"
                :aria-expanded="showMobileFilters"
                aria-controls="keys-filter-panel"
                @click="showMobileFilters = !showMobileFilters"
              >
                <Icon name="filter" size="sm" />
                <span class="keys-utility-label">{{ t('common.filter') }}</span>
                <span v-if="activeFilterCount" data-test="keys-filter-count" class="keys-filter-count">{{ activeFilterCount }}</span>
              </button>
              <button
                v-if="endpointCount"
                ref="mobileEndpointsTriggerRef"
                type="button"
                class="keys-endpoints-toggle"
                data-test="keys-endpoints-toggle"
                :aria-label="t('keys.endpoints.routes')"
                :title="t('keys.endpoints.routes')"
                aria-haspopup="dialog"
                :aria-expanded="showMobileEndpoints"
                :aria-controls="showMobileEndpoints ? 'keys-endpoint-popover' : undefined"
                @click="toggleMobileEndpoints"
                @keydown.down.prevent="openMobileEndpoints"
              >
                <Icon name="globe" size="sm" />
                <span class="keys-utility-label">{{ t('keys.endpoints.routesCompact') }}</span>
              </button>
              <button
                type="button"
                class="keys-custom-groups-entry"
                data-test="keys-mobile-custom-groups"
                aria-haspopup="dialog"
                @click="showCustomGroupsModal = true"
              >
                <Icon name="grid" size="sm" />
                {{ t('nav.customGroups') }}
              </button>
              <label class="keys-toolbar-selection" :title="t('common.selectAll')">
                <input
                  type="checkbox"
                  data-test="keys-select-all-toolbar"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
                  :aria-label="t('common.selectAll')"
                  :checked="allVisibleKeysSelected"
                  :indeterminate="selectedApiKeys.length > 0 && !allVisibleKeysSelected"
                  :disabled="loading || apiKeys.length === 0"
                  @change="handleSelectionChange(($event.target as HTMLInputElement).checked ? apiKeys.map(key => key.id) : [])"
                />
                <span class="hidden min-[360px]:inline">{{ t('common.selectAll') }}</span>
              </label>
              <button
                id="keys-more-trigger"
                ref="mobileToolsTriggerRef"
                type="button"
                class="keys-more-toggle"
                data-test="keys-more-toggle"
                :aria-label="t('common.more')"
                :title="t('common.more')"
                aria-haspopup="menu"
                :aria-expanded="showMobileTools"
                :aria-controls="showMobileTools ? 'keys-tools-menu' : undefined"
                @click="toggleMobileTools"
                @keydown.down.prevent="openMobileTools"
                @keydown.up.prevent="openMobileTools"
              >
                <Icon name="more" size="md" />
              </button>
            </div>
          </div>
          <EndpointPopover
            v-if="endpointCount"
            class="keys-inline-endpoints"
            :api-base-url="publicSettings?.api_base_url || ''"
            :custom-endpoints="publicSettings?.custom_endpoints || []"
          />
          <div v-if="selectedIds.length" class="hidden flex-wrap items-center gap-3 text-sm md:flex">
            <span class="text-gray-600 dark:text-gray-300">
              {{ t('keys.bulkEdit.selectedCount', { count: selectedIds.length }) }}
            </span>
            <button
              class="btn btn-primary btn-sm"
              :disabled="loading"
              data-test="bulk-edit-keys"
              @click="showBulkEditModal = true"
            >
              {{ t('keys.bulkEdit.title') }}
            </button>
            <button class="btn btn-secondary btn-sm" @click="selectedIds = []">
              {{ t('keys.bulkEdit.clearSelection') }}
            </button>
          </div>
        </div>
      </template>
      <template #table>
        <div class="keys-table" :class="{ 'has-selection': selectedIds.length > 0 }">
          <DataTable
            :columns="columns"
            :data="apiKeys"
            :loading="loading"
            selectable
            row-key="id"
            :selected-keys="selectedIds"
            :selection-label="(key: ApiKey) => t('keys.bulkEdit.selectKey', { name: key.name })"
            @update:selected-keys="handleSelectionChange"
            :server-side-sort="true"
            default-sort-key="created_at"
            default-sort-order="desc"
            @sort="handleSort"
          >
          <template #mobile-selection-actions>
            <div v-if="selectedIds.length" class="keys-mobile-selection-actions">
              <span class="text-gray-600 dark:text-gray-300" :title="t('keys.bulkEdit.selectedCount', { count: selectedIds.length })">
                {{ t('keys.bulkEdit.selectedCountCompact', { count: selectedIds.length }) }}
              </span>
              <button
                class="btn btn-primary btn-sm"
                :disabled="loading"
                data-test="bulk-edit-keys-mobile"
                @click="showBulkEditModal = true"
              >
                {{ t('keys.bulkEdit.title') }}
              </button>
              <button
                class="btn btn-secondary btn-sm"
                :title="t('keys.bulkEdit.clearSelection')"
                :aria-label="t('keys.bulkEdit.clearSelection')"
                @click="selectedIds = []"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </template>
          <template #mobile-row="{ row, columns: mobileColumns, cells, selectable, selected, selectionLabel, select }">
            <KeyMobileCard
              :row="row"
              :columns="mobileColumns"
              :cells="cells"
              :selectable="selectable"
              :selected="selected"
              :selection-label="selectionLabel"
              @select="select"
            />
          </template>
          <template #cell-id="{ value }">
            <span class="font-mono text-xs text-gray-500 dark:text-gray-400">#{{ value }}</span>
          </template>

          <template #cell-key="{ value, row }">
            <div class="keys-key-value flex items-center gap-2">
              <code class="code text-xs">
                {{ maskApiKey(value) }}
              </code>
              <button
                @click="copyToClipboard(value, row.id)"
                class="rounded-lg p-1 transition-colors hover:bg-gray-100 dark:hover:bg-dark-700"
                :class="
                  copiedKeyId === row.id
                    ? 'text-green-500'
                    : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                "
                :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
              >
                <Icon
                  v-if="copiedKeyId === row.id"
                  name="check"
                  size="sm"
                  :stroke-width="2"
                />
                <Icon v-else name="clipboard" size="sm" />
              </button>
            </div>
          </template>

          <template #cell-name="{ value, row }">
            <div class="keys-name flex items-center gap-1.5">
              <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
              <Icon
                v-if="row.ip_whitelist?.length > 0 || row.ip_blacklist?.length > 0"
                name="shield"
                size="sm"
                class="text-blue-500"
                :title="t('keys.ipRestrictionEnabled')"
              />
            </div>
          </template>

          <template #cell-group="{ row }">
            <div class="group/dropdown relative">
              <button
                :ref="(el) => setGroupButtonRef(row.id, el)"
                @click="openGroupSelector(row)"
                data-test="group-selector-trigger"
                class="keys-group-selector -mx-2 -my-1 flex max-w-full min-w-0 flex-wrap cursor-pointer items-center gap-2 rounded-lg px-2 py-1 transition-all duration-200 hover:bg-gray-100 dark:hover:bg-dark-700"
                :title="t('keys.clickToChangeGroup')"
              >
                <GroupBadge
                  v-if="row.group"
                  :name="row.group.name"
                  :platform="row.group.platform"
                  :subscription-type="row.group.subscription_type"
                  :rate-multiplier="row.group.rate_multiplier"
                  :user-rate-multiplier="userGroupRates[row.group.id]"
                  :peak-rate-enabled="row.group.peak_rate_enabled"
                  :peak-start="row.group.peak_start"
                  :peak-end="row.group.peak_end"
                  :peak-rate-multiplier="row.group.peak_rate_multiplier"
                  :wrap-name="true"
                />
                <span
                  v-else-if="row.custom_group"
                  class="group-badge-wrap inline-flex max-w-full items-center gap-1.5 rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300"
                >
                  <PlatformIcon
                    platform="composite"
                    size="sm"
                    class="keys-custom-group-icon"
                    aria-hidden="true"
                  />
                  <span data-test="group-badge-name" class="min-w-0 whitespace-normal [overflow-wrap:anywhere] sm:truncate">
                    {{ row.custom_group.name }}
                  </span>
                </span>
                <span v-else class="text-sm text-gray-400 dark:text-dark-500">{{
                  t('keys.noGroup')
                }}</span>
                <span class="keys-group-hint text-xs text-gray-500 dark:text-gray-400">{{ t('keys.selectGroup') }}</span>
                <svg
                  class="h-3.5 w-3.5 text-gray-400 opacity-60 transition-opacity group-hover/dropdown:opacity-100"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M8.25 15L12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9"
                  />
                </svg>
              </button>
            </div>
          </template>

          <template #cell-current_concurrency="{ value }">
            <span
              :class="[
                'inline-flex min-w-8 items-center justify-center rounded px-2 py-1 text-sm font-semibold tabular-nums',
                (value ?? 0) > 0
                  ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-300 dark:ring-emerald-800'
                  : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400'
              ]"
            >
              {{ value ?? 0 }}
            </span>
          </template>

          <template #cell-usage="{ row }">
            <div class="keys-usage text-sm">
              <div class="keys-usage-period flex items-center gap-1.5">
                <span class="text-gray-500 dark:text-gray-400">{{ t('keys.today') }}:</span>
                <span class="font-medium text-gray-900 dark:text-white">
                  ${{ (usageStats[row.id]?.today_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
              <div class="keys-usage-period mt-0.5 flex items-center gap-1.5">
                <span class="text-gray-500 dark:text-gray-400">{{ t('keys.total') }}:</span>
                <span class="font-medium text-gray-900 dark:text-white">
                  ${{ (usageStats[row.id]?.total_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
              <!-- Quota progress (if quota is set) -->
              <div v-if="row.quota > 0" class="keys-quota mt-1.5">
                <div class="flex items-center gap-1.5">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('keys.quota') }}:</span>
                  <span :class="[
                    'font-medium',
                    row.quota_used >= row.quota ? 'text-red-500' :
                    row.quota_used >= row.quota * 0.8 ? 'text-yellow-500' :
                    'text-gray-900 dark:text-white'
                  ]">
                    ${{ row.quota_used?.toFixed(2) || '0.00' }} / ${{ row.quota?.toFixed(2) }}
                  </span>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.quota_used >= row.quota ? 'bg-red-500' :
                      row.quota_used >= row.quota * 0.8 ? 'bg-yellow-500' :
                      'bg-primary-500'
                    ]"
                    :style="{ width: Math.min((row.quota_used / row.quota) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-rate_limit="{ row }">
            <div v-if="row.rate_limit_5h > 0 || row.rate_limit_1d > 0 || row.rate_limit_7d > 0" class="space-y-1.5 min-w-[140px]">
              <!-- 5h window -->
              <div v-if="row.rate_limit_5h > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">5h</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_5h >= row.rate_limit_5h ? 'text-red-500' :
                    row.usage_5h >= row.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ${{ row.usage_5h?.toFixed(2) || '0.00' }}/${{ row.rate_limit_5h?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_5h >= row.rate_limit_5h ? 'bg-red-500' :
                      row.usage_5h >= row.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_5h / row.rate_limit_5h) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_5h_at && formatResetTime(row.reset_5h_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_5h_at) }}
                </div>
              </div>
              <!-- 1d window -->
              <div v-if="row.rate_limit_1d > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">1d</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_1d >= row.rate_limit_1d ? 'text-red-500' :
                    row.usage_1d >= row.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ${{ row.usage_1d?.toFixed(2) || '0.00' }}/${{ row.rate_limit_1d?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_1d >= row.rate_limit_1d ? 'bg-red-500' :
                      row.usage_1d >= row.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_1d / row.rate_limit_1d) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_1d_at && formatResetTime(row.reset_1d_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_1d_at) }}
                </div>
              </div>
              <!-- 7d window -->
              <div v-if="row.rate_limit_7d > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">7d</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_7d >= row.rate_limit_7d ? 'text-red-500' :
                    row.usage_7d >= row.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ${{ row.usage_7d?.toFixed(2) || '0.00' }}/${{ row.rate_limit_7d?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_7d >= row.rate_limit_7d ? 'bg-red-500' :
                      row.usage_7d >= row.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_7d / row.rate_limit_7d) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_7d_at && formatResetTime(row.reset_7d_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_7d_at) }}
                </div>
              </div>
              <!-- Reset button -->
              <button
                v-if="row.usage_5h > 0 || row.usage_1d > 0 || row.usage_7d > 0"
                @click.stop="confirmResetRateLimitFromTable(row)"
                class="mt-0.5 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('keys.resetRateLimitUsage')"
              >
                <Icon name="refresh" size="xs" />
                {{ t('keys.resetUsage') }}
              </button>
            </div>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-expires_at="{ value }">
            <span v-if="value" :class="[
              'text-sm',
              new Date(value) < new Date() ? 'text-red-500 dark:text-red-400' : 'text-gray-500 dark:text-dark-400'
            ]">
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">{{ t('keys.noExpiration') }}</span>
          </template>

          <template #cell-status="{ value }">
            <span :class="[
              'badge',
              value === 'active' ? 'badge-success' :
              value === 'quota_exhausted' ? 'badge-warning' :
              value === 'expired' ? 'badge-danger' :
              'badge-gray'
            ]">
              {{ t('keys.status.' + value) }}
            </span>
          </template>

          <template #cell-last_used_at="{ value }">
            <span v-if="value" class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-last_used_ip="{ value }">
            <span v-if="value" class="text-sm text-gray-500 dark:text-dark-400">
              {{ value }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row, mobile }">
            <div class="keys-row-actions flex items-center gap-1">
              <!-- Use Key Button -->
              <button
                v-if="!mobile"
                @click="openUseKeyModal(row)"
                :title="t('keys.useKey')"
                :aria-label="t('keys.useKey')"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400"
              >
                <Icon name="terminal" size="sm" />
                <span class="text-xs">{{ t('keys.useKey') }}</span>
              </button>
              <!-- Import to CC Switch Button -->
              <button
                v-if="!mobile && !publicSettings?.hide_ccs_import_button"
                @click="importToCcswitch(row)"
                :title="t('keys.importToCcSwitch')"
                :aria-label="t('keys.importToCcSwitch')"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
              >
                <Icon name="upload" size="sm" />
                <span class="text-xs md:hidden">{{ t('common.import') }}</span>
                <span class="hidden text-xs md:inline">{{ t('keys.importToCcSwitch') }}</span>
              </button>
              <!-- Toggle Status Button -->
              <button
                @click="toggleKeyStatus(row)"
                :title="row.status === 'active' ? t('keys.disable') : t('keys.enable')"
                :aria-label="row.status === 'active' ? t('keys.disable') : t('keys.enable')"
                :class="[
                  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors',
                  row.status === 'active'
                    ? 'text-gray-500 hover:bg-yellow-50 hover:text-yellow-600 dark:hover:bg-yellow-900/20 dark:hover:text-yellow-400'
                    : 'text-gray-500 hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400'
                ]"
              >
                <Icon v-if="row.status === 'active'" name="ban" size="sm" />
                <Icon v-else name="checkCircle" size="sm" />
                <span class="text-xs">{{ row.status === 'active' ? t('keys.disable') : t('keys.enable') }}</span>
              </button>
              <!-- Edit Button -->
              <button
                @click="editKey(row)"
                :title="t('common.edit')"
                :aria-label="t('common.edit')"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit') }}</span>
              </button>
              <!-- Delete Button -->
              <button
                @click="confirmDelete(row)"
                :title="t('common.delete')"
                :aria-label="t('common.delete')"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('keys.noKeysYet')"
              :description="t('keys.createFirstKey')"
              :action-text="t('keys.createKey')"
              @action="showCreateModal = true"
            />
          </template>
          </DataTable>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          variant="compact"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <Teleport to="body">
      <Transition name="keys-menu">
        <div
          v-if="showMobileEndpoints"
          id="keys-endpoint-popover"
          ref="mobileEndpointsPanelRef"
          class="keys-mobile-tools keys-mobile-endpoints"
          :style="mobileEndpointsPosition"
          data-test="keys-endpoint-popover"
          role="dialog"
          aria-modal="false"
          :aria-label="t('keys.endpoints.routes')"
          tabindex="-1"
          @keydown.esc.stop.prevent="closeMobileEndpoints"
          @keydown.tab="handleEndpointsTab"
        >
          <EndpointPopover
            inline-details
            :api-base-url="publicSettings?.api_base_url || ''"
            :custom-endpoints="publicSettings?.custom_endpoints || []"
          />
        </div>
      </Transition>
      <Transition name="keys-menu">
        <div
          v-if="showMobileTools"
          id="keys-tools-menu"
          ref="mobileToolsMenuRef"
          class="keys-mobile-tools"
          :style="mobileToolsPosition"
          data-test="keys-tools-menu"
          role="menu"
          aria-labelledby="keys-more-trigger"
          @keydown.esc.stop.prevent="closeMobileTools"
          @keydown.tab="closeMobileTools"
          @keydown.up.prevent
          @keydown.down.prevent
        >
          <button type="button" role="menuitem" tabindex="-1" data-test="keys-mobile-columns" @click.stop="runMobileTool(toggleColumnSelector)">
            <Icon name="cog" size="md" />{{ t('keys.columnSettings') }}
            <Icon name="chevronRight" size="sm" class="ml-auto opacity-40" />
          </button>
        </div>
      </Transition>
    </Teleport>

    <BaseDialog
      :show="showCustomGroupsModal"
      :title="t('nav.customGroups')"
      width="full"
      appearance="neutral"
      data-test="custom-groups-dialog"
      @close="showCustomGroupsModal = false"
    >
      <div class="keys-groups-content">
        <CustomGroupsManager appearance="neutral" @changed="loadCustomGroups" />
      </div>
    </BaseDialog>

    <!-- Create/Edit Modal -->
    <BaseDialog
      :show="showCreateModal || showEditModal"
      :title="showEditModal ? t('keys.editKey') : t('keys.createKey')"
      width="normal"
      appearance="neutral"
      @close="closeModals"
    >
      <form id="key-form" @submit.prevent="handleSubmit" class="keys-key-form space-y-4">
        <div>
          <label class="input-label">{{ t('keys.nameLabel') }}</label>
          <input
            v-model="formData.name"
            type="text"
            required
            class="input"
            :placeholder="t('keys.namePlaceholder')"
            data-tour="key-form-name"
          />
        </div>

        <fieldset v-if="!showEditModal" data-tour="key-form-provider">
          <legend class="input-label">{{ t('keys.providerLabel') }}</legend>
          <div class="keys-provider-options grid grid-cols-2 gap-2">
            <label
              v-for="provider in createProviderOptions"
              :key="provider.value"
              class="relative min-w-0"
              :class="provider.count === 0 ? 'cursor-not-allowed' : 'cursor-pointer'"
            >
              <input
                type="radio"
                name="key-provider"
                :value="provider.value"
                :checked="createProvider === provider.value"
                :disabled="provider.count === 0"
                class="peer sr-only"
                @change="selectCreateProvider(provider.value)"
              />
              <span
                class="keys-provider-option flex h-full items-center gap-2 rounded-md border border-gray-200 px-2.5 py-2 transition-colors peer-checked:border-primary-500 peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary-500 peer-disabled:opacity-40 dark:border-dark-600"
                :class="provider.count > 0 && 'hover:border-primary-300 dark:hover:border-primary-700'"
              >
                <span class="keys-provider-icons flex shrink-0 items-center justify-center" aria-hidden="true">
                  <span
                    v-for="platform in KEY_GROUP_PROVIDER_ICONS[provider.value]"
                    :key="platform"
                    class="flex h-6 w-6 items-center justify-center rounded"
                    :class="platformBadgeLightClass(platform)"
                  >
                    <PlatformIcon :platform="platform" size="sm" />
                  </span>
                </span>
                <span class="keys-provider-label text-xs font-medium text-gray-800 dark:text-gray-100">{{ provider.label }}</span>
              </span>
              <span
                v-if="createProvider === provider.value"
                class="absolute right-2 top-1/2 flex h-3.5 w-3.5 -translate-y-1/2 items-center justify-center text-primary-500"
                aria-hidden="true"
              >
                <Icon name="check" size="xs" :stroke-width="3" />
              </span>
            </label>
          </div>
          <p class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400" aria-live="polite">
            {{ groups.length === 0 ? t('common.noGroupsAvailable') : t(`keys.providerHints.${createProvider}`) }}
          </p>
        </fieldset>

        <div>
          <label class="input-label" for="key-form-group">{{ t('keys.groupLabel') }}</label>
          <Select
            :key="showEditModal ? 'edit' : createProvider"
            id="key-form-group"
            :aria-label="t('keys.groupLabel')"
            v-model="formData.group_id"
            @update:modelValue="formData.custom_group_id = null"
            :options="formGroupOptions"
            :placeholder="t('keys.selectGroup')"
            :empty-text="t('common.noGroupsAvailable')"
            :searchable="true"
            :search-placeholder="t('keys.searchGroup')"
            data-tour="key-form-group"
          >
            <template #selected="{ option }">
              <GroupBadge
                v-if="option"
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :user-rate-multiplier="(option as unknown as GroupOption).userRate"
                :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
                :peak-start="(option as unknown as GroupOption).peakStart"
                :peak-end="(option as unknown as GroupOption).peakEnd"
                :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
              />
              <span v-else class="text-gray-400">{{ t('keys.selectGroup') }}</span>
            </template>
            <template #option="{ option, selected }">
              <GroupOptionItem
                :tag="(option as unknown as GroupOption).tag"
                :tag-color="(option as unknown as GroupOption).tag_color"
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :user-rate-multiplier="(option as unknown as GroupOption).userRate"
                :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
                :peak-start="(option as unknown as GroupOption).peakStart"
                :peak-end="(option as unknown as GroupOption).peakEnd"
                :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
                :description="(option as unknown as GroupOption).description"
                :selected="selected"
              />
            </template>
          </Select>
        </div>

        <div>
          <label class="input-label">{{ t('nav.customGroups') }}</label>
          <Select
            v-model="formData.custom_group_id"
            :options="customGroupOptions"
            :searchable="customGroups.length > 5"
            data-test="custom-group-selector"
            @update:modelValue="formData.group_id = null"
          />
          <p class="input-hint">选择后，此 Key 会根据请求模型转发到你配置的来源分组。</p>
        </div>

        <!-- Custom Key Section (only for create) -->
        <div v-if="!showEditModal" class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.customKeyLabel') }}</label>
            <button
              type="button"
              role="switch"
              :aria-checked="formData.use_custom_key"
              :aria-label="t('keys.customKeyLabel')"
              @click="formData.use_custom_key = !formData.use_custom_key"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.use_custom_key ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.use_custom_key ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <div v-if="formData.use_custom_key">
            <input
              v-model="formData.custom_key"
              type="text"
              class="input font-mono"
              :placeholder="t('keys.customKeyPlaceholder')"
              :class="{ 'border-red-500 dark:border-red-500': customKeyError }"
            />
            <p v-if="customKeyError" class="mt-1 text-sm text-red-500">{{ customKeyError }}</p>
            <p v-else class="input-hint">{{ t('keys.customKeyHint') }}</p>
          </div>
        </div>

        <div v-if="showEditModal">
          <label class="input-label">{{ t('keys.statusLabel') }}</label>
          <Select
            v-model="formData.status"
            :options="statusOptions"
            :placeholder="t('keys.selectStatus')"
          />
        </div>

        <!-- IP Restriction Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
            <button
              type="button"
              role="switch"
              :aria-checked="formData.enable_ip_restriction"
              :aria-label="t('keys.ipRestriction')"
              @click="formData.enable_ip_restriction = !formData.enable_ip_restriction"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_ip_restriction ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_ip_restriction ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_ip_restriction" class="space-y-4 pt-2">
            <div>
              <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
              <textarea
                v-model="formData.ip_whitelist"
                rows="3"
                class="input font-mono text-sm"
                :placeholder="t('keys.ipWhitelistPlaceholder')"
              />
              <p class="input-hint">{{ t('keys.ipWhitelistHint') }}</p>
            </div>

            <div>
              <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
              <textarea
                v-model="formData.ip_blacklist"
                rows="3"
                class="input font-mono text-sm"
                :placeholder="t('keys.ipBlacklistPlaceholder')"
              />
              <p class="input-hint">{{ t('keys.ipBlacklistHint') }}</p>
            </div>
          </div>
        </div>

        <!-- Quota Limit Section -->
        <div class="space-y-3">
          <label class="input-label">{{ t('keys.quotaLimit') }}</label>
          <!-- Switch commented out - always show input, 0 = unlimited
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.quotaLimit') }}</label>
            <button
              type="button"
              @click="formData.enable_quota = !formData.enable_quota"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_quota ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_quota ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          -->

          <div class="space-y-4">
            <div>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.quota"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="t('keys.quotaAmountPlaceholder')"
                />
              </div>
              <p class="input-hint">{{ t('keys.quotaAmountHint') }}</p>
            </div>

            <!-- Quota used display (only in edit mode) -->
            <div v-if="showEditModal && selectedKey && selectedKey.quota > 0">
              <label class="input-label">{{ t('keys.quotaUsed') }}</label>
              <div class="flex items-center gap-2">
                <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700">
                  <span class="font-medium text-gray-900 dark:text-white">
                    ${{ selectedKey.quota_used?.toFixed(4) || '0.0000' }}
                  </span>
                  <span class="mx-2 text-gray-400">/</span>
                  <span class="text-gray-500 dark:text-gray-400">
                    ${{ selectedKey.quota?.toFixed(2) || '0.00' }}
                  </span>
                </div>
                <button
                  type="button"
                  @click="confirmResetQuota"
                  class="btn btn-secondary text-sm"
                  :title="t('keys.resetQuotaUsed')"
                >
                  {{ t('keys.reset') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Rate Limit Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
            <button
              type="button"
              @click="formData.enable_rate_limit = !formData.enable_rate_limit"
              role="switch"
              :aria-checked="formData.enable_rate_limit"
              :aria-label="t('keys.rateLimitSection')"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_rate_limit ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_rate_limit ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_rate_limit" class="space-y-4 pt-2">
            <p class="input-hint -mt-2">{{ t('keys.rateLimitHint') }}</p>
            <!-- 5-Hour Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit5h') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.rate_limit_5h"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_5h > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'text-red-500' :
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ${{ selectedKey.usage_5h?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ${{ selectedKey.rate_limit_5h?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'bg-red-500' :
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_5h / selectedKey.rate_limit_5h) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- Daily Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit1d') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.rate_limit_1d"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_1d > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'text-red-500' :
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ${{ selectedKey.usage_1d?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ${{ selectedKey.rate_limit_1d?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'bg-red-500' :
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_1d / selectedKey.rate_limit_1d) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- 7-Day Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit7d') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.rate_limit_7d"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_7d > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'text-red-500' :
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ${{ selectedKey.usage_7d?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ${{ selectedKey.rate_limit_7d?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'bg-red-500' :
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_7d / selectedKey.rate_limit_7d) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- Reset Rate Limit button (edit mode only) -->
            <div v-if="showEditModal && selectedKey && (selectedKey.rate_limit_5h > 0 || selectedKey.rate_limit_1d > 0 || selectedKey.rate_limit_7d > 0)">
              <button
                type="button"
                @click="confirmResetRateLimit"
                class="btn btn-secondary text-sm"
              >
                {{ t('keys.resetRateLimitUsage') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Expiration Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
            <button
              type="button"
              @click="formData.enable_expiration = !formData.enable_expiration"
              role="switch"
              :aria-checked="formData.enable_expiration"
              :aria-label="t('keys.expiration')"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_expiration ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_expiration ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_expiration" class="space-y-4 pt-2">
            <!-- Quick select buttons (for both create and edit mode) -->
            <div class="flex flex-wrap gap-2">
              <button
                v-for="days in ['7', '30', '90']"
                :key="days"
                type="button"
                @click="setExpirationDays(parseInt(days))"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm transition-colors',
                  formData.expiration_preset === days
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                ]"
              >
                {{ showEditModal ? t('keys.extendDays', { days }) : t('keys.expiresInDays', { days }) }}
              </button>
              <button
                type="button"
                @click="formData.expiration_preset = 'custom'"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm transition-colors',
                  formData.expiration_preset === 'custom'
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                ]"
              >
                {{ t('keys.customDate') }}
              </button>
            </div>

            <!-- Date picker (always show for precise adjustment) -->
            <div>
              <label class="input-label">{{ t('keys.expirationDate') }}</label>
              <input
                v-model="formData.expiration_date"
                type="datetime-local"
                class="input"
              />
              <p class="input-hint">{{ t('keys.expirationDateHint') }}</p>
            </div>

            <!-- Current expiration display (only in edit mode) -->
            <div v-if="showEditModal && selectedKey?.expires_at" class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('keys.currentExpiration') }}: </span>
              <span class="font-medium text-gray-900 dark:text-white">
                {{ formatDateTime(selectedKey.expires_at) }}
              </span>
            </div>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeModals" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            form="key-form"
            type="submit"
            :disabled="submitting"
            class="btn btn-primary"
            data-tour="key-form-submit"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{
              submitting
                ? t('keys.saving')
                : showEditModal
                  ? t('common.update')
                  : t('common.create')
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BulkEditKeysModal
      :show="showBulkEditModal"
      :selected-keys="selectedApiKeys"
      :groups="groups"
      @close="showBulkEditModal = false"
      @updated="handleBulkUpdated"
    />

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('keys.deleteKey')"
      :message="t('keys.deleteConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaDialog"
      :title="t('keys.resetQuotaTitle')"
      :message="t('keys.resetQuotaConfirmMessage', { name: selectedKey?.name, used: selectedKey?.quota_used?.toFixed(4) })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetQuotaUsed"
      @cancel="showResetQuotaDialog = false"
    />

    <!-- Reset Rate Limit Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetRateLimitDialog"
      :title="t('keys.resetRateLimitTitle')"
      :message="t('keys.resetRateLimitConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetRateLimitUsage"
      @cancel="showResetRateLimitDialog = false"
    />

    <!-- Use Key Modal -->
    <UseKeyModal
      :show="showUseKeyModal"
      :api-key="selectedKey?.key || ''"
      :base-url="publicSettings?.api_base_url || ''"
      :platform="selectedKey?.group?.platform || null"
      :allow-messages-dispatch="selectedKey?.group?.allow_messages_dispatch || false"
      @close="closeUseKeyModal"
    />

    <!-- CCS Client Selection Dialog for Antigravity -->
    <BaseDialog
      :show="showCcsClientSelect"
      :title="t('keys.ccsClientSelect.title')"
      width="narrow"
      @close="closeCcsClientSelect"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-400">
          {{ t('keys.ccsClientSelect.description') }}
	        </p>
	        <div class="grid grid-cols-2 gap-3">
	          <button
	            @click="handleCcsClientSelect('claude')"
	            class="flex flex-col items-center gap-2 p-4 rounded-xl border-2 border-gray-200 dark:border-dark-600 hover:border-primary-500 dark:hover:border-primary-500 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-all"
	          >
	            <Icon name="terminal" size="xl" class="text-gray-600 dark:text-gray-400" />
	            <span class="font-medium text-gray-900 dark:text-white">{{
	              t('keys.ccsClientSelect.claudeCode')
	            }}</span>
	            <span class="text-xs text-gray-500 dark:text-gray-400">{{
	              t('keys.ccsClientSelect.claudeCodeDesc')
	            }}</span>
	          </button>
	          <button
	            @click="handleCcsClientSelect('gemini')"
	            class="flex flex-col items-center gap-2 p-4 rounded-xl border-2 border-gray-200 dark:border-dark-600 hover:border-primary-500 dark:hover:border-primary-500 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-all"
	          >
	            <Icon name="sparkles" size="xl" class="text-gray-600 dark:text-gray-400" />
	            <span class="font-medium text-gray-900 dark:text-white">{{
	              t('keys.ccsClientSelect.geminiCli')
	            }}</span>
	            <span class="text-xs text-gray-500 dark:text-gray-400">{{
	              t('keys.ccsClientSelect.geminiCliDesc')
	            }}</span>
	          </button>
	        </div>
	      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeCcsClientSelect" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Group Selector Dropdown (Teleported to body to avoid overflow clipping) -->
    <Teleport to="body">
      <button
        v-if="showColumnDropdown && isMobileColumnSelector"
        type="button"
        data-test="column-selector-backdrop"
        class="fixed inset-0 z-[100000019] bg-black/35 backdrop-blur-[1px]"
        :aria-label="t('common.close')"
        @click="closeColumnSelector"
      />
      <section
        v-if="showColumnDropdown && isMobileColumnSelector"
        role="dialog"
        aria-modal="true"
        :aria-label="t('keys.columnSettings')"
        data-test="column-selector-sheet"
        class="animate-in fade-in slide-in-from-bottom-2 fixed inset-x-2 bottom-2 z-[100000020] max-h-[calc(100dvh-16px)] overflow-hidden rounded-2xl border border-white/70 bg-white/95 shadow-2xl shadow-slate-950/20 backdrop-blur-xl duration-200 dark:border-white/10 dark:bg-dark-800/95"
      >
        <header class="flex items-center justify-between border-b border-slate-100 px-4 py-3 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-slate-900 dark:text-white">
              {{ t('keys.columnSettings') }}
            </h2>
            <p class="mt-0.5 text-xs text-slate-500 dark:text-dark-400">
              {{ t('keys.columnSettingsHint') }}
            </p>
          </div>
          <button
            type="button"
            class="inline-flex h-9 w-9 items-center justify-center rounded-xl text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 focus:outline-none focus:ring-2 focus:ring-primary-500/40 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white"
            :aria-label="t('common.close')"
            @click="closeColumnSelector"
          >
            <Icon name="x" size="sm" />
          </button>
        </header>
        <div class="max-h-[min(62dvh,28rem)] overflow-y-auto p-2">
          <button
            v-for="col in toggleableColumns"
            :key="col.key"
            type="button"
            class="flex min-h-12 w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-left text-sm font-medium text-slate-700 transition-colors hover:bg-primary-50/70 dark:text-dark-200 dark:hover:bg-primary-500/10"
            @click="toggleColumn(col.key)"
          >
            <span class="min-w-0 pr-4">{{ col.label }}</span>
            <span
              class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-lg transition-colors"
              :class="isColumnVisible(col.key) ? 'bg-primary-500 text-white shadow-sm shadow-primary-500/25' : 'bg-slate-100 text-transparent dark:bg-dark-700'"
            >
              <Icon name="check" size="xs" :stroke-width="2.5" />
            </span>
          </button>
        </div>
      </section>

      <button
        v-if="groupSelectorKeyId !== null && dropdownPosition && isMobileGroupSelector"
        type="button"
        data-test="group-selector-backdrop"
        class="fixed inset-0 z-[100000019] bg-black/35 backdrop-blur-[1px]"
        :aria-label="t('common.close')"
        @click="closeGroupSelector"
      />
      <div
        v-if="groupSelectorKeyId !== null && dropdownPosition"
        ref="dropdownRef"
        role="dialog"
        :aria-modal="isMobileGroupSelector ? 'true' : undefined"
        :class="[
          'animate-in fade-in fixed z-[100000020] overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-black/5 duration-200 dark:bg-dark-800 dark:ring-white/10',
          isMobileGroupSelector
            ? 'inset-x-2 bottom-2 max-h-[calc(100dvh-16px)] w-auto slide-in-from-bottom-2'
            : 'w-[480px] slide-in-from-top-2'
        ]"
        style="pointer-events: auto !important;"
        :style="isMobileGroupSelector ? undefined : {
          top: dropdownPosition.top !== undefined ? dropdownPosition.top + 'px' : undefined,
          bottom: dropdownPosition.bottom !== undefined ? dropdownPosition.bottom + 'px' : undefined,
          left: dropdownPosition.left + 'px'
        }"
      >
        <!-- Search box -->
        <div class="border-b border-gray-100 p-2 dark:border-dark-700">
          <div class="relative">
            <svg class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              v-model="groupSearchQuery"
              type="text"
              class="w-full rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-sm text-gray-900 placeholder-gray-400 outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-300 dark:border-dark-600 dark:bg-dark-700 dark:text-white dark:placeholder-gray-500 dark:focus:border-primary-600 dark:focus:ring-primary-600"
              :placeholder="t('keys.searchGroup')"
              @click.stop
            />
          </div>
        </div>
        <!-- Group list -->
        <div :class="[isMobileGroupSelector ? 'max-h-[70dvh]' : 'max-h-80', 'space-y-2 overflow-y-auto bg-gray-50/80 p-2 dark:bg-black/10']">
          <button
            v-for="option in filteredGroupOptions"
            :key="option.value ?? 'null'"
            @click="changeGroup(selectedKeyForGroup!, option.value)"
            class="flex w-full items-center justify-between rounded-lg text-sm focus-visible:outline-none"
            :title="option.description || undefined"
          >
            <GroupOptionItem
              :tag="option.tag"
              :tag-color="option.tag_color"
              :name="option.label"
              :platform="option.platform"
              :subscription-type="option.subscriptionType"
              :rate-multiplier="option.rate"
              :user-rate-multiplier="option.userRate"
              :peak-rate-enabled="option.peakRateEnabled"
              :peak-start="option.peakStart"
              :peak-end="option.peakEnd"
              :peak-rate-multiplier="option.peakRateMultiplier"
              :description="option.description"
              :selected="
                selectedKeyForGroup?.group_id === option.value ||
                (!selectedKeyForGroup?.group_id && option.value === null)
              "
            />
          </button>
          <!-- Empty state when search has no results -->
          <div v-if="filteredGroupOptions.length === 0" class="py-4 text-center text-sm text-gray-400 dark:text-gray-500">
            {{ t('keys.noGroupFound') }}
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
	import { ref, reactive, computed, watch, onMounted, onUnmounted, nextTick, type ComponentPublicInstance } from 'vue'
	import { useI18n } from 'vue-i18n'
	import { useAppStore } from '@/stores/app'
	import { useOnboardingStore } from '@/stores/onboarding'
	import { useClipboard } from '@/composables/useClipboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
import { keysAPI, authAPI, usageAPI, userGroupsAPI } from '@/api'
import { customGroupsAPI } from '@/api/customGroups'
import AppLayout from '@/components/layout/AppLayout.vue'
import CustomGroupsManager from '@/components/custom-groups/CustomGroupsManager.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import BulkEditKeysModal from '@/components/keys/BulkEditKeysModal.vue'
	import DataTable from '@/components/common/DataTable.vue'
import KeyMobileCard from '@/components/keys/KeyMobileCard.vue'
	import Pagination from '@/components/common/Pagination.vue'
	import BaseDialog from '@/components/common/BaseDialog.vue'
	import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
	import EmptyState from '@/components/common/EmptyState.vue'
	import Select from '@/components/common/Select.vue'
	import SearchInput from '@/components/common/SearchInput.vue'
	import Icon from '@/components/icons/Icon.vue'
	import UseKeyModal from '@/components/keys/UseKeyModal.vue'
	import EndpointPopover from '@/components/keys/EndpointPopover.vue'
	import GroupBadge from '@/components/common/GroupBadge.vue'
	import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
	import type { ApiKey, Group, PublicSettings, SubscriptionType, GroupPlatform, UpdateApiKeyRequest, UserCustomGroup } from '@/types'
import type { Column } from '@/components/common/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import { formatDateTime } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformBadgeLightClass } from '@/utils/platformColors'
import { KEY_GROUP_PROVIDERS, KEY_GROUP_PROVIDER_ICONS, getKeyGroupProvider, type KeyGroupProvider } from '@/utils/keyGroupProviders'
import {
  buildCcSwitchImportDeeplink,
  type CcSwitchClientType
} from '@/utils/ccswitchImport'

// Helper to format date for datetime-local input
const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

interface GroupOption {
  tag?: Group['tag']
  tag_color?: string
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
}

const appStore = useAppStore()
const onboardingStore = useOnboardingStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

const allColumns = computed<Column[]>(() => [
  { key: 'name', label: t('common.name'), sortable: true },
  { key: 'id', label: t('keys.id'), sortable: true },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'group', label: t('keys.group'), sortable: false },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: true },
  { key: 'usage', label: t('keys.usage'), sortable: false },
  { key: 'rate_limit', label: t('keys.rateLimitColumn'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'last_used_at', label: t('keys.lastUsedAt'), sortable: true },
  { key: 'last_used_ip', label: t('keys.lastUsedIP'), sortable: false },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const ALWAYS_VISIBLE_COLUMNS = new Set(['name', 'actions'])
const DEFAULT_HIDDEN_COLUMNS = ['id', 'rate_limit', 'last_used_at', 'last_used_ip']
const HIDDEN_COLUMNS_KEY = 'api-key-hidden-columns'
const COLUMN_SETTINGS_VERSION_KEY = 'api-key-column-settings-version'
const COLUMN_SETTINGS_VERSION = 3
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['last_used_ip'],
  3: ['id']
}

const toggleableColumns = computed(() =>
  allColumns.value.filter((col) => !ALWAYS_VISIBLE_COLUMNS.has(col.key))
)

const hiddenColumns = reactive<Set<string>>(new Set())

const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
  } catch (error) {
    console.error('Failed to save API key table columns:', error)
  }
}

const loadSavedColumns = () => {
  hiddenColumns.clear()
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      const validColumnKeys = new Set(allColumns.value.map((col) => col.key))
      parsed
        .filter((key) =>
          typeof key === 'string' &&
          validColumnKeys.has(key) &&
          !ALWAYS_VISIBLE_COLUMNS.has(key)
        )
        .forEach((key) => hiddenColumns.add(key))
      const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (validColumnKeys.has(key) && !ALWAYS_VISIBLE_COLUMNS.has(key)) {
              hiddenColumns.add(key)
            }
          }
        }
        saveColumnsToStorage()
      } else {
        localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    }
  } catch (error) {
    console.error('Failed to load API key table columns:', error)
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
  }
}

const toggleColumn = (key: string) => {
  if (ALWAYS_VISIBLE_COLUMNS.has(key)) return
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const columns = computed<Column[]>(() =>
  allColumns.value.filter((col) => ALWAYS_VISIBLE_COLUMNS.has(col.key) || !hiddenColumns.has(col.key))
)

const apiKeys = ref<ApiKey[]>([])
const selectedIds = ref<number[]>([])
const showBulkEditModal = ref(false)
const selectedApiKeys = computed(() => apiKeys.value.filter((key) => selectedIds.value.includes(key.id)))
const allVisibleKeysSelected = computed(() => apiKeys.value.length > 0 && selectedApiKeys.value.length === apiKeys.value.length)

const handleSelectionChange = (ids: Array<string | number>) => {
  const visibleIds = new Set(apiKeys.value.map((key) => key.id))
  selectedIds.value = [...new Set(ids.map(Number))].filter((id) => visibleIds.has(id))
}

const handleBulkUpdated = (succeededIds: number[]) => {
  const succeeded = new Set(succeededIds)
  selectedIds.value = selectedIds.value.filter((id) => !succeeded.has(id))
  loadApiKeys()
}

const groups = ref<Group[]>([])
const customGroups = ref<UserCustomGroup[]>([])
const loading = ref(false)
const submitting = ref(false)
const now = ref(new Date())
let resetTimer: ReturnType<typeof setInterval> | null = null
const usageStats = ref<Record<string, BatchApiKeyUsageStats>>({})
const userGroupRates = ref<Record<number, number>>({})

const pagination = ref({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = ref({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// Filter state
const filterSearch = ref('')
const filterStatus = ref('')
const filterGroupId = ref<string | number>('')
const showMobileFilters = ref(false)
const showMobileEndpoints = ref(false)
const mobileEndpointsTriggerRef = ref<HTMLButtonElement | null>(null)
const mobileEndpointsPanelRef = ref<HTMLElement | null>(null)
const mobileEndpointsPosition = ref({ top: '0px', left: '0px', maxHeight: 'none' })
const showMobileTools = ref(false)
const mobileToolsTriggerRef = ref<HTMLButtonElement | null>(null)
const mobileToolsMenuRef = ref<HTMLElement | null>(null)
const mobileToolsPosition = ref({ top: '0px', left: '0px' })
const activeFilterCount = computed(() => Number(filterGroupId.value !== '') + Number(filterStatus.value !== ''))

const closeMobileEndpoints = () => {
  if (!showMobileEndpoints.value) return
  showMobileEndpoints.value = false
  if (mobileEndpointsPanelRef.value?.contains(document.activeElement)) {
    mobileEndpointsTriggerRef.value?.focus({ preventScroll: true })
  }
}

const positionMobileEndpoints = () => {
  const trigger = mobileEndpointsTriggerRef.value
  const panel = mobileEndpointsPanelRef.value
  if (!showMobileEndpoints.value || !trigger || !panel) return
  const anchor = trigger.getBoundingClientRect()
  if (anchor.bottom < 0 || anchor.top > window.innerHeight) {
    closeMobileEndpoints()
    return
  }
  const below = window.innerHeight - anchor.bottom - 14
  const above = anchor.top - 14
  const openBelow = panel.scrollHeight <= below || below >= above
  const maxHeight = Math.max(0, openBelow ? below : above)
  const height = Math.min(panel.scrollHeight + 2, maxHeight)
  mobileEndpointsPosition.value = {
    top: `${Math.max(8, openBelow ? anchor.bottom + 6 : anchor.top - height - 6)}px`,
    left: `${Math.max(8, Math.min(anchor.left, window.innerWidth - panel.offsetWidth - 8))}px`,
    maxHeight: `${maxHeight}px`
  }
}

const openMobileEndpoints = async () => {
  closeMobileTools()
  showMobileEndpoints.value = true
  await nextTick()
  positionMobileEndpoints()
  await nextTick()
  if (showMobileEndpoints.value) {
    const panel = mobileEndpointsPanelRef.value
    const focusTarget = panel?.querySelector<HTMLElement>('[role="button"], button, a[href]') ?? panel
    focusTarget?.focus({ preventScroll: true })
  }
}

const toggleMobileEndpoints = () => {
  if (showMobileEndpoints.value) closeMobileEndpoints()
  else void openMobileEndpoints()
}

const handleEndpointsTab = (event: KeyboardEvent) => {
  const items = mobileEndpointsPanelRef.value?.querySelectorAll<HTMLElement>('[role="button"], button, a[href]')
  if (!items?.length || document.activeElement === items[event.shiftKey ? 0 : items.length - 1]) {
    closeMobileEndpoints()
    if (event.shiftKey) event.preventDefault()
  }
}

const closeMobileTools = () => {
  if (!showMobileTools.value) return
  showMobileTools.value = false
  if (mobileToolsMenuRef.value?.contains(document.activeElement)) {
    mobileToolsTriggerRef.value?.focus({ preventScroll: true })
  }
}

const positionMobileTools = () => {
  const trigger = mobileToolsTriggerRef.value
  const menu = mobileToolsMenuRef.value
  if (!showMobileTools.value || !trigger || !menu) return
  const anchor = trigger.getBoundingClientRect()
  if (anchor.bottom < 0 || anchor.top > window.innerHeight) {
    closeMobileTools()
    return
  }
  const top = anchor.bottom + 6 + menu.offsetHeight <= window.innerHeight - 8
    ? anchor.bottom + 6
    : anchor.top - menu.offsetHeight - 6
  mobileToolsPosition.value = {
    top: `${Math.max(8, top)}px`,
    left: `${Math.max(8, Math.min(anchor.right - menu.offsetWidth, window.innerWidth - menu.offsetWidth - 8))}px`
  }
}

const openMobileTools = async () => {
  closeMobileEndpoints()
  showMobileTools.value = true
  await nextTick()
  positionMobileTools()
  await nextTick()
  if (showMobileTools.value) {
    mobileToolsMenuRef.value?.querySelector<HTMLButtonElement>('[role="menuitem"]')?.focus({ preventScroll: true })
  }
}

const toggleMobileTools = () => {
  if (showMobileTools.value) closeMobileTools()
  else void openMobileTools()
}

const clearMobileFilters = () => {
  filterGroupId.value = ''
  filterStatus.value = ''
  onFilterChange()
}

const runMobileTool = async (action: () => void) => {
  closeMobileTools()
  await nextTick()
  action()
}

const showCreateModal = ref(false)
const showCustomGroupsModal = ref(false)
const showEditModal = ref(false)
const showDeleteDialog = ref(false)
const showResetQuotaDialog = ref(false)
const showResetRateLimitDialog = ref(false)
const showUseKeyModal = ref(false)
const showCcsClientSelect = ref(false)
const showColumnDropdown = ref(false)
const isMobileColumnSelector = ref(false)
const pendingCcsRow = ref<ApiKey | null>(null)
const selectedKey = ref<ApiKey | null>(null)
const copiedKeyId = ref<number | null>(null)
const groupSelectorKeyId = ref<number | null>(null)
const isMobileGroupSelector = ref(false)
const publicSettings = ref<PublicSettings | null>(null)
const endpointCount = computed(() => Number(Boolean(publicSettings.value?.api_base_url)) + (publicSettings.value?.custom_endpoints?.length ?? 0))
const dropdownRef = ref<HTMLElement | null>(null)
const columnDropdownRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<{ top?: number; bottom?: number; left: number } | null>(null)
const dropdownViewportPadding = 8
const groupButtonRefs = ref<Map<number, HTMLElement>>(new Map())
let abortController: AbortController | null = null

// Get the currently selected key for group change
const selectedKeyForGroup = computed(() => {
  if (groupSelectorKeyId.value === null) return null
  return apiKeys.value.find((k) => k.id === groupSelectorKeyId.value) || null
})

const setGroupButtonRef = (keyId: number, el: Element | ComponentPublicInstance | null) => {
  if (el instanceof HTMLElement) {
    groupButtonRefs.value.set(keyId, el)
  } else {
    groupButtonRefs.value.delete(keyId)
  }
}

const formData = ref({
  name: '',
  group_id: null as number | null,
  custom_group_id: null as number | null,
  status: 'active' as 'active' | 'inactive',
  use_custom_key: false,
  custom_key: '',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  // Quota settings (empty = unlimited)
  enable_quota: false,
  quota: null as number | null,
  // Rate limit settings
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_preset: '30' as '7' | '30' | '90' | 'custom',
  expiration_date: ''
})

// 自定义Key验证
const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  // 检查字符：只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

const shouldSubmitEditStatus = (key: ApiKey, status: 'active' | 'inactive') => {
  if (key.status === 'quota_exhausted' || key.status === 'expired') {
    return status === 'active'
  }
  return true
}

// Filter dropdown options
const groupFilterOptions = computed(() => [
  { value: '', label: t('keys.allGroups') },
  { value: 0, label: t('keys.noGroup') },
  ...groups.value.map((g) => ({ value: g.id, label: g.name, platform: g.platform }))
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('keys.allStatus') },
  { value: 'active', label: t('keys.status.active') },
  { value: 'inactive', label: t('keys.status.inactive') },
  { value: 'quota_exhausted', label: t('keys.status.quota_exhausted') },
  { value: 'expired', label: t('keys.status.expired') }
])

const onFilterChange = () => {
  selectedIds.value = []
  pagination.value.page = 1
  loadApiKeys()
}

const onGroupFilterChange = (value: string | number | boolean | null) => {
  filterGroupId.value = value as string | number
  onFilterChange()
}

const onStatusFilterChange = (value: string | number | boolean | null) => {
  filterStatus.value = value as string
  onFilterChange()
}

// Convert groups to Select options format with rate multiplier and subscription type
const groupOptions = computed(() =>
  groups.value.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    tag: group.tag,
    tag_color: group.tag_color,
    rate: group.rate_multiplier,
    userRate: userGroupRates.value[group.id] ?? null,
    peakRateEnabled: group.peak_rate_enabled,
    peakStart: group.peak_start,
    peakEnd: group.peak_end,
    peakRateMultiplier: group.peak_rate_multiplier,
    subscriptionType: group.subscription_type,
    platform: group.platform
  }))
)

const customGroupOptions = computed(() => [
  { value: null, label: '不使用自定义分组' },
  ...customGroups.value.map((group) => ({
    value: group.id,
    label: `${group.name} · ${group.models.length} 个模型`
  }))
])

const createProvider = ref<KeyGroupProvider>('anthropic')
const createProviderOptions = computed(() => KEY_GROUP_PROVIDERS.map((value) => ({
  value,
  label: t(`keys.providers.${value}`),
  count: groups.value.filter((group) => getKeyGroupProvider(group.platform) === value).length
})))

const formGroupOptions = computed(() => showEditModal.value
  ? groupOptions.value
  : groupOptions.value.filter((group) => getKeyGroupProvider(group.platform) === createProvider.value)
)

const selectCreateProvider = (provider: KeyGroupProvider) => {
  if (createProvider.value === provider) return
  createProvider.value = provider
  formData.value.group_id = null
}

// Also handles groups arriving after the create dialog has already opened.
watch([showCreateModal, createProviderOptions], ([isOpen, providers], [wasOpen]) => {
  if (!isOpen) return
  if (!wasOpen || !providers.some((provider) => provider.value === createProvider.value && provider.count > 0)) {
    selectCreateProvider(providers.find((provider) => provider.count > 0)?.value ?? 'anthropic')
  }
  if (!formGroupOptions.value.some((group) => group.value === formData.value.group_id)) {
    formData.value.group_id = null
  }
})

// Group dropdown search
const groupSearchQuery = ref('')
const filteredGroupOptions = computed(() => {
  const query = groupSearchQuery.value.trim().toLowerCase()
  if (!query) return groupOptions.value
  return groupOptions.value.filter((opt) => {
    return opt.label.toLowerCase().includes(query) ||
      (opt.description && opt.description.toLowerCase().includes(query))
  })
})

const copyToClipboard = async (text: string, keyId: number) => {
  const success = await clipboardCopy(text, t('keys.copied'))
  if (success) {
    copiedKeyId.value = keyId
    setTimeout(() => {
      copiedKeyId.value = null
    }, 800)
  }
}

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const { name, code } = error as { name?: string; code?: string }
  return name === 'AbortError' || code === 'ERR_CANCELED'
}

const loadApiKeys = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  const { signal } = controller
  loading.value = true
  try {
    // Build filters
    const filters: {
      search?: string
      status?: string
      group_id?: number | string
      sort_by?: string
      sort_order?: 'asc' | 'desc'
    } = {}
    if (filterSearch.value) filters.search = filterSearch.value
    if (filterStatus.value) filters.status = filterStatus.value
    if (filterGroupId.value !== '') filters.group_id = filterGroupId.value
    filters.sort_by = sortState.value.sort_by
    filters.sort_order = sortState.value.sort_order

    const response = await keysAPI.list(pagination.value.page, pagination.value.page_size, filters, {
      signal
    })
    if (signal.aborted) return
    apiKeys.value = response.items
    handleSelectionChange(selectedIds.value)
    pagination.value.total = response.total
    pagination.value.pages = response.pages

    // Load usage stats for all API keys in the list
    if (response.items.length > 0) {
      const keyIds = response.items.map((k) => k.id)
      try {
        const usageResponse = await usageAPI.getDashboardApiKeysUsage(keyIds, { signal })
        if (signal.aborted) return
        usageStats.value = usageResponse.stats
      } catch (e) {
        if (!isAbortError(e)) {
          console.error('Failed to load usage stats:', e)
        }
      }
    }
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('keys.failedToLoad'))
  } finally {
    if (abortController === controller) {
      loading.value = false
    }
  }
}

const loadGroups = async () => {
  try {
    groups.value = await userGroupsAPI.getAvailable()
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

const loadCustomGroups = async () => {
  try {
    customGroups.value = (await customGroupsAPI.list()).filter(group => group.status === 'active')
  } catch {
    customGroups.value = []
  }
}

const loadUserGroupRates = async () => {
  try {
    userGroupRates.value = await userGroupsAPI.getUserGroupRates()
  } catch (error) {
    console.error('Failed to load user group rates:', error)
  }
}

const loadPublicSettings = async () => {
  try {
    publicSettings.value = await authAPI.getPublicSettings()
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
}

const openUseKeyModal = (key: ApiKey) => {
  selectedKey.value = key
  showUseKeyModal.value = true
}

const closeUseKeyModal = () => {
  showUseKeyModal.value = false
  selectedKey.value = null
}

const handlePageChange = (page: number) => {
  selectedIds.value = []
  pagination.value.page = page
  loadApiKeys()
}

const handlePageSizeChange = (pageSize: number) => {
  selectedIds.value = []
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadApiKeys()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  selectedIds.value = []
  sortState.value.sort_by = key
  sortState.value.sort_order = order
  pagination.value.page = 1
  loadApiKeys()
}

const editKey = (key: ApiKey) => {
  selectedKey.value = key
  const hasIPRestriction = (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)
  const hasExpiration = !!key.expires_at
  formData.value = {
    name: key.name,
    group_id: key.group_id,
    custom_group_id: key.custom_group_id,
    status: key.status === 'quota_exhausted' || key.status === 'expired' ? 'inactive' : key.status,
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: hasIPRestriction,
    ip_whitelist: (key.ip_whitelist || []).join('\n'),
    ip_blacklist: (key.ip_blacklist || []).join('\n'),
    enable_quota: key.quota > 0,
    quota: key.quota > 0 ? key.quota : null,
    enable_rate_limit: (key.rate_limit_5h > 0) || (key.rate_limit_1d > 0) || (key.rate_limit_7d > 0),
    rate_limit_5h: key.rate_limit_5h || null,
    rate_limit_1d: key.rate_limit_1d || null,
    rate_limit_7d: key.rate_limit_7d || null,
    enable_expiration: hasExpiration,
    expiration_preset: 'custom',
    expiration_date: key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  }
  showEditModal.value = true
}

const toggleKeyStatus = async (key: ApiKey) => {
  const newStatus = key.status === 'active' ? 'inactive' : 'active'
  try {
    await keysAPI.toggleStatus(key.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('keys.keyEnabledSuccess') : t('keys.keyDisabledSuccess')
    )
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToUpdateStatus'))
  }
}

const openGroupSelector = (key: ApiKey) => {
  if (groupSelectorKeyId.value === key.id) {
    closeGroupSelector()
  } else {
    isMobileGroupSelector.value = window.innerWidth < 768
    const buttonEl = groupButtonRefs.value.get(key.id)
    if (buttonEl) {
      const rect = buttonEl.getBoundingClientRect()
      const dropdownEstHeight = 400 // estimated max dropdown height
      const dropdownEstWidth = Math.min(480, window.innerWidth - dropdownViewportPadding * 2)
      const spaceBelow = window.innerHeight - rect.bottom
      const spaceAbove = rect.top
      // 夹取 left，避免窄屏下浮层超出视口右缘
      const left = Math.max(
        dropdownViewportPadding,
        Math.min(rect.left, window.innerWidth - dropdownEstWidth - dropdownViewportPadding)
      )

      if (spaceBelow < dropdownEstHeight && spaceAbove > spaceBelow) {
        // Not enough space below, pop upward
        dropdownPosition.value = {
          bottom: window.innerHeight - rect.top + 4,
          left
        }
      } else {
        // Default: pop downward
        dropdownPosition.value = {
          top: rect.bottom + 4,
          left
        }
      }
    }
    groupSelectorKeyId.value = key.id
    groupSearchQuery.value = ''
  }
}

const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  groupSelectorKeyId.value = null
  dropdownPosition.value = null
  if (key.group_id === newGroupId) return

  try {
    await keysAPI.update(key.id, { group_id: newGroupId })
    appStore.showSuccess(t('keys.groupChangedSuccess'))
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToChangeGroup'))
  }
}

const closeGroupSelector = () => {
  groupSelectorKeyId.value = null
  dropdownPosition.value = null
  isMobileGroupSelector.value = false
}

const closeColumnSelector = () => {
  showColumnDropdown.value = false
  isMobileColumnSelector.value = false
}

const toggleColumnSelector = () => {
  if (showColumnDropdown.value) {
    closeColumnSelector()
    return
  }
  isMobileColumnSelector.value = window.innerWidth < 640
  showColumnDropdown.value = true
}

const handleDocumentClick = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!mobileEndpointsTriggerRef.value?.contains(target) && !mobileEndpointsPanelRef.value?.contains(target)) {
    closeMobileEndpoints()
  }
  if (!mobileToolsTriggerRef.value?.contains(target) && !mobileToolsMenuRef.value?.contains(target)) {
    closeMobileTools()
  }
  // Check if click is inside the dropdown or the trigger button
  if (!target.closest('.group\\/dropdown') && !dropdownRef.value?.contains(target)) {
    closeGroupSelector()
  }
  if (
    columnDropdownRef.value &&
    !columnDropdownRef.value.contains(target) &&
    !target.closest('[data-test="column-selector-sheet"]')
  ) {
    closeColumnSelector()
  }
}

const confirmDelete = (key: ApiKey) => {
  selectedKey.value = key
  showDeleteDialog.value = true
}

const handleSubmit = async () => {
  if (formData.value.group_id === null && formData.value.custom_group_id === null) {
    appStore.showError(t('keys.groupRequired'))
    return
  }

  // Validate custom key if enabled
  if (!showEditModal.value && formData.value.use_custom_key) {
    if (!formData.value.custom_key) {
      appStore.showError(t('keys.customKeyRequired'))
      return
    }
    if (customKeyError.value) {
      appStore.showError(customKeyError.value)
      return
    }
  }

  // Parse IP lists only if IP restriction is enabled
  const parseIPList = (text: string): string[] =>
    text.split('\n').map(ip => ip.trim()).filter(ip => ip.length > 0)
  const ipWhitelist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_whitelist) : []
  const ipBlacklist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_blacklist) : []

  // Calculate quota value (null/empty/0 = unlimited, stored as 0)
  const quota = formData.value.quota && formData.value.quota > 0 ? formData.value.quota : 0

  // Calculate expiration
  let expiresInDays: number | undefined
  let expiresAt: string | null | undefined
  if (formData.value.enable_expiration && formData.value.expiration_date) {
    if (!showEditModal.value) {
      // Create mode: calculate days from date
      const expDate = new Date(formData.value.expiration_date)
      const now = new Date()
      const diffDays = Math.ceil((expDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
      expiresInDays = diffDays > 0 ? diffDays : 1
    } else {
      // Edit mode: use custom date directly
      expiresAt = new Date(formData.value.expiration_date).toISOString()
    }
  } else if (showEditModal.value) {
    // Edit mode: if expiration disabled or date cleared, send empty string to clear
    expiresAt = ''
  }

  // Calculate rate limit values (send 0 when toggle is off)
  const rateLimitData = formData.value.enable_rate_limit ? {
    rate_limit_5h: formData.value.rate_limit_5h && formData.value.rate_limit_5h > 0 ? formData.value.rate_limit_5h : 0,
    rate_limit_1d: formData.value.rate_limit_1d && formData.value.rate_limit_1d > 0 ? formData.value.rate_limit_1d : 0,
    rate_limit_7d: formData.value.rate_limit_7d && formData.value.rate_limit_7d > 0 ? formData.value.rate_limit_7d : 0,
  } : { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }

  submitting.value = true
  try {
    if (showEditModal.value && selectedKey.value) {
      const updates: UpdateApiKeyRequest = {
        name: formData.value.name,
        group_id: formData.value.group_id,
        custom_group_id: formData.value.custom_group_id,
        ip_whitelist: ipWhitelist,
        ip_blacklist: ipBlacklist,
        quota: quota,
        expires_at: expiresAt,
        rate_limit_5h: rateLimitData.rate_limit_5h,
        rate_limit_1d: rateLimitData.rate_limit_1d,
        rate_limit_7d: rateLimitData.rate_limit_7d,
      }
      if (shouldSubmitEditStatus(selectedKey.value, formData.value.status)) {
        updates.status = formData.value.status
      }
      await keysAPI.update(selectedKey.value.id, updates)
      appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    } else {
      const customKey = formData.value.use_custom_key ? formData.value.custom_key : undefined
      await keysAPI.create(
        formData.value.name,
        formData.value.group_id,
        customKey,
        ipWhitelist,
        ipBlacklist,
        quota,
        expiresInDays,
        rateLimitData
        ,formData.value.custom_group_id
      )
      appStore.showSuccess(t('keys.keyCreatedSuccess'))
      // Only advance tour if active, on submit step, and creation succeeded
      if (onboardingStore.isCurrentStep('[data-tour="key-form-submit"]')) {
        onboardingStore.nextStep(500)
      }
    }
    closeModals()
    loadApiKeys()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToSave')
    appStore.showError(errorMsg)
    // Don't advance tour on error
  } finally {
    submitting.value = false
  }
}

/**
 * 处理删除 API Key 的操作
 * 优化：错误处理改进，优先显示后端返回的具体错误消息（如权限不足等），
 * 若后端未返回消息则显示默认的国际化文本
 */
const handleDelete = async () => {
  if (!selectedKey.value) return

  try {
    await keysAPI.delete(selectedKey.value.id)
    appStore.showSuccess(t('keys.keyDeletedSuccess'))
    showDeleteDialog.value = false
    loadApiKeys()
  } catch (error: any) {
    // 优先使用后端返回的错误消息，提供更具体的错误信息给用户
    const errorMsg = error?.message || t('keys.failedToDelete')
    appStore.showError(errorMsg)
  }
}

const closeModals = () => {
  showCreateModal.value = false
  showEditModal.value = false
  selectedKey.value = null
  formData.value = {
    name: '',
    group_id: null,
    custom_group_id: null,
    status: 'active',
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: false,
    ip_whitelist: '',
    ip_blacklist: '',
    enable_quota: false,
    quota: null,
    enable_rate_limit: false,
    rate_limit_5h: null,
    rate_limit_1d: null,
    rate_limit_7d: null,
    enable_expiration: false,
    expiration_preset: '30',
    expiration_date: ''
  }
}

// Show reset quota confirmation dialog
const confirmResetQuota = () => {
  showResetQuotaDialog.value = true
}

// Set expiration date based on quick select days
const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const expDate = new Date()
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

// Reset quota used for an API key
const resetQuotaUsed = async () => {
  const key = selectedKey.value
  if (!key) return
  showResetQuotaDialog.value = false
  try {
    const updatedKey = await keysAPI.update(key.id, { reset_quota: true })
    appStore.showSuccess(t('keys.quotaResetSuccess'))
    key.quota_used = updatedKey.quota_used
    if (key.status !== updatedKey.status) {
      key.status = updatedKey.status
      if (selectedKey.value?.id === key.id) {
        formData.value.status = updatedKey.status === 'active' ? 'active' : 'inactive'
      }
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetQuota')
    appStore.showError(errorMsg)
  }
}

// Show reset rate limit confirmation dialog (from edit modal)
const confirmResetRateLimit = () => {
  showResetRateLimitDialog.value = true
}

// Show reset rate limit confirmation dialog (from table row)
const confirmResetRateLimitFromTable = (row: ApiKey) => {
  selectedKey.value = row
  showResetRateLimitDialog.value = true
}

// Reset rate limit usage for an API key
const resetRateLimitUsage = async () => {
  if (!selectedKey.value) return
  showResetRateLimitDialog.value = false
  try {
    await keysAPI.update(selectedKey.value.id, { reset_rate_limit_usage: true })
    appStore.showSuccess(t('keys.rateLimitResetSuccess'))
    // Refresh key data
    await loadApiKeys()
    // Update the editing key with fresh data
    const refreshedKey = apiKeys.value.find(k => k.id === selectedKey.value!.id)
    if (refreshedKey) {
      selectedKey.value = refreshedKey
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
    appStore.showError(errorMsg)
  }
}

const importToCcswitch = (row: ApiKey) => {
  const platform = row.group?.platform || 'anthropic'

  // For antigravity platform, show client selection dialog
  if (platform === 'antigravity') {
    pendingCcsRow.value = row
    showCcsClientSelect.value = true
    return
  }

  // For other platforms, execute directly
  executeCcsImport(row, platform === 'gemini' ? 'gemini' : 'claude')
}

const executeCcsImport = (row: ApiKey, clientType: CcSwitchClientType) => {
  const baseUrl = publicSettings.value?.api_base_url || window.location.origin
  const platform = row.group?.platform || 'anthropic'

  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`
  const providerName = (publicSettings.value?.site_name || 'sub2api').trim() || 'sub2api'
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl,
    platform,
    clientType,
    providerName,
    apiKey: row.key,
    usageScript
  })

  try {
    window.open(deeplink, '_self')

    // Check if the protocol handler worked by detecting if we're still focused
    setTimeout(() => {
      if (document.hasFocus()) {
        // Still focused means the protocol handler likely failed
        appStore.showError(t('keys.ccSwitchNotInstalled'))
      }
    }, 100)
  } catch (error) {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  }
}

const handleCcsClientSelect = (clientType: CcSwitchClientType) => {
  if (pendingCcsRow.value) {
    executeCcsImport(pendingCcsRow.value, clientType)
  }
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

const closeCcsClientSelect = () => {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

function formatResetTime(resetAt: string | null): string {
  if (!resetAt) return ''
  const diff = new Date(resetAt).getTime() - now.value.getTime()
  if (diff <= 0) return t('keys.resetNow')
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.floor((diff % 3600000) / 60000)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

onMounted(() => {
  loadSavedColumns()
  loadApiKeys()
  loadGroups()
  loadCustomGroups()
  loadUserGroupRates()
  loadPublicSettings()
  document.addEventListener('click', handleDocumentClick)
  window.addEventListener('resize', closeGroupSelector)
  window.addEventListener('resize', closeColumnSelector)
  window.addEventListener('resize', closeMobileTools)
  window.addEventListener('resize', closeMobileEndpoints)
  window.addEventListener('scroll', positionMobileTools, true)
  window.addEventListener('scroll', positionMobileEndpoints, true)
  resetTimer = setInterval(() => { now.value = new Date() }, 60000)
})

onUnmounted(() => {
  document.removeEventListener('click', handleDocumentClick)
  window.removeEventListener('resize', closeGroupSelector)
  window.removeEventListener('resize', closeColumnSelector)
  window.removeEventListener('resize', closeMobileTools)
  window.removeEventListener('resize', closeMobileEndpoints)
  window.removeEventListener('scroll', positionMobileTools, true)
  window.removeEventListener('scroll', positionMobileEndpoints, true)
  if (resetTimer) clearInterval(resetTimer)
})
</script>

<style scoped>
.keys-groups-content {
  display: flex;
  min-height: 0;
  max-height: min(72dvh, 42rem);
  flex-direction: column;
}
.keys-provider-option {
  min-height: 2.75rem;
  padding-right: 1.625rem;
  background: var(--dialog-control);
  box-shadow: inset 0 1px 0 var(--dialog-highlight);
}
.keys-provider-icons > span + span { margin-left: -0.5rem; }
.keys-provider-label { min-width: 0; overflow-wrap: anywhere; }
.keys-provider-options input:checked + .keys-provider-option {
  background: rgb(var(--brand-rgb) / 5%);
  box-shadow: inset 0 1px 0 var(--dialog-highlight);
}
.keys-key-form > .space-y-3 { margin-top: 0.75rem; }
.keys-key-form > .space-y-3 > .flex:first-child { min-height: 2rem; }
.keys-key-form [role="switch"] { position: relative; }
.keys-key-form [role="switch"]::before { content: ''; position: absolute; inset: -0.625rem -0.25rem; }
.keys-key-form [role="switch"]:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 50%); outline-offset: 3px; }
.keys-workspace {
  --keys-rule: rgb(148 163 184 / 24%);
  --keys-control: rgb(255 255 255 / 92%);
  gap: 1rem;
}

.keys-workspace :deep(.admin-toolbar-surface) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.keys-toolbar { gap: 1rem; }
.keys-workspace :deep(.admin-pagination-surface) {
  padding: 0.25rem 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
}
.keys-toolbar-main { gap: 0.625rem; }
.keys-filter-controls { gap: 0.625rem; }
.keys-mobile-utilities { display: none; }

.keys-mobile-tools {
  position: fixed;
  z-index: 50;
  width: 11.5rem;
  max-width: calc(100vw - 1rem);
  padding: 0.375rem;
  border: 1px solid rgb(100 116 139 / 16%);
  border-radius: 8px;
  background: rgb(255 255 255 / 94%);
  box-shadow: 0 12px 32px rgb(15 23 42 / 10%), 0 2px 6px rgb(15 23 42 / 4%), inset 0 1px 0 rgb(255 255 255 / 90%);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  transform-origin: top right;
}
.dark .keys-mobile-tools {
  border-color: rgb(255 255 255 / 12%);
  background: rgb(28 31 36 / 96%);
  box-shadow: 0 12px 32px rgb(0 0 0 / 24%), inset 0 1px 0 rgb(255 255 255 / 4%);
}
.keys-mobile-endpoints {
  width: 22rem;
  padding: 0.25rem;
  overflow-y: auto;
  overscroll-behavior: contain;
  transform-origin: top left;
}
.keys-mobile-tools > button {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  width: 100%;
  min-height: 2.75rem;
  padding: 0.625rem;
  border-radius: 6px;
  color: #374151;
  text-align: left;
  font-size: 0.875rem;
  transition: background-color 140ms ease;
}
.keys-mobile-tools > button > svg { flex-shrink: 0; color: #7b8492; }
.keys-mobile-tools > button:hover { background: rgb(148 163 184 / 10%); }
.keys-mobile-tools > button:active { background: rgb(148 163 184 / 16%); }
.dark .keys-mobile-tools > button { color: #e2e8f0; }
.keys-mobile-tools > button:focus-visible {
  outline: 2px solid rgb(var(--brand-rgb) / 0.5);
  outline-offset: -2px;
  background: rgb(148 163 184 / 10%);
}
.keys-menu-enter-active { transition: opacity 160ms ease-out, transform 160ms ease-out; }
.keys-menu-leave-active { transition: opacity 110ms ease-in, transform 110ms ease-in; pointer-events: none; }
.keys-menu-enter-from,
.keys-menu-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }
@media (prefers-reduced-motion: reduce) {
  .keys-menu-enter-active,
  .keys-menu-leave-active { transition: none; }
  .keys-menu-enter-from,
  .keys-menu-leave-to { transform: none; }
}

.keys-filter-controls :deep(.input),
.keys-filter-controls :deep(.select-trigger),
.keys-toolbar-actions .btn {
  min-height: 2.75rem;
  border: 1px solid var(--keys-rule);
  border-radius: 8px;
  background: linear-gradient(135deg, rgb(255 255 255 / 70%), transparent), var(--keys-control);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 4px rgb(15 23 42 / 3%);
  transition: border-color 180ms ease, background-color 180ms ease, box-shadow 180ms ease;
}

.keys-filter-controls :deep(.input:focus),
.keys-filter-controls :deep(.select-trigger-open) {
  border-color: rgb(var(--brand-rgb) / 0.5);
  box-shadow: inset 0 1px 0 #fff, 0 0 0 3px rgb(var(--brand-rgb) / 0.08);
}

.keys-toolbar-actions .keys-mobile-secondary-action {
  color: #334155;
  background-color: rgb(248 250 252 / 95%);
}

.keys-custom-group-icon {
  --group-node-primary: #2563eb;
  --group-node-secondary: #0891b2;
  color: #38bdf8;
}

.keys-custom-group-icon :deep(circle) {
  fill: var(--group-node-secondary);
  stroke: none;
}

.keys-custom-group-icon :deep(circle:first-of-type) {
  fill: var(--group-node-primary);
}

.dark .keys-custom-group-icon {
  --group-node-primary: #60a5fa;
  --group-node-secondary: #22d3ee;
}

.keys-toolbar-actions .btn-primary {
  border-color: rgb(var(--brand-rgb) / 0.3);
  color: #fff;
  background: linear-gradient(135deg, rgb(255 255 255 / 14%), transparent), rgb(var(--brand-rgb));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 25%), 0 2px 5px rgb(var(--brand-rgb) / 0.16);
}

.keys-toolbar-actions .btn:focus-visible {
  outline: 2px solid rgb(var(--brand-rgb) / 0.5);
  outline-offset: 2px;
}

.dark .keys-workspace {
  --keys-rule: rgb(255 255 255 / 12%);
  --keys-control: #27292e;
}

.dark .keys-filter-controls :deep(.input),
.dark .keys-filter-controls :deep(.select-trigger),
.dark .keys-toolbar-actions .btn-secondary {
  background: linear-gradient(135deg, rgb(255 255 255 / 4%), transparent), var(--keys-control);
  color: #e2e8f0;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 6%), 0 2px 4px rgb(0 0 0 / 10%);
}

@media (hover: hover) {
  .keys-toolbar-actions .btn:hover:not(:disabled),
  .keys-filter-controls :deep(.select-trigger:hover) { border-color: rgb(var(--brand-rgb) / 0.4); }
}

@media (prefers-reduced-motion: reduce) {
  .keys-filter-controls :deep(.input),
  .keys-filter-controls :deep(.select-trigger),
  .keys-toolbar-actions .btn { transition: none; }
}

.keys-table {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex: 1;
  flex-direction: column;
}

.keys-mobile-selection-actions {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 0.5rem;
  font-size: 0.75rem;
  white-space: nowrap;
}

.keys-mobile-selection-actions .btn {
  flex-shrink: 0;
  min-height: 2.5rem;
  padding: 0.375rem 0.5rem;
  font-size: 0.75rem;
}

.keys-table :deep(.table-mobile-selection > label) {
  min-height: 2.5rem;
  gap: 0.375rem;
  font-size: 0.75rem;
  white-space: nowrap;
}

@media (max-width: 1023px) {
  .keys-table {
    flex: none;
  }
}

@media (max-width: 639px) {
  .keys-toolbar { gap: 0; }

  .keys-table :deep(.table-mobile-selection) { display: none; }
  .keys-table.has-selection :deep(.table-mobile-selection) { display: flex; }
  .keys-table :deep(.table-mobile-selection > label) { display: none; }
  .keys-table:not(.has-selection) :deep(.table-mobile-selection + [data-mobile-table-row]) { margin-top: 0; }

  .keys-toolbar-main {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 2.75rem auto;
    align-items: stretch;
    gap: 0.375rem 0.5rem;
  }

  .keys-filter-controls,
  .keys-toolbar-actions { display: contents; }

  .keys-desktop-action,
  .keys-inline-endpoints { display: none; }

  .keys-search {
    grid-column: 1;
    grid-row: 1;
    min-width: 0;
  }

  .keys-toolbar-actions [data-test="keys-create-entry"] {
    grid-column: 3;
    grid-row: 1;
    padding-inline: 0.75rem;
  }

  .keys-refresh-entry {
    grid-column: 2;
    grid-row: 1;
  }

  .keys-filter-selects { display: none; }
  .keys-filter-selects.is-open {
    display: grid;
    grid-column: 1 / -1;
    grid-row: 3;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.5rem;
    padding: 0.25rem 0 0.5rem;
  }

  .keys-filter-selects > * {
    width: 100% !important;
    min-width: 0;
  }

  .keys-clear-filters {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.25rem;
    min-height: 2.75rem;
    color: #677282;
    font-size: 0.75rem;
  }

  .keys-mobile-utilities {
    grid-column: 1 / -1;
    grid-row: 2;
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }

  .keys-toolbar-selection {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.375rem;
    min-width: 2.75rem;
    min-height: 2.75rem;
    margin-left: auto;
    color: #677282;
    font-size: 0.75rem;
    white-space: nowrap;
    cursor: pointer;
  }

  .keys-mobile-utilities > button {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.375rem;
    min-width: 2.75rem;
    min-height: 2.75rem;
    border-radius: 6px;
    color: #677282;
    font-size: 0.75rem;
    white-space: nowrap;
    transition: color 160ms ease;
  }

  .keys-mobile-utilities > button svg { flex-shrink: 0; }
  .keys-mobile-utilities > .keys-custom-groups-entry {
    justify-content: flex-start;
    margin-left: auto;
    color: #334155;
    font-weight: 600;
  }
  .keys-utility-label { display: none; }
  .dark .keys-mobile-utilities > .keys-custom-groups-entry { color: #e2e8f0; }
  .keys-mobile-utilities > .keys-more-toggle { width: 2.75rem; flex-shrink: 0; }
  .keys-mobile-utilities > .keys-more-toggle[aria-expanded="true"] { background: rgb(148 163 184 / 12%); }
  .dark .keys-mobile-utilities > button,
  .dark .keys-toolbar-selection,
  .dark .keys-clear-filters { color: #a0a9b7; }
  .keys-mobile-utilities > button:hover,
  .keys-mobile-utilities > button.is-active { color: rgb(var(--brand-rgb)); }
  .keys-mobile-utilities > button:focus-visible,
  .keys-clear-filters:focus-visible {
    outline: 2px solid rgb(var(--brand-rgb) / 0.5);
    outline-offset: 2px;
  }

  .keys-filter-count {
    position: absolute;
    right: 0;
    top: 0.125rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1rem;
    height: 1rem;
    border-radius: 4px;
    background: rgb(var(--brand-rgb) / 0.1);
    font-size: 0.625rem;
    font-weight: 600;
  }
}

@media (min-width: 360px) and (max-width: 639px) {
  .keys-utility-label { display: inline; }
  .keys-filter-count { right: -0.25rem; }
}
</style>
