<template>
  <AppLayout>
    <div class="admin-lottery-page mx-auto w-full max-w-7xl space-y-6">
      <div class="admin-toolbar-surface">
        <div class="admin-toolbar">
          <div class="admin-toolbar-group min-w-0 flex-1">
            <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/25 dark:text-blue-300"><Icon name="gift" size="sm" /></div>
            <div class="min-w-0">
              <h1 class="truncate text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.title') }}</h1>
              <p class="mt-1 truncate text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.description') }}</p>
            </div>
          </div>
          <div class="admin-toolbar-group w-full justify-end lg:w-auto lg:flex-none">
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="loading || recordsLoading || attemptBalancesLoading" @click="refreshAll"><Icon name="refresh" size="sm" :class="loading || recordsLoading || attemptBalancesLoading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</button>
            <button type="button" data-test="save-lottery-activity" class="btn btn-primary inline-flex items-center gap-2" :disabled="savingActivity" @click="saveActivityForm"><Icon name="check" size="sm" />{{ savingActivity ? t('common.saving') : t('lottery.admin.saveActivity') }}</button>
          </div>
        </div>
      </div>

      <div v-if="loadError" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300">{{ loadError }}</div>

      <section class="admin-surface p-5 sm:p-6">
        <div class="mb-5 flex items-start gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/25 dark:text-blue-300"><Icon name="calendar" size="sm" /></div>
          <div>
            <h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.activity') }}</h2>
            <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.activityHint') }}</p>
          </div>
        </div>
        <div class="grid gap-4 md:grid-cols-2">
          <div class="md:col-span-2"><label class="input-label">{{ t('lottery.admin.name') }}</label><input v-model="activityForm.name" class="input" :placeholder="t('lottery.admin.namePlaceholder')" /></div>
          <div class="md:col-span-2"><label class="input-label">{{ t('lottery.admin.descriptionLabel') }}</label><textarea v-model="activityForm.description" class="input min-h-24 resize-y" :placeholder="t('lottery.admin.descriptionPlaceholder')"></textarea></div>
          <div><label class="input-label">{{ t('lottery.admin.status') }}</label><select v-model="activityForm.status" class="input"><option value="draft">{{ t('lottery.admin.draft') }}</option><option value="active">{{ t('lottery.admin.active') }}</option><option value="disabled">{{ t('lottery.admin.disabledStatus') }}</option><option value="ended">{{ t('lottery.admin.ended') }}</option></select></div>
          <div><label class="input-label">{{ t('lottery.admin.startsAt') }}</label><input v-model="activityForm.starts_at" class="input" type="datetime-local" /></div>
          <div><label class="input-label">{{ t('lottery.admin.endsAt') }}</label><input v-model="activityForm.ends_at" class="input" type="datetime-local" /></div>
        </div>
      </section>

      <section class="admin-surface p-5 sm:p-6" data-test="lottery-attempt-grant">
        <div class="mb-5 flex items-start gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-900/25 dark:text-emerald-300"><Icon name="gift" size="sm" /></div>
          <div>
            <h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.grantAttempts') }}</h2>
            <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.grantAttemptsHint') }}</p>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div class="md:col-span-2 flex flex-wrap gap-4 text-sm font-medium text-slate-700 dark:text-slate-200">
            <label class="inline-flex cursor-pointer items-center gap-2"><input v-model="grantTarget" data-test="lottery-grant-selected" type="radio" value="selected" class="h-4 w-4 text-emerald-600 focus:ring-emerald-500" />{{ t('lottery.admin.grantSelectedUsers') }}</label>
            <label class="inline-flex cursor-pointer items-center gap-2"><input v-model="grantTarget" data-test="lottery-grant-all" type="radio" value="all" class="h-4 w-4 text-emerald-600 focus:ring-emerald-500" />{{ t('lottery.admin.grantAllUsers') }}</label>
          </div>

          <div v-if="grantTarget === 'selected'" class="md:col-span-2">
            <div v-if="selectedGrantUsers.length" class="mb-2 flex flex-wrap gap-2">
              <span v-for="user in selectedGrantUsers" :key="user.id" class="inline-flex items-center gap-1.5 rounded-md bg-emerald-50 px-2.5 py-1.5 text-xs text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200">
                <span class="max-w-64 truncate">{{ user.email || user.username || `#${user.id}` }}</span>
                <button type="button" class="text-emerald-600 hover:text-red-600 dark:text-emerald-300" :aria-label="t('lottery.admin.removeGrantUser')" @click="removeGrantUser(user.id)">×</button>
              </span>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row">
              <input v-model="grantUserSearch" data-test="lottery-grant-user-search" class="input flex-1" :placeholder="t('lottery.admin.grantUserSearchPlaceholder')" @keyup.enter="searchGrantUsers" />
              <button type="button" data-test="lottery-grant-search" class="btn btn-secondary" :disabled="grantSearching" @click="searchGrantUsers">{{ grantSearching ? t('common.loading') : t('common.search') }}</button>
            </div>
            <div v-if="grantUserResults.length" class="mt-2 max-h-48 overflow-y-auto rounded-lg border border-slate-200 dark:border-dark-700">
              <button v-for="user in grantUserResults" :key="user.id" type="button" :data-test="`lottery-grant-user-result-${user.id}`" class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-slate-50 dark:hover:bg-dark-800" @click="selectGrantUser(user)">
                <span class="min-w-0 truncate text-slate-800 dark:text-slate-100">{{ user.email || user.username || `#${user.id}` }}</span><span class="shrink-0 text-xs text-slate-400">#{{ user.id }}</span>
              </button>
            </div>
          </div>

          <div><label class="input-label">{{ t('lottery.admin.grantAmount') }}</label><input v-model.number="grantAmount" data-test="lottery-grant-amount" class="input" type="number" min="1" step="1" /></div>
          <div><label class="input-label">{{ t('lottery.admin.grantDescription') }}</label><input v-model="grantDescription" data-test="lottery-grant-description" class="input" :placeholder="t('lottery.admin.grantDescriptionPlaceholder')" /></div>
        </div>

        <div class="mt-4 flex items-center justify-between gap-3">
          <p v-if="grantResult" data-test="lottery-grant-result" class="text-sm text-emerald-700 dark:text-emerald-300">{{ t('lottery.admin.grantSuccess') }}: {{ grantResult.affected }} {{ t('lottery.admin.grantUsersAffected') }}, {{ grantResult.total_granted }} {{ t('lottery.admin.grantAttemptsIssued') }}</p>
          <span v-else></span>
          <button type="button" data-test="lottery-grant-submit" class="btn btn-primary" :disabled="grantSaving" @click="submitGrantAttempts"><Icon name="gift" size="sm" />{{ grantSaving ? t('common.saving') : t('lottery.admin.grantSubmit') }}</button>
        </div>
      </section>

      <section class="admin-surface p-5 sm:p-6" data-test="lottery-attempt-balances">
        <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex items-start gap-3">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-cyan-50 text-cyan-600 dark:bg-cyan-900/25 dark:text-cyan-300"><Icon name="users" size="sm" /></div>
            <div><h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.attemptBalances') }}</h2><p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.attemptBalancesHint') }}</p></div>
          </div>
          <span class="text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.attemptBalanceCount', { count: attemptBalancePagination.total }) }}</span>
        </div>

        <div class="mb-4 flex flex-col gap-2 sm:flex-row">
          <input v-model="attemptBalanceSearch" data-test="lottery-attempt-balance-search" class="input flex-1" :placeholder="t('lottery.admin.attemptBalanceSearchPlaceholder')" @keyup.enter="searchAttemptBalances" />
          <button type="button" data-test="lottery-attempt-balance-search-submit" class="btn btn-secondary" :disabled="attemptBalancesLoading" @click="searchAttemptBalances">{{ attemptBalancesLoading ? t('common.loading') : t('common.search') }}</button>
        </div>

        <div v-if="attemptBalancesError" class="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300">{{ attemptBalancesError }}</div>
        <div v-if="attemptBalancesLoading" class="py-10 text-center text-sm text-slate-500 dark:text-slate-400">{{ t('lottery.admin.attemptBalancesLoading') }}</div>
        <div v-else-if="attemptBalances.length" class="overflow-x-auto rounded-xl border border-slate-200 dark:border-dark-700">
          <table class="min-w-full divide-y divide-slate-200 text-left text-sm dark:divide-dark-700">
            <thead class="bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:bg-dark-900/60 dark:text-slate-400">
              <tr>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.attemptBalanceUser') }}</th>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.attemptBalanceStatus') }}</th>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.totalAttemptsRemaining') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200 bg-white dark:divide-dark-700 dark:bg-dark-950/20">
              <tr v-for="balance in attemptBalances" :key="balance.user_id" :data-test="`lottery-attempt-balance-row-${balance.user_id}`">
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="font-medium text-slate-900 dark:text-white">{{ balance.user_email || balance.user_name || `#${balance.user_id}` }}</div>
                  <div v-if="balance.user_name && balance.user_email" class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ balance.user_name }}</div>
                  <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">ID {{ balance.user_id }}</div>
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-slate-600 dark:text-slate-300">{{ formatUserStatus(balance.user_status) }}</td>
                <td class="whitespace-nowrap px-4 py-3 font-semibold text-cyan-700 dark:text-cyan-300">{{ balance.total_remaining }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="rounded-xl border border-dashed border-slate-300 px-5 py-10 text-center text-sm text-slate-500 dark:border-dark-600 dark:text-slate-400">{{ t('lottery.admin.attemptBalancesEmpty') }}</p>

        <Pagination
          v-if="attemptBalancePagination.total > 0"
          :page="attemptBalancePagination.page"
          :total="attemptBalancePagination.total"
          :page-size="attemptBalancePagination.page_size"
          @update:page="handleAttemptBalancePageChange"
          @update:pageSize="handleAttemptBalancePageSizeChange"
        />
      </section>

      <section class="admin-surface p-5 sm:p-6">
        <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex items-start gap-3">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-900/25 dark:text-amber-300"><Icon name="trophy" size="sm" /></div>
            <div><h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.prizes') }}</h2><p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.prizesHint') }}</p></div>
          </div>
          <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" @click="startNewPrize"><Icon name="plus" size="sm" />{{ t('lottery.admin.addPrize') }}</button>
        </div>

        <div v-if="editing" class="mb-6 rounded-xl border border-blue-200 bg-blue-50/60 p-4 dark:border-blue-900/60 dark:bg-blue-950/20">
          <div class="mb-4 flex items-center justify-between gap-3"><h3 class="text-sm font-semibold text-slate-950 dark:text-white">{{ editing.id ? t('lottery.admin.editPrize') : t('lottery.admin.newPrize') }}</h3><button type="button" class="icon-button" :title="t('lottery.close')" @click="editing = null"><Icon name="x" size="sm" /></button></div>
          <div class="grid gap-4 md:grid-cols-2">
            <div><label class="input-label">{{ t('lottery.admin.prizeName') }}</label><input v-model="editing.name" data-test="lottery-prize-name" class="input" /></div>
            <div><label class="input-label">{{ t('lottery.admin.prizeType') }}</label><select v-model="editing.type" data-test="lottery-prize-type" class="input"><option value="balance">{{ t('lottery.admin.balanceType') }}</option><option value="product">{{ t('lottery.admin.productType') }}</option></select></div>
            <div class="md:col-span-2"><label class="input-label">{{ t('lottery.admin.prizeDescription') }}</label><input v-model="editing.description" class="input" /></div>
            <div v-if="editing.type === 'balance'"><label class="input-label">{{ t('lottery.admin.balanceAmount') }}</label><input v-model.number="editing.balance_amount" data-test="lottery-prize-balance-amount" class="input" type="number" min="0.01" step="0.01" /></div>
            <div><label class="input-label">{{ t('lottery.admin.weight') }}</label><input v-model.number="editing.weight" data-test="lottery-prize-weight" class="input" type="number" min="1" step="1" /></div>
            <div><label class="input-label">{{ t('lottery.admin.sortOrder') }}</label><input v-model.number="editing.sort_order" data-test="lottery-prize-sort-order" class="input" type="number" step="1" /></div>
            <label class="flex cursor-pointer items-center gap-3 self-end pb-2 text-sm font-medium text-slate-700 dark:text-slate-200"><input v-model="editing.enabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" />{{ t('lottery.admin.enabled') }}</label>
          </div>
          <div class="mt-4 flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="editing = null">{{ t('lottery.close') }}</button><button type="button" data-test="lottery-save-prize" class="btn btn-primary" :disabled="savingPrize" @click="savePrize"><Icon name="check" size="sm" />{{ savingPrize ? t('common.saving') : t('lottery.admin.savePrize') }}</button></div>
        </div>

        <div v-if="prizes.length" class="space-y-4">
          <article v-for="prize in prizes" :key="prize.id" class="rounded-xl border border-slate-200 dark:border-dark-700">
            <div class="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><h3 class="break-words text-sm font-semibold text-slate-950 dark:text-white">{{ prize.name }}</h3><span class="rounded bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500 dark:bg-dark-700 dark:text-slate-300">{{ prize.type === 'balance' ? t('lottery.admin.balanceType') : t('lottery.admin.productType') }}</span><span v-if="!prize.enabled" class="rounded bg-slate-100 px-2 py-0.5 text-[11px] text-slate-500 dark:bg-dark-700 dark:text-slate-300">{{ t('lottery.admin.disabledStatus') }}</span></div><p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ prize.type === 'balance' ? `${t('lottery.admin.balanceAmount')}: ${formatAmount(prize.balance_amount)}` : `${t('lottery.admin.available', { count: prize.available_item_count })}` }} · {{ t('lottery.admin.weight') }} {{ prize.weight }}</p></div>
              <div class="flex flex-wrap items-center gap-2"><button v-if="prize.type === 'product'" type="button" class="btn btn-secondary btn-sm" @click="toggleInventory(prize.id)"><Icon name="database" size="sm" />{{ t('lottery.admin.inventory') }}</button><button type="button" class="btn btn-secondary btn-sm" @click="editPrize(prize)"><Icon name="edit" size="sm" />{{ t('lottery.admin.editPrize') }}</button><button type="button" class="btn btn-secondary btn-sm text-red-600 hover:text-red-700" @click="removePrize(prize)"><Icon name="trash" size="sm" /></button></div>
            </div>
            <div v-if="prize.type === 'product' && inventoryOpen === prize.id" class="border-t border-slate-200 bg-slate-50/70 p-4 dark:border-dark-700 dark:bg-dark-950/40">
              <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,360px)]">
                <div><div class="mb-2 flex items-center justify-between gap-2"><p class="text-xs font-semibold text-slate-700 dark:text-slate-200">{{ t('lottery.admin.inventory') }}</p><span class="text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.available', { count: inventoryItems.length ? inventoryItems.filter(item => item.status === 'available').length : prize.available_item_count }) }}</span></div><div v-if="inventoryItems.length" class="max-h-56 space-y-2 overflow-y-auto"> <label v-for="item in inventoryItems" :key="item.id" class="flex items-start gap-2 rounded-lg border border-slate-200 bg-white p-2 text-xs dark:border-dark-700 dark:bg-dark-900"><input v-if="item.status === 'available'" v-model="selectedItemIds" :value="item.id" type="checkbox" class="mt-0.5 rounded border-slate-300 text-blue-600" /><code class="min-w-0 flex-1 break-all" :class="item.status === 'claimed' ? 'text-slate-400 line-through' : 'text-slate-700 dark:text-slate-200'">{{ item.content }}</code><span class="shrink-0 text-[10px] text-slate-400">{{ item.status }}</span></label></div><p v-else class="py-5 text-center text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.noInventory') }}</p><button v-if="selectedItemIds.length" type="button" class="btn btn-secondary btn-sm mt-3 text-red-600" @click="deleteSelectedItems(prize.id)">{{ t('lottery.admin.deleteAvailable') }} ({{ selectedItemIds.length }})</button></div>
                <div><label class="input-label">{{ t('lottery.admin.appendInventory') }}</label><p class="mb-2 text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.inventoryHint') }}</p><textarea v-model="inventoryText" class="input min-h-32 resize-y font-mono text-xs" :placeholder="t('lottery.admin.inventoryPlaceholder')"></textarea><button type="button" class="btn btn-primary mt-3 w-full justify-center" :disabled="!inventoryText.trim() || inventorySaving" @click="appendInventory(prize.id)"><Icon name="upload" size="sm" />{{ inventorySaving ? t('common.saving') : t('lottery.admin.appendInventory') }}</button></div>
              </div>
            </div>
          </article>
        </div>
        <p v-else class="rounded-xl border border-dashed border-slate-300 px-5 py-10 text-center text-sm text-slate-500 dark:border-dark-600 dark:text-slate-400">{{ t('lottery.noPrizes') }}</p>
      </section>

      <section class="admin-surface p-5 sm:p-6" data-test="lottery-draw-records">
        <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex items-start gap-3">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-violet-50 text-violet-600 dark:bg-violet-900/25 dark:text-violet-300"><Icon name="clock" size="sm" /></div>
            <div><h2 class="text-base font-semibold text-slate-950 dark:text-white">{{ t('lottery.admin.drawRecords') }}</h2><p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('lottery.admin.drawRecordsHint') }}</p></div>
          </div>
          <span class="text-xs text-slate-500 dark:text-slate-400">{{ t('lottery.admin.drawRecordCount', { count: recordsPagination.total }) }}</span>
        </div>

        <div v-if="recordsError" class="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300">{{ recordsError }}</div>
        <div v-if="recordsLoading" class="py-10 text-center text-sm text-slate-500 dark:text-slate-400">{{ t('lottery.admin.recordsLoading') }}</div>
        <div v-else-if="records.length" class="overflow-x-auto rounded-xl border border-slate-200 dark:border-dark-700">
          <table class="min-w-full divide-y divide-slate-200 text-left text-sm dark:divide-dark-700">
            <thead class="bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:bg-dark-900/60 dark:text-slate-400">
              <tr>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.drawUser') }}</th>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.drawPrize') }}</th>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.drawReward') }}</th>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.drawSource') }}</th>
                <th class="px-4 py-3 font-semibold">{{ t('lottery.admin.drawTime') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200 bg-white dark:divide-dark-700 dark:bg-dark-950/20">
              <tr v-for="record in records" :key="record.id" :data-test="`lottery-draw-record-${record.id}`" class="align-top">
                <td class="whitespace-nowrap px-4 py-3">
                  <div class="font-medium text-slate-900 dark:text-white">{{ record.user_deleted ? t('lottery.admin.deletedUser') : (record.user_email || record.user_name || `#${record.user_id}`) }}</div>
                  <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">ID {{ record.user_id }}</div>
                </td>
                <td class="whitespace-nowrap px-4 py-3">
                  <div :data-test="`lottery-draw-prize-${record.id}`" class="font-medium text-slate-900 dark:text-white">{{ record.prize_name }}</div>
                  <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ record.prize_type === 'balance' ? t('lottery.admin.balanceType') : t('lottery.admin.productType') }}</div>
                </td>
                <td class="max-w-sm px-4 py-3 text-slate-700 dark:text-slate-200">
                  <span v-if="record.prize_type === 'balance'">{{ formatAmount(record.balance_amount) }}</span>
                  <code v-else class="break-all text-xs">{{ record.product_content || '—' }}</code>
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-slate-600 dark:text-slate-300">{{ record.attempt_source === 'wallet' ? t('lottery.admin.walletSource') : t('lottery.admin.activitySource') }}</td>
                <td class="whitespace-nowrap px-4 py-3 text-slate-500 dark:text-slate-400">{{ formatDateTime(record.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="rounded-xl border border-dashed border-slate-300 px-5 py-10 text-center text-sm text-slate-500 dark:border-dark-600 dark:text-slate-400">{{ t('lottery.admin.recordsEmpty') }}</p>

        <Pagination
          v-if="recordsPagination.total > 0"
          :page="recordsPagination.page"
          :total="recordsPagination.total"
          :page-size="recordsPagination.page_size"
          @update:page="handleRecordsPageChange"
          @update:pageSize="handleRecordsPageSizeChange"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { adminAPI } from '@/api/admin'
import { lotteryAdminAPI, type LotteryAdminAttemptBalance, type LotteryAdminDraw, type LotteryPrizeItem } from '@/api/admin/lottery'
import type { LotteryActivity, LotteryPrize } from '@/api/lottery'
import type { AdminUser } from '@/types'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const loadError = ref('')
const savingActivity = ref(false)
const savingPrize = ref(false)
const inventorySaving = ref(false)
const activityId = ref(0)
const prizes = ref<LotteryPrize[]>([])
const editing = ref<PrizeDraft | null>(null)
const inventoryOpen = ref(0)
const inventoryText = ref('')
const inventoryItems = ref<LotteryPrizeItem[]>([])
const selectedItemIds = ref<number[]>([])
const recordsLoading = ref(true)
const recordsError = ref('')
const records = ref<LotteryAdminDraw[]>([])
const recordsPagination = reactive({ page: 1, page_size: 10, total: 0 })
const attemptBalancesLoading = ref(true)
const attemptBalancesError = ref('')
const attemptBalances = ref<LotteryAdminAttemptBalance[]>([])
const attemptBalanceSearch = ref('')
const attemptBalancePagination = reactive({ page: 1, page_size: 10, total: 0 })
const grantTarget = ref<'selected' | 'all'>('selected')
const grantUserSearch = ref('')
const grantUserResults = ref<AdminUser[]>([])
const selectedGrantUsers = ref<AdminUser[]>([])
const grantAmount = ref(1)
const grantDescription = ref('')
const grantSearching = ref(false)
const grantSaving = ref(false)
const grantResult = ref<{ affected: number; total_granted: number } | null>(null)
const newGrantRequestKey = () => {
  const cryptoAPI = globalThis.crypto as (Crypto & { randomUUID?: () => string }) | undefined
  return cryptoAPI?.randomUUID?.() || `lottery-${Date.now()}-${Math.random().toString(36).slice(2)}`
}
const grantRequestKey = ref(newGrantRequestKey())

interface ActivityForm { id?: number; name: string; description: string; status: string; starts_at: string; ends_at: string }
interface PrizeDraft { id?: number; name: string; description: string; type: 'balance' | 'product'; weight: number; balance_amount?: number | null; enabled: boolean; sort_order: number }

const activityForm = reactive<ActivityForm>({ name: '', description: '', status: 'draft', starts_at: '', ends_at: '' })
const formatAmount = (value?: number | null) => `$${Number(value || 0).toFixed(2)}`
const formatDateTime = (value?: string | null) => value ? new Date(value).toLocaleString() : '—'
const formatUserStatus = (status: string) => status === 'active' ? t('lottery.admin.active') : status === 'disabled' ? t('lottery.admin.disabledStatus') : status
const toDateTimeLocal = (value?: string | null) => value ? new Date(value).toISOString().slice(0, 16) : ''
const toISOStringOrNull = (value: string) => value ? new Date(value).toISOString() : null

function applyActivity(activity?: LotteryActivity | null) {
  activityId.value = activity?.id || 0
  activityForm.id = activity?.id
  activityForm.name = activity?.name || ''
  activityForm.description = activity?.description || ''
  activityForm.status = activity?.status || 'draft'
  activityForm.starts_at = toDateTimeLocal(activity?.starts_at)
  activityForm.ends_at = toDateTimeLocal(activity?.ends_at)
}

async function loadConfig() {
  loading.value = true
  loadError.value = ''
  try {
    const config = await lotteryAdminAPI.getConfig()
    applyActivity(config.activity)
    prizes.value = config.prizes || []
  } catch (error: any) {
    loadError.value = error?.message || t('lottery.admin.loadFailed')
  } finally { loading.value = false }
}

async function loadDraws(page = recordsPagination.page) {
  recordsLoading.value = true
  recordsError.value = ''
  try {
    const response = await lotteryAdminAPI.listDraws({ page, page_size: recordsPagination.page_size })
    records.value = response.items || []
    recordsPagination.page = response.page || page
    recordsPagination.page_size = response.page_size || recordsPagination.page_size
    recordsPagination.total = response.total || 0
  } catch (error: any) {
    recordsError.value = error?.message || t('lottery.admin.recordsLoadFailed')
  } finally { recordsLoading.value = false }
}

async function loadAttemptBalances(page = attemptBalancePagination.page) {
  attemptBalancesLoading.value = true
  attemptBalancesError.value = ''
  try {
    const response = await lotteryAdminAPI.listAttemptBalances({ page, page_size: attemptBalancePagination.page_size, search: attemptBalanceSearch.value.trim() })
    attemptBalances.value = response.items || []
    attemptBalancePagination.page = response.page || page
    attemptBalancePagination.page_size = response.page_size || attemptBalancePagination.page_size
    attemptBalancePagination.total = response.total || 0
  } catch (error: any) {
    attemptBalancesError.value = error?.message || t('lottery.admin.attemptBalancesLoadFailed')
  } finally { attemptBalancesLoading.value = false }
}

function searchAttemptBalances() {
  attemptBalancePagination.page = 1
  loadAttemptBalances(1)
}

async function searchGrantUsers() {
  const search = grantUserSearch.value.trim()
  if (!search) {
    appStore.showError(t('lottery.admin.grantUserSearchRequired'))
    return
  }
  grantSearching.value = true
  try {
    const response = await adminAPI.users.list(1, 8, { search })
    const selected = new Set(selectedGrantUsers.value.map(user => user.id))
    grantUserResults.value = (response.items || []).filter(user => !selected.has(user.id))
  } catch (error: any) {
    appStore.showError(error?.message || t('lottery.admin.grantSearchFailed'))
  } finally { grantSearching.value = false }
}

function selectGrantUser(user: AdminUser) {
  if (!selectedGrantUsers.value.some(item => item.id === user.id)) selectedGrantUsers.value.push(user)
  grantUserResults.value = grantUserResults.value.filter(item => item.id !== user.id)
}

function removeGrantUser(userID: number) {
  selectedGrantUsers.value = selectedGrantUsers.value.filter(user => user.id !== userID)
}

async function submitGrantAttempts() {
  grantResult.value = null
  const amount = Math.floor(Number(grantAmount.value) || 0)
  if (amount <= 0) {
    appStore.showError(t('lottery.admin.grantAmountInvalid'))
    return
  }
  if (grantTarget.value === 'selected' && selectedGrantUsers.value.length === 0) {
    appStore.showError(t('lottery.admin.grantUsersRequired'))
    return
  }
  grantSaving.value = true
  try {
    const request = grantTarget.value === 'all'
      ? { all: true, amount, description: grantDescription.value.trim(), request_key: grantRequestKey.value }
      : { user_ids: selectedGrantUsers.value.map(user => user.id), amount, description: grantDescription.value.trim(), request_key: grantRequestKey.value }
    grantResult.value = await lotteryAdminAPI.grantAttempts(request)
    appStore.showSuccess(t('lottery.admin.grantSuccess'))
    selectedGrantUsers.value = []
    grantUserResults.value = []
    grantUserSearch.value = ''
    grantDescription.value = ''
    grantRequestKey.value = newGrantRequestKey()
    await loadAttemptBalances(1)
  } catch (error: any) {
    appStore.showError(error?.message || t('lottery.admin.grantFailed'))
  } finally { grantSaving.value = false }
}

async function refreshAll() {
  await Promise.all([loadConfig(), loadDraws(1), loadAttemptBalances(1)])
}

function handleRecordsPageChange(page: number) {
  recordsPagination.page = page
  loadDraws(page)
}

function handleRecordsPageSizeChange(pageSize: number) {
  recordsPagination.page_size = pageSize
  recordsPagination.page = 1
  loadDraws(1)
}

function handleAttemptBalancePageChange(page: number) {
  attemptBalancePagination.page = page
  loadAttemptBalances(page)
}

function handleAttemptBalancePageSizeChange(pageSize: number) {
  attemptBalancePagination.page_size = pageSize
  attemptBalancePagination.page = 1
  loadAttemptBalances(1)
}

async function saveActivityForm() {
  if (!activityForm.name.trim()) { appStore.showError(t('lottery.admin.saveFailed')); return }
  savingActivity.value = true
  try {
    const saved = await lotteryAdminAPI.saveActivity({ id: activityForm.id, name: activityForm.name.trim(), description: activityForm.description, status: activityForm.status, starts_at: toISOStringOrNull(activityForm.starts_at), ends_at: toISOStringOrNull(activityForm.ends_at) })
    applyActivity(saved)
    await loadAttemptBalances(1)
    appStore.showSuccess(t('lottery.admin.saved'))
  } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) } finally { savingActivity.value = false }
}

function startNewPrize() {
  if (!activityId.value) { appStore.showWarning(t('lottery.admin.saveFailed')); return }
  editing.value = { name: '', description: '', type: 'balance', weight: 1, balance_amount: 1, enabled: true, sort_order: prizes.value.length }
}

function editPrize(prize: LotteryPrize) {
  editing.value = { id: prize.id, name: prize.name, description: prize.description, type: prize.type, weight: prize.weight, balance_amount: prize.balance_amount, enabled: prize.enabled, sort_order: prize.sort_order }
}

async function savePrize() {
  if (!editing.value || !activityId.value) return
  const draft = editing.value
  const name = draft.name.trim()
  if (!name || name.length > 120) {
    appStore.showError(t('lottery.admin.prizeNameInvalid'))
    return
  }
  if (draft.type !== 'balance' && draft.type !== 'product') {
    appStore.showError(t('lottery.admin.prizeTypeInvalid'))
    return
  }
  const balanceAmount = draft.type === 'balance' ? Number(draft.balance_amount) : null
  if (draft.type === 'balance' && (balanceAmount === null || !Number.isFinite(balanceAmount) || balanceAmount <= 0)) {
    appStore.showError(t('lottery.admin.prizeBalanceAmountInvalid'))
    return
  }

  savingPrize.value = true
  try {
    const request = { name, description: draft.description, type: draft.type, weight: Math.max(1, Number(draft.weight) || 1), balance_amount: balanceAmount, enabled: draft.enabled, sort_order: Number(draft.sort_order) || 0 }
    const saved = draft.id ? await lotteryAdminAPI.updatePrize(draft.id, request) : await lotteryAdminAPI.createPrize({ ...request, activity_id: activityId.value })
    const index = prizes.value.findIndex(item => item.id === saved.id)
    if (index >= 0) prizes.value[index] = saved
    else prizes.value.push(saved)
    editing.value = null
    appStore.showSuccess(t('lottery.admin.prizeSaved'))
  } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) } finally { savingPrize.value = false }
}

async function removePrize(prize: LotteryPrize) {
  if (!window.confirm(t('lottery.admin.deleteConfirm'))) return
  try { await lotteryAdminAPI.deletePrize(prize.id); prizes.value = prizes.value.filter(item => item.id !== prize.id); if (inventoryOpen.value === prize.id) inventoryOpen.value = 0; appStore.showSuccess(t('lottery.admin.prizeDeleted')) } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) }
}

async function toggleInventory(prizeId: number) {
  if (inventoryOpen.value === prizeId) { inventoryOpen.value = 0; return }
  inventoryOpen.value = prizeId; inventoryText.value = ''; selectedItemIds.value = []
  try { inventoryItems.value = await lotteryAdminAPI.listPrizeItems(prizeId) } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.loadFailed')) }
}

async function appendInventory(prizeId: number) {
  const contents = inventoryText.value.split(/\r?\n/).map(item => item.trim()).filter(Boolean)
  if (!contents.length) return
  inventorySaving.value = true
  try { const result = await lotteryAdminAPI.appendPrizeItems(prizeId, contents); inventoryText.value = ''; inventoryItems.value = await lotteryAdminAPI.listPrizeItems(prizeId); await loadConfig(); inventoryOpen.value = prizeId; appStore.showSuccess(t('lottery.admin.inventoryAdded', { count: result.added })) } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) } finally { inventorySaving.value = false }
}

async function deleteSelectedItems(prizeId: number) {
  if (!selectedItemIds.value.length) return
  try { await lotteryAdminAPI.deletePrizeItems(prizeId, selectedItemIds.value); inventoryItems.value = await lotteryAdminAPI.listPrizeItems(prizeId); selectedItemIds.value = []; await loadConfig(); inventoryOpen.value = prizeId } catch (error: any) { appStore.showError(error?.message || t('lottery.admin.saveFailed')) }
}

onMounted(() => {
  loadConfig()
  loadDraws()
  loadAttemptBalances()
})
</script>

<style scoped>
.admin-lottery-page { padding-bottom: 2rem; }
</style>
