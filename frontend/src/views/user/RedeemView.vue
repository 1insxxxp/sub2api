<template>
  <AppLayout>
    <div
      class="redeem-shell mx-auto w-full max-w-4xl space-y-5 sm:space-y-6"
      data-redeem-build="assets-404-hotfix-20260823"
    >
      <!-- Current Balance Card -->
      <section class="redeem-balance-card">
        <div class="redeem-balance-orbit"></div>
        <div class="redeem-balance-content">
          <div class="redeem-balance-main">
            <div class="redeem-balance-icon">
              <Icon name="creditCard" size="lg" class="text-white" />
            </div>
            <div class="min-w-0">
              <p class="redeem-eyebrow">{{ t('redeem.currentBalance') }}</p>
              <p class="redeem-balance-value">
                ${{ user?.balance?.toFixed(2) || '0.00' }}
              </p>
            </div>
          </div>
          <div class="redeem-concurrency-metric">
            <div class="redeem-concurrency-icon">
              <Icon name="bolt" size="md" class="text-white" />
            </div>
            <div class="min-w-0">
              <p class="redeem-concurrency-label">{{ t('redeem.concurrency') }}</p>
              <div class="redeem-concurrency-row">
                <strong class="redeem-concurrency-value">{{ user?.concurrency || 0 }}</strong>
                <span class="redeem-concurrency-unit">{{ t('redeem.requests') }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Redeem Form -->
      <section class="brand-surface redeem-code-panel">
        <div class="p-5 sm:p-6">
          <form @submit.prevent="handleRedeem" class="redeem-form">
            <div class="redeem-panel-header">
              <div class="brand-floating-icon redeem-panel-icon">
                <Icon name="gift" size="md" />
              </div>
              <div class="min-w-0">
                <h2 class="text-base font-semibold text-slate-950 dark:text-white">
                  {{ t('redeem.redeemCodeLabel') }}
                </h2>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                  {{ t('redeem.redeemCodeHint') }}
                </p>
              </div>
            </div>
            <div>
              <label for="code" class="sr-only">
                {{ t('redeem.redeemCodeLabel') }}
              </label>
              <div class="redeem-input-wrap">
                <div class="redeem-input-icon">
                  <Icon name="gift" size="md" />
                </div>
                <input
                  id="code"
                  v-model="redeemCode"
                  type="text"
                  required
                  :placeholder="t('redeem.redeemCodePlaceholder')"
                  :disabled="submitting"
                  class="redeem-input"
                />
              </div>
            </div>

            <button
              type="submit"
              :disabled="!redeemCode || submitting"
              class="btn btn-primary redeem-submit w-full"
            >
              <svg
                v-if="submitting"
                class="-ml-1 mr-2 h-5 w-5 animate-spin"
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
              <Icon v-else name="checkCircle" size="md" class="mr-2" />
              {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
            </button>
          </form>
        </div>
      </section>

      <!-- Balance Transfer Code Generator -->
      <section
        v-if="canGenerateBalanceTransferCodes"
        data-test="balance-transfer-panel"
        class="brand-surface redeem-transfer-panel"
      >
        <div class="p-5 sm:p-6">
          <div class="redeem-panel-header">
            <div class="brand-floating-icon redeem-panel-icon">
              <Icon name="swap" size="md" />
            </div>
            <div class="min-w-0">
              <h2 class="text-base font-semibold text-slate-950 dark:text-white">
                {{ t('redeem.balanceTransfer.title') }}
              </h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                {{ t('redeem.balanceTransfer.subtitle') }}
              </p>
            </div>
          </div>

          <form
            data-test="balance-transfer-form"
            class="redeem-transfer-form"
            @submit.prevent="handleGenerateBalanceTransferCode"
          >
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_7.5rem_9.5rem]">
              <div>
                <label for="balance-transfer-amount" class="redeem-transfer-label">
                  {{ t('redeem.balanceTransfer.amount') }}
                </label>
                <input
                  id="balance-transfer-amount"
                  v-model="transferForm.amount"
                  data-test="balance-transfer-amount"
                  type="number"
                  min="0.01"
                  step="0.01"
                  inputmode="decimal"
                  class="redeem-transfer-field"
                  :placeholder="t('redeem.balanceTransfer.amountPlaceholder')"
                  :disabled="generatingTransferCode"
                />
              </div>
              <div>
                <label for="balance-transfer-count" class="redeem-transfer-label">
                  {{ t('redeem.balanceTransfer.count') }}
                </label>
                <input
                  id="balance-transfer-count"
                  v-model.number="transferForm.count"
                  data-test="balance-transfer-count"
                  type="number"
                  min="1"
                  max="100"
                  step="1"
                  class="redeem-transfer-field"
                  :disabled="generatingTransferCode"
                />
              </div>
              <div>
                <label for="balance-transfer-expiry" class="redeem-transfer-label">
                  {{ t('redeem.balanceTransfer.expiresInDays') }}
                </label>
                <input
                  id="balance-transfer-expiry"
                  v-model.number="transferForm.expires_in_days"
                  data-test="balance-transfer-expiry"
                  type="number"
                  min="1"
                  max="3650"
                  step="1"
                  class="redeem-transfer-field"
                  :disabled="generatingTransferCode"
                />
              </div>
            </div>

            <div>
              <label for="balance-transfer-notes" class="redeem-transfer-label">
                {{ t('redeem.balanceTransfer.notes') }}
              </label>
              <input
                id="balance-transfer-notes"
                v-model="transferForm.notes"
                data-test="balance-transfer-notes"
                type="text"
                maxlength="120"
                class="redeem-transfer-field"
                :placeholder="t('redeem.balanceTransfer.notesPlaceholder')"
                :disabled="generatingTransferCode"
              />
            </div>

            <label class="redeem-transfer-toggle">
              <input
                v-model="transferForm.single_use_per_user"
                data-test="balance-transfer-single-use"
                type="checkbox"
                :disabled="generatingTransferCode"
              />
              <span>
                <strong>{{ t('redeem.balanceTransfer.singleUsePerUser') }}</strong>
                <small>{{ t('redeem.balanceTransfer.singleUsePerUserHint') }}</small>
              </span>
            </label>

            <p
              v-if="transferErrorMessage"
              data-test="balance-transfer-error"
              class="text-sm font-medium text-red-600 dark:text-red-400"
            >
              {{ transferErrorMessage }}
            </p>

            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p class="text-sm text-slate-500 dark:text-slate-400">
                {{ t('redeem.balanceTransfer.availableBalance') }}:
                <span class="font-semibold text-slate-700 dark:text-slate-200">
                  ${{ availableBalance.toFixed(2) }}
                </span>
              </p>
              <button
                type="submit"
                class="btn btn-primary redeem-transfer-submit"
                :disabled="generatingTransferCode"
              >
                <svg
                  v-if="generatingTransferCode"
                  class="-ml-1 mr-2 h-5 w-5 animate-spin"
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
                <Icon v-else name="gift" size="md" class="mr-2" />
                {{
                  generatingTransferCode
                    ? t('redeem.balanceTransfer.generating')
                    : t('redeem.balanceTransfer.generate')
                }}
              </button>
            </div>
          </form>

          <div class="redeem-generated-list">
            <div class="redeem-generated-toolbar">
              <div class="min-w-0">
                <h3 class="text-sm font-semibold text-slate-900 dark:text-white">
                  {{ t('redeem.balanceTransfer.listTitle') }}
                </h3>
                <p
                  v-if="selectedGeneratedCodeIds.length > 0"
                  class="mt-1 text-xs text-slate-500 dark:text-slate-400"
                >
                  {{
                    t('redeem.balanceTransfer.selectedCount', {
                      count: selectedGeneratedCodeIds.length
                    })
                  }}
                </p>
              </div>
              <div class="redeem-generated-toolbar-actions">
                <label
                  v-if="deletableGeneratedCodes.length > 0"
                  class="redeem-generated-select-all"
                >
                  <input
                    data-test="select-all-generated-codes"
                    type="checkbox"
                    :checked="allDeletableGeneratedCodesSelected"
                    :disabled="batchDeletingGeneratedCodes"
                    @change="handleSelectAllGeneratedCodes"
                  />
                  <span>{{ t('redeem.balanceTransfer.selectAll') }}</span>
                </label>
                <button
                  type="button"
                  data-test="delete-selected-generated-codes"
                  class="btn btn-secondary flex items-center gap-2 px-3 py-2 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                  :disabled="selectedGeneratedCodeIds.length === 0 || batchDeletingGeneratedCodes"
                  @click="handleDeleteSelectedGeneratedCodes"
                >
                  <Icon
                    :name="batchDeletingGeneratedCodes ? 'refresh' : 'trash'"
                    size="sm"
                    :class="batchDeletingGeneratedCodes ? 'animate-spin' : ''"
                  />
                  <span>{{ t('redeem.balanceTransfer.batchDelete') }}</span>
                </button>
                <button
                  type="button"
                  class="btn btn-secondary px-3 py-2"
                  :disabled="loadingGeneratedCodes"
                  @click="fetchGeneratedCodes"
                >
                  <Icon name="refresh" size="sm" />
                </button>
              </div>
            </div>

            <div v-if="loadingGeneratedCodes" class="flex items-center justify-center py-6">
              <svg class="h-5 w-5 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
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
            </div>

            <div v-else-if="generatedCodes.length > 0" class="mt-3 space-y-3">
              <div
                v-for="item in generatedCodes"
                :key="item.id"
                class="redeem-generated-row"
              >
                <label
                  v-if="canDeleteGeneratedCode(item)"
                  class="redeem-generated-row-select"
                >
                  <input
                    v-model="selectedGeneratedCodeIds"
                    :value="item.id"
                    type="checkbox"
                    :data-test="`select-generated-code-${item.id}`"
                    :disabled="isDeletingGeneratedCode(item.id) || batchDeletingGeneratedCodes"
                  />
                </label>
                <span v-else class="redeem-generated-row-select-spacer"></span>
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="break-all font-mono text-sm font-semibold text-slate-950 dark:text-white">
                      {{ item.code }}
                    </span>
                    <span :class="getGeneratedStatusClass(item.status)">
                      {{ getGeneratedStatusLabel(item.status) }}
                    </span>
                    <span
                      v-if="item.single_use_per_user"
                      class="inline-flex items-center rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                    >
                      {{ t('redeem.balanceTransfer.singleUseBadge') }}
                    </span>
                  </div>
                  <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                    ${{ item.value.toFixed(2) }} ·
                    {{ t('redeem.balanceTransfer.expiresAt') }}:
                    {{ formatGeneratedExpiry(item) }}
                  </p>
                </div>
                <div class="redeem-generated-actions">
                  <button
                    type="button"
                    class="btn btn-secondary shrink-0 px-3 py-2"
                    @click="copyTransferCode(item.code)"
                  >
                    <Icon name="copy" size="sm" />
                  </button>
                  <button
                    v-if="canDeleteGeneratedCode(item)"
                    type="button"
                    class="btn btn-secondary shrink-0 px-3 py-2 text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                    :data-test="`delete-generated-code-${item.id}`"
                    :disabled="isDeletingGeneratedCode(item.id)"
                    @click="handleDeleteGeneratedCode(item)"
                  >
                    <Icon name="xCircle" size="sm" />
                  </button>
                </div>
              </div>
            </div>

            <p v-else class="py-5 text-center text-sm text-slate-500 dark:text-slate-400">
              {{ t('redeem.balanceTransfer.empty') }}
            </p>

            <Pagination
              v-if="!loadingGeneratedCodes && generatedPagination.total > generatedPagination.page_size"
              class="mt-3 overflow-hidden rounded-lg border border-slate-200 dark:border-dark-700"
              :page="generatedPagination.page"
              :total="generatedPagination.total"
              :page-size="generatedPagination.page_size"
              :page-size-options="redeemPageSizeOptions"
              @update:page="handleGeneratedPageChange"
              @update:pageSize="handleGeneratedPageSizeChange"
            />
          </div>
        </div>
      </section>

      <transition name="fade">
        <div
          v-if="showGeneratedCodesModal"
          data-test="generated-codes-modal"
          class="redeem-generated-modal-backdrop"
        >
          <div class="redeem-generated-modal" role="dialog" aria-modal="true">
            <div class="redeem-generated-modal-header">
              <div class="flex items-center gap-3">
                <div class="redeem-generated-modal-icon">
                  <Icon name="check" size="md" />
                </div>
                <div class="min-w-0">
                  <h2 class="text-base font-semibold text-slate-950 dark:text-white">
                    {{ t('redeem.balanceTransfer.generatedModalTitle') }}
                  </h2>
                  <p class="mt-0.5 text-sm text-slate-500 dark:text-slate-400">
                    {{
                      t('redeem.balanceTransfer.generatedModalSubtitle', {
                        count: generatedCodeResults.length
                      })
                    }}
                  </p>
                </div>
              </div>
              <button
                type="button"
                class="redeem-generated-modal-close"
                :aria-label="t('common.close')"
                @click="closeGeneratedCodesModal"
              >
                <Icon name="x" size="md" />
              </button>
            </div>

            <div class="redeem-generated-modal-body">
              <div class="redeem-generated-modal-code-list">
                <div
                  v-for="item in generatedCodeResults"
                  :key="item.id"
                  class="redeem-generated-modal-code-row"
                >
                  <span class="break-all font-mono text-sm font-semibold text-slate-950 dark:text-white">
                    {{ item.code }}
                  </span>
                  <span
                    v-if="item.single_use_per_user"
                    class="inline-flex shrink-0 items-center rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                  >
                    {{ t('redeem.balanceTransfer.singleUseBadge') }}
                  </span>
                </div>
              </div>
            </div>

            <div class="redeem-generated-modal-footer">
              <button
                type="button"
                data-test="generated-codes-copy-all"
                class="btn btn-secondary flex items-center"
                @click="copyGeneratedCodeResults"
              >
                <Icon name="copy" size="sm" class="mr-2" />
                {{ t('redeem.balanceTransfer.copyAll') }}
              </button>
              <button
                type="button"
                data-test="generated-codes-download"
                class="btn btn-primary flex items-center"
                @click="downloadGeneratedCodeResults"
              >
                <Icon name="download" size="sm" class="mr-2" />
                {{ t('redeem.balanceTransfer.download') }}
              </button>
            </div>
          </div>
        </div>
      </transition>

      <transition name="fade">
        <div
          v-if="showSubscriptionRedeemGuide"
          data-test="subscription-redeem-guide"
          class="redeem-generated-modal-backdrop"
        >
          <div
            class="redeem-subscription-guide-modal"
            role="dialog"
            aria-modal="true"
            :aria-label="t('redeem.subscriptionGuide.title')"
          >
            <div class="redeem-subscription-guide-header">
              <div class="redeem-subscription-guide-hero">
                <div class="redeem-subscription-guide-icon">
                  <img
                    v-if="showSubscriptionGuideLogo"
                    :src="subscriptionGuideLogoSrc"
                    alt=""
                    aria-hidden="true"
                    class="redeem-subscription-guide-logo"
                    @error="handleSubscriptionGuideLogoError"
                  />
                  <Icon v-else name="badge" size="lg" :stroke-width="2" />
                </div>
                <div class="min-w-0">
                  <p class="redeem-subscription-guide-eyebrow">
                    {{ t('redeem.subscriptionGuide.eyebrow') }}
                  </p>
                  <h2 class="redeem-subscription-guide-title">
                    {{ t('redeem.subscriptionGuide.title') }}
                  </h2>
                  <p class="redeem-subscription-guide-subtitle">
                    {{
                      t('redeem.subscriptionGuide.subtitle', {
                        group: subscriptionGuideGroupName
                      })
                    }}
                  </p>
                </div>
              </div>
              <button
                type="button"
                data-test="subscription-guide-close"
                class="redeem-generated-modal-close"
                :aria-label="t('common.close')"
                @click="closeSubscriptionRedeemGuide"
              >
                <Icon name="x" size="md" />
              </button>
            </div>

            <div class="redeem-subscription-guide-body">
              <div class="redeem-subscription-guide-summary">
                <div v-if="subscriptionGuideGroupName" class="redeem-subscription-guide-chip">
                  <span>{{ t('redeem.subscriptionGuide.groupLabel') }}</span>
                  <strong>{{ subscriptionGuideGroupName }}</strong>
                </div>
                <div v-if="subscriptionGuideValidityDays" class="redeem-subscription-guide-chip">
                  <span>{{ t('redeem.subscriptionGuide.daysLabel') }}</span>
                  <strong>{{ subscriptionGuideValidityDays }}{{ t('redeem.days') }}</strong>
                </div>
              </div>

              <ol class="redeem-subscription-guide-steps">
                <li>
                  <span class="redeem-subscription-guide-step-icon">
                    <Icon name="key" size="sm" />
                  </span>
                  <span>
                    <strong>{{ t('redeem.subscriptionGuide.stepApiKeyTitle') }}</strong>
                    <small>{{ t('redeem.subscriptionGuide.stepApiKeyDesc') }}</small>
                  </span>
                </li>
                <li>
                  <span class="redeem-subscription-guide-step-icon">
                    <Icon name="clock" size="sm" />
                  </span>
                  <span>
                    <strong>{{ t('redeem.subscriptionGuide.stepQuotaTitle') }}</strong>
                    <small>{{ t('redeem.subscriptionGuide.stepQuotaDesc') }}</small>
                  </span>
                </li>
                <li>
                  <span class="redeem-subscription-guide-step-icon">
                    <Icon name="terminal" size="sm" />
                  </span>
                  <span>
                    <strong>{{ t('redeem.subscriptionGuide.stepUsageTitle') }}</strong>
                    <small>{{ t('redeem.subscriptionGuide.stepUsageDesc') }}</small>
                  </span>
                </li>
              </ol>
            </div>

            <div class="redeem-subscription-guide-footer">
              <button
                type="button"
                data-test="subscription-guide-dismiss"
                class="btn btn-secondary"
                @click="closeSubscriptionRedeemGuide"
              >
                {{ t('redeem.subscriptionGuide.acknowledge') }}
              </button>
              <button
                type="button"
                data-test="subscription-guide-go-keys"
                class="btn btn-primary"
                @click="goToApiKeysFromSubscriptionGuide"
              >
                <Icon name="key" size="sm" class="mr-2" />
                {{ t('redeem.subscriptionGuide.goToKeys') }}
              </button>
            </div>
          </div>
        </div>
      </transition>

      <!-- Success Message -->
      <transition name="fade">
        <div
          v-if="redeemResult"
          class="brand-surface redeem-feedback-panel redeem-feedback-success"
        >
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-100 dark:bg-emerald-900/30"
              >
                <Icon name="checkCircle" size="md" class="text-emerald-600 dark:text-emerald-400" />
              </div>
              <div class="flex-1">
                <h3 class="text-sm font-semibold text-emerald-800 dark:text-emerald-300">
                  {{ t('redeem.redeemSuccess') }}
                </h3>
                <div class="mt-2 text-sm text-emerald-700 dark:text-emerald-400">
                  <p>{{ redeemResult.message }}</p>
                  <div class="mt-3 space-y-1">
                    <p v-if="redeemResult.type === 'balance'" class="font-medium">
                      {{ t('redeem.added') }}: ${{ redeemResult.value.toFixed(2) }}
                    </p>
                    <p v-else-if="redeemResult.type === 'concurrency'" class="font-medium">
                      {{ t('redeem.added') }}: {{ redeemResult.value }}
                      {{ t('redeem.concurrentRequests') }}
                    </p>
                    <p v-else-if="redeemResult.type === 'subscription'" class="font-medium">
                      {{ t('redeem.subscriptionAssigned') }}
                      <span v-if="redeemResult.group_name"> - {{ redeemResult.group_name }}</span>
                      <span v-if="redeemResult.validity_days">
                        ({{
                          t('redeem.subscriptionDays', { days: redeemResult.validity_days })
                        }})</span
                      >
                    </p>
                    <p v-if="redeemResult.new_balance !== undefined">
                      {{ t('redeem.newBalance') }}:
                      <span class="font-semibold">${{ redeemResult.new_balance.toFixed(2) }}</span>
                    </p>
                    <p v-if="redeemResult.new_concurrency !== undefined">
                      {{ t('redeem.newConcurrency') }}:
                      <span class="font-semibold"
                        >{{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}</span
                      >
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </transition>

      <!-- Error Message -->
      <transition name="fade">
        <div
          v-if="errorMessage"
          class="brand-surface redeem-feedback-panel redeem-feedback-error"
        >
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-red-100 dark:bg-red-900/30"
              >
                <Icon
                  name="exclamationCircle"
                  size="md"
                  class="text-red-600 dark:text-red-400"
                />
              </div>
              <div class="flex-1">
                <h3 class="text-sm font-semibold text-red-800 dark:text-red-300">
                  {{ t('redeem.redeemFailed') }}
                </h3>
                <p class="mt-2 text-sm text-red-700 dark:text-red-400">
                  {{ errorMessage }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </transition>

      <!-- Information Card -->
      <section class="brand-surface brand-rail redeem-info-panel">
        <div class="p-5 pl-7 sm:p-6 sm:pl-8">
          <div class="flex items-start gap-4">
            <div class="redeem-info-icon">
              <Icon name="infoCircle" size="md" class="text-primary-600 dark:text-primary-400" />
            </div>
            <div class="flex-1">
              <h3 class="text-sm font-semibold text-primary-800 dark:text-primary-300">
                {{ t('redeem.aboutCodes') }}
              </h3>
              <ul
                class="mt-2 list-inside list-disc space-y-1 text-sm text-primary-700 dark:text-primary-400"
              >
                <li>{{ t('redeem.codeRule1') }}</li>
                <li>{{ t('redeem.codeRule2') }}</li>
                <li>
                  {{ t('redeem.codeRule3') }}
                  <span
                    v-if="contactInfo"
                    class="ml-1.5 inline-flex items-center rounded-md bg-primary-200/50 px-2 py-0.5 text-xs font-medium text-primary-800 dark:bg-primary-800/40 dark:text-primary-200"
                  >
                    {{ contactInfo }}
                  </span>
                </li>
                <li>{{ t('redeem.codeRule4') }}</li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- Lottery balance rewards -->
      <section
        v-if="!loadingLotteryHistory && lotteryBalanceHistory.length > 0"
        data-test="lottery-balance-history"
        class="brand-surface redeem-history-panel"
      >
        <div class="redeem-history-header">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('redeem.lotteryBalanceHistory') }}
          </h2>
        </div>
        <div class="space-y-3 p-6">
          <div
            v-for="item in lotteryBalanceHistory"
            :key="`lottery-${item.id}`"
            class="redeem-history-row"
          >
            <div class="redeem-history-card">
              <div class="flex items-center gap-4">
                <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-100 dark:bg-emerald-900/30">
                  <Icon name="dollar" size="md" class="text-emerald-600 dark:text-emerald-400" />
                </div>
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ item.prize_name }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">
                    {{ formatDateTime(item.created_at) }}
                  </p>
                </div>
              </div>
              <div class="text-right">
                <p class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">
                  +${{ Number(item.balance_amount || 0).toFixed(2) }}
                </p>
                <p class="text-xs text-gray-400 dark:text-dark-500">
                  {{ t('redeem.lotteryBalanceHistoryDetail') }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Recent Activity -->
      <section class="brand-surface redeem-history-panel">
        <div class="redeem-history-header">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('redeem.recentActivity') }}
          </h2>
        </div>
        <div class="p-6">
          <!-- Loading State -->
          <div v-if="loadingHistory" class="flex items-center justify-center py-8">
            <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
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
          </div>

          <!-- History List -->
          <div v-else-if="history.length > 0" class="space-y-3">
            <div
              v-for="item in history"
              :key="item.id"
              class="redeem-history-row"
            >
              <div class="redeem-history-card">
                <div class="flex items-center gap-4">
                  <div
                    :class="[
                      'flex h-10 w-10 items-center justify-center rounded-xl',
                      isBalanceType(item.type)
                        ? item.value >= 0
                          ? 'bg-emerald-100 dark:bg-emerald-900/30'
                          : 'bg-red-100 dark:bg-red-900/30'
                        : isSubscriptionType(item.type)
                          ? 'bg-purple-100 dark:bg-purple-900/30'
                          : item.value >= 0
                            ? 'bg-blue-100 dark:bg-blue-900/30'
                            : 'bg-orange-100 dark:bg-orange-900/30'
                    ]"
                  >
                  <!-- 余额类型图标 -->
                  <Icon
                    v-if="isBalanceType(item.type)"
                    name="dollar"
                    size="md"
                    :class="
                      item.value >= 0
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-red-600 dark:text-red-400'
                    "
                  />
                  <!-- 订阅类型图标 -->
                  <Icon
                    v-else-if="isSubscriptionType(item.type)"
                    name="badge"
                    size="md"
                    class="text-purple-600 dark:text-purple-400"
                  />
                  <!-- 并发类型图标 -->
                  <Icon
                    v-else
                    name="bolt"
                    size="md"
                    :class="
                      item.value >= 0
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-orange-600 dark:text-orange-400'
                    "
                  />
                  </div>
                  <div>
                    <p class="text-sm font-medium text-gray-900 dark:text-white">
                      {{ getHistoryItemTitle(item) }}
                    </p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">
                      {{ formatDateTime(item.used_at) }}
                    </p>
                  </div>
                </div>
                <div class="text-right">
                  <p
                    :class="[
                      'text-sm font-semibold',
                      isBalanceType(item.type)
                        ? item.value >= 0
                          ? 'text-emerald-600 dark:text-emerald-400'
                          : 'text-red-600 dark:text-red-400'
                        : isSubscriptionType(item.type)
                          ? 'text-purple-600 dark:text-purple-400'
                          : item.value >= 0
                            ? 'text-blue-600 dark:text-blue-400'
                            : 'text-orange-600 dark:text-orange-400'
                    ]"
                  >
                    {{ formatHistoryValue(item) }}
                  </p>
                  <p v-if="shouldShowHistoryCode(item)" class="font-mono text-xs text-gray-400 dark:text-dark-500">
                    {{ item.code.slice(0, 8) }}...
                  </p>
                  <p v-else class="text-xs text-gray-400 dark:text-dark-500">
                    {{ getHistoryItemDetail(item) }}
                  </p>
                  <!-- Display notes for admin adjustments -->
                  <p
                    v-if="item.notes"
                    class="mt-1 text-xs text-gray-500 dark:text-dark-400 italic max-w-[200px] truncate"
                    :title="item.notes"
                  >
                    {{ item.notes }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- Empty State -->
          <div v-else class="empty-state py-8">
            <div
              class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-800"
            >
              <Icon name="clock" size="xl" class="text-gray-400 dark:text-dark-500" />
            </div>
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('redeem.historyWillAppear') }}
            </p>
          </div>

          <Pagination
            v-if="!loadingHistory && historyPagination.total > historyPagination.page_size"
            class="mt-4 overflow-hidden rounded-lg border border-slate-200 dark:border-dark-700"
            :page="historyPagination.page"
            :total="historyPagination.total"
            :page-size="historyPagination.page_size"
            :page-size-options="redeemPageSizeOptions"
            @update:page="handleHistoryPageChange"
            @update:pageSize="handleHistoryPageSizeChange"
          />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, authAPI, type GeneratedRedeemCode, type RedeemHistoryItem } from '@/api'
import { lotteryAPI, type LotteryDraw } from '@/api/lottery'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)
const availableBalance = computed(() => user.value?.balance || 0)
const canGenerateBalanceTransferCodes = computed(
  () => user.value?.balance_redeem_code_enabled === true
)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_name?: string
  validity_days?: number
} | null>(null)
const errorMessage = ref('')
const subscriptionRedeemGuideStoragePrefix = 'passionapi.subscriptionRedeemGuideSeen'
const showSubscriptionRedeemGuide = ref(false)
const subscriptionGuideLogoLoadFailed = ref(false)
const subscriptionGuideContext = ref<{
  groupName?: string
  validityDays?: number
} | null>(null)
const subscriptionRedeemGuideStorageKey = computed(() => {
  return `${subscriptionRedeemGuideStoragePrefix}.${user.value?.id ?? 'guest'}`
})
const subscriptionGuideGroupName = computed(() => {
  return subscriptionGuideContext.value?.groupName || t('redeem.subscriptionGuide.defaultGroup')
})
const subscriptionGuideValidityDays = computed(() => {
  return subscriptionGuideContext.value?.validityDays || 0
})
const subscriptionGuideLogoSrc = computed(() => {
  const configuredLogo = appStore.effectiveSiteLogo || appStore.siteLogo || '/logo.svg'
  return sanitizeUrl(configuredLogo, { allowRelative: true, allowDataUrl: true }) || '/logo.svg'
})
const showSubscriptionGuideLogo = computed(() => {
  return !subscriptionGuideLogoLoadFailed.value && Boolean(subscriptionGuideLogoSrc.value)
})

watch(subscriptionGuideLogoSrc, () => {
  subscriptionGuideLogoLoadFailed.value = false
})

// History data
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const lotteryBalanceHistory = ref<LotteryDraw[]>([])
const loadingLotteryHistory = ref(false)
const contactInfo = ref('')
const redeemPageSizeOptions = [10, 20, 50]
const historyPagination = reactive({
  page: 1,
  page_size: 10,
  total: 0
})
const emptyResponseHistoryType = 'empty_response'

const transferForm = reactive({
  amount: '',
  count: 1,
  expires_in_days: 30,
  notes: '',
  single_use_per_user: false
})
const generatingTransferCode = ref(false)
const generatedCodeResults = ref<GeneratedRedeemCode[]>([])
const showGeneratedCodesModal = ref(false)
const generatedCodes = ref<GeneratedRedeemCode[]>([])
const loadingGeneratedCodes = ref(false)
const deletingGeneratedCodeIds = ref<number[]>([])
const selectedGeneratedCodeIds = ref<number[]>([])
const batchDeletingGeneratedCodes = ref(false)
const transferErrorMessage = ref('')
const generatedPagination = reactive({
  page: 1,
  page_size: 10,
  total: 0
})

// Helper functions for history display
const isBalanceType = (type: string) => {
  return (
    type === 'balance' ||
    type === 'admin_balance' ||
    type === 'checkin_reward' ||
    type === emptyResponseHistoryType
  )
}

const isSubscriptionType = (type: string) => {
  return type === 'subscription'
}

const isAdminAdjustment = (type: string) => {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

const isEmptyResponseRefund = (type: string) => {
  return type === emptyResponseHistoryType
}

const shouldShowHistoryCode = (item: RedeemHistoryItem) => {
  return !isAdminAdjustment(item.type) && !isEmptyResponseRefund(item.type)
}

const getHistoryItemDetail = (item: RedeemHistoryItem) => {
  if (isEmptyResponseRefund(item.type)) {
    return t('redeem.emptyResponseRefundDetail')
  }
  if (isAdminAdjustment(item.type)) {
    return t('redeem.adminAdjustment')
  }
  return item.code.slice(0, 8) + '...'
}

const getHistoryItemTitle = (item: RedeemHistoryItem) => {
  if (item.type === 'balance') {
    return t('redeem.balanceAddedRedeem')
  } else if (item.type === 'admin_balance') {
    return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
  } else if (item.type === 'checkin_reward') {
    return t('redeem.checkinReward')
  } else if (isEmptyResponseRefund(item.type)) {
    return t('redeem.emptyResponseRefund')
  } else if (item.type === 'concurrency') {
    return t('redeem.concurrencyAddedRedeem')
  } else if (item.type === 'admin_concurrency') {
    return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
  } else if (item.type === 'subscription') {
    return t('redeem.subscriptionAssigned')
  }
  return t('common.unknown')
}

const formatHistoryValue = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  } else if (isSubscriptionType(item.type)) {
    // 订阅类型显示有效天数和分组名称
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}${t('redeem.days')} - ${groupName}` : `${days}${t('redeem.days')}`
  } else {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}${item.value} ${t('redeem.requests')}`
  }
}

const fetchHistory = async () => {
  loadingHistory.value = true
  try {
    const response = await redeemAPI.getHistory({
      page: historyPagination.page,
      page_size: historyPagination.page_size
    })
    if (Array.isArray(response)) {
      history.value = response
      historyPagination.total = response.length
      historyPagination.page = 1
      historyPagination.page_size = Math.max(response.length, historyPagination.page_size)
    } else {
      history.value = response.items || []
      historyPagination.total = response.total
      historyPagination.page = response.page
      historyPagination.page_size = response.page_size
    }
  } catch (error) {
    console.error('Failed to fetch history:', error)
  } finally {
    loadingHistory.value = false
  }
}

const fetchLotteryBalanceHistory = async () => {
  loadingLotteryHistory.value = true
  try {
    const response = await lotteryAPI.history({ page: 1, page_size: 20 })
    const draws = Array.isArray(response) ? response : response.items || []
    lotteryBalanceHistory.value = draws.filter((item) => item.prize_type === 'balance')
  } catch (error) {
    // Lottery history is an optional activity panel; it must not block redeem history.
    console.error('Failed to fetch lottery balance history:', error)
    lotteryBalanceHistory.value = []
  } finally {
    loadingLotteryHistory.value = false
  }
}

const handleHistoryPageChange = (page: number) => {
  historyPagination.page = page
  fetchHistory()
}

const handleHistoryPageSizeChange = (pageSize: number) => {
  historyPagination.page_size = pageSize
  historyPagination.page = 1
  fetchHistory()
}

const fetchGeneratedCodes = async () => {
  if (!canGenerateBalanceTransferCodes.value) {
    generatedCodes.value = []
    generatedPagination.total = 0
    generatedPagination.page = 1
    return
  }

  loadingGeneratedCodes.value = true
  try {
    const response = await redeemAPI.getUserGenerated({
      page: generatedPagination.page,
      page_size: generatedPagination.page_size
    })
    generatedCodes.value = response.items
    generatedPagination.total = response.total
    generatedPagination.page = response.page
    generatedPagination.page_size = response.page_size
  } catch (error) {
    console.error('Failed to fetch generated redeem codes:', error)
    appStore.showError(t('redeem.balanceTransfer.failedToLoad'))
  } finally {
    loadingGeneratedCodes.value = false
  }
}

const handleGeneratedPageChange = (page: number) => {
  generatedPagination.page = page
  fetchGeneratedCodes()
}

const handleGeneratedPageSizeChange = (pageSize: number) => {
  generatedPagination.page_size = pageSize
  generatedPagination.page = 1
  fetchGeneratedCodes()
}

const handleGenerateBalanceTransferCode = async () => {
  const amount = Number(transferForm.amount)
  const count = Number(transferForm.count)
  const expiresInDays = Number(transferForm.expires_in_days)
  transferErrorMessage.value = ''

  if (!Number.isFinite(amount) || amount <= 0) {
    const message = t('redeem.balanceTransfer.invalidAmount')
    transferErrorMessage.value = message
    appStore.showError(message)
    return
  }
  if (!Number.isInteger(count) || count < 1 || count > 100) {
    const message = t('redeem.balanceTransfer.invalidCount')
    transferErrorMessage.value = message
    appStore.showError(message)
    return
  }
  if (amount * count > availableBalance.value) {
    const message = t('redeem.balanceTransfer.insufficientBalance')
    transferErrorMessage.value = message
    appStore.showError(message)
    return
  }
  if (!Number.isInteger(expiresInDays) || expiresInDays < 1 || expiresInDays > 3650) {
    const message = t('redeem.balanceTransfer.invalidExpiry')
    transferErrorMessage.value = message
    appStore.showError(message)
    return
  }

  generatingTransferCode.value = true
  try {
    const codes = await redeemAPI.generateUserBalanceTransferCodes({
      amount,
      count,
      expires_in_days: expiresInDays,
      notes: transferForm.notes.trim(),
      single_use_per_user: transferForm.single_use_per_user
    })
    generatedCodeResults.value = codes
    showGeneratedCodesModal.value = codes.length > 0
    transferForm.amount = ''
    transferForm.notes = ''
    await authStore.refreshUser()
    generatedPagination.page = 1
    await fetchGeneratedCodes()
    appStore.showSuccess(t('redeem.balanceTransfer.generated'))
  } catch (error: any) {
    const message = extractApiErrorMessage(error, t('redeem.balanceTransfer.failedToGenerate'))
    transferErrorMessage.value = message
    appStore.showError(message)
  } finally {
    generatingTransferCode.value = false
  }
}

const canDeleteGeneratedCode = (item: GeneratedRedeemCode) => {
  return item.used_by == null && (item.status === 'unused' || item.status === 'expired')
}

const deletableGeneratedCodes = computed(() => generatedCodes.value.filter(canDeleteGeneratedCode))

const allDeletableGeneratedCodesSelected = computed(() => {
  if (deletableGeneratedCodes.value.length === 0) {
    return false
  }
  const selected = new Set(selectedGeneratedCodeIds.value)
  return deletableGeneratedCodes.value.every((item) => selected.has(item.id))
})

const generatedCodeResultsText = computed(() => {
  return generatedCodeResults.value.map((item) => item.code).join('\n')
})

const isDeletingGeneratedCode = (id: number) => {
  return deletingGeneratedCodeIds.value.includes(id)
}

const handleDeleteGeneratedCode = async (item: GeneratedRedeemCode) => {
  if (!canDeleteGeneratedCode(item) || isDeletingGeneratedCode(item.id)) {
    return
  }
  if (!window.confirm(t('redeem.balanceTransfer.deleteConfirm'))) {
    return
  }

  deletingGeneratedCodeIds.value = [...deletingGeneratedCodeIds.value, item.id]
  try {
    await redeemAPI.deleteUserGenerated(item.id)
    await authStore.refreshUser()
    generatedPagination.page = 1
    await fetchGeneratedCodes()
    selectedGeneratedCodeIds.value = selectedGeneratedCodeIds.value.filter((id) => id !== item.id)
    appStore.showSuccess(t('redeem.balanceTransfer.deleted'))
  } catch (error: any) {
    const message = extractApiErrorMessage(error, t('redeem.balanceTransfer.failedToDelete'))
    appStore.showError(message)
  } finally {
    deletingGeneratedCodeIds.value = deletingGeneratedCodeIds.value.filter((id) => id !== item.id)
  }
}

const handleSelectAllGeneratedCodes = (event: Event) => {
  const checked = (event.target as HTMLInputElement).checked
  if (!checked) {
    selectedGeneratedCodeIds.value = []
    return
  }
  selectedGeneratedCodeIds.value = deletableGeneratedCodes.value.map((item) => item.id)
}

const handleDeleteSelectedGeneratedCodes = async () => {
  const deletableIDSet = new Set(deletableGeneratedCodes.value.map((item) => item.id))
  const ids = selectedGeneratedCodeIds.value.filter((id) => deletableIDSet.has(id))
  if (ids.length === 0 || batchDeletingGeneratedCodes.value) {
    return
  }
  if (!window.confirm(t('redeem.balanceTransfer.batchDeleteConfirm'))) {
    return
  }

  batchDeletingGeneratedCodes.value = true
  try {
    await redeemAPI.deleteUserGeneratedBatch(ids)
    selectedGeneratedCodeIds.value = []
    await authStore.refreshUser()
    generatedPagination.page = 1
    await fetchGeneratedCodes()
    appStore.showSuccess(t('redeem.balanceTransfer.batchDeleted'))
  } catch (error: any) {
    const message = extractApiErrorMessage(error, t('redeem.balanceTransfer.failedToBatchDelete'))
    appStore.showError(message)
  } finally {
    batchDeletingGeneratedCodes.value = false
  }
}

const copyTransferCode = async (code: string) => {
  try {
    await navigator.clipboard.writeText(code)
    appStore.showSuccess(t('redeem.balanceTransfer.copied'))
  } catch (error) {
    console.error('Failed to copy generated redeem code:', error)
    appStore.showError(t('redeem.balanceTransfer.copyFailed'))
  }
}

const closeGeneratedCodesModal = () => {
  showGeneratedCodesModal.value = false
}

const copyGeneratedCodeResults = async () => {
  try {
    await navigator.clipboard.writeText(generatedCodeResultsText.value)
    appStore.showSuccess(t('redeem.balanceTransfer.copied'))
  } catch (error) {
    console.error('Failed to copy generated redeem codes:', error)
    appStore.showError(t('redeem.balanceTransfer.copyFailed'))
  }
}

const downloadGeneratedCodeResults = () => {
  const blob = new Blob([generatedCodeResultsText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

const getGeneratedStatusLabel = (status: string) => {
  const labels: Record<string, string> = {
    unused: t('redeem.balanceTransfer.status.unused'),
    used: t('redeem.balanceTransfer.status.used'),
    expired: t('redeem.balanceTransfer.status.expired'),
    disabled: t('redeem.balanceTransfer.status.disabled'),
    active: t('redeem.balanceTransfer.status.active')
  }
  return labels[status] || status
}

const getGeneratedStatusClass = (status: string) => {
  const base =
    'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-semibold'
  if (status === 'unused' || status === 'active') {
    return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300`
  }
  if (status === 'used') {
    return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300`
  }
  return `${base} bg-slate-100 text-slate-600 dark:bg-dark-800 dark:text-dark-300`
}

const formatGeneratedExpiry = (item: GeneratedRedeemCode) => {
  return item.expires_at ? formatDateTime(item.expires_at) : t('redeem.balanceTransfer.neverExpires')
}

const hasSeenSubscriptionRedeemGuide = () => {
  try {
    return window.localStorage.getItem(subscriptionRedeemGuideStorageKey.value) === '1'
  } catch (error) {
    console.warn('Failed to read subscription redeem guide state:', error)
    return false
  }
}

const rememberSubscriptionRedeemGuide = () => {
  try {
    window.localStorage.setItem(subscriptionRedeemGuideStorageKey.value, '1')
  } catch (error) {
    console.warn('Failed to save subscription redeem guide state:', error)
  }
}

const maybeShowSubscriptionRedeemGuide = (result: {
  type: string
  value: number
  group_name?: string
  validity_days?: number
}) => {
  if (result.type !== 'subscription' || hasSeenSubscriptionRedeemGuide()) {
    return
  }

  subscriptionGuideContext.value = {
    groupName: result.group_name,
    validityDays: result.validity_days || Math.round(result.value)
  }
  showSubscriptionRedeemGuide.value = true
}

const closeSubscriptionRedeemGuide = () => {
  rememberSubscriptionRedeemGuide()
  showSubscriptionRedeemGuide.value = false
}

const goToApiKeysFromSubscriptionGuide = () => {
  closeSubscriptionRedeemGuide()
  router.push('/keys')
}

const handleSubscriptionGuideLogoError = () => {
  subscriptionGuideLogoLoadFailed.value = true
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    await authStore.refreshUser()

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
      maybeShowSubscriptionRedeemGuide(result)
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh history
    historyPagination.page = 1
    await fetchHistory()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    const message =
      extractApiErrorCode(error) === 'REDEEM_BATCH_USER_LIMIT'
        ? t('redeem.batchSingleUse')
        : extractApiErrorMessage(error, t('redeem.failedToRedeem'))
    errorMessage.value = message
    appStore.showError(message)
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchHistory()
  fetchLotteryBalanceHistory()
  if (canGenerateBalanceTransferCodes.value) {
    fetchGeneratedCodes()
  }
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})

watch(canGenerateBalanceTransferCodes, (enabled) => {
  if (enabled) {
    generatedPagination.page = 1
    fetchGeneratedCodes()
  } else {
    generatedCodes.value = []
    generatedPagination.total = 0
    generatedPagination.page = 1
    generatedCodeResults.value = []
    showGeneratedCodesModal.value = false
    selectedGeneratedCodeIds.value = []
  }
})

watch(generatedCodes, (codes) => {
  const availableIds = new Set(codes.filter(canDeleteGeneratedCode).map((item) => item.id))
  selectedGeneratedCodeIds.value = selectedGeneratedCodeIds.value.filter((id) => availableIds.has(id))
})
</script>

<style scoped>
.redeem-shell {
  padding-bottom: 2rem;
}

.redeem-balance-card {
  position: relative;
  overflow: hidden;
  border-radius: 1.5rem;
  padding: 1.65rem 1.75rem;
  color: white;
  background:
    radial-gradient(circle at 14% 8%, rgba(255, 255, 255, 0.24), transparent 28%),
    radial-gradient(circle at 90% 0%, rgba(6, 182, 212, 0.42), transparent 32%),
    radial-gradient(circle at 72% 86%, rgba(255, 255, 255, 0.13), transparent 26%),
    linear-gradient(135deg, var(--brand-700), var(--brand-500) 54%, var(--brand-cyan));
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.32) inset,
    0 24px 58px rgba(37, 99, 235, 0.24);
}

.redeem-balance-card::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(115deg, rgba(255, 255, 255, 0.2), transparent 34%),
    linear-gradient(90deg, transparent 64%, rgba(255, 255, 255, 0.09)),
    linear-gradient(180deg, transparent, rgba(15, 23, 42, 0.12));
}

.redeem-balance-orbit {
  position: absolute;
  right: -5rem;
  top: -5.5rem;
  width: 15rem;
  height: 15rem;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  background: rgba(255, 255, 255, 0.08);
}

.redeem-balance-content {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 2rem;
  min-height: 8.35rem;
}

.redeem-balance-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 1.35rem;
}

.redeem-balance-icon {
  position: relative;
  display: inline-flex;
  height: 4.65rem;
  width: 4.65rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 1.35rem;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.17);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.26) inset,
    0 16px 34px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(12px);
}

.redeem-eyebrow {
  position: relative;
  font-size: 0.875rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.8);
}

.redeem-balance-value {
  position: relative;
  margin-top: 0.35rem;
  font-size: clamp(2.7rem, 5.8vw, 4.45rem);
  font-weight: 800;
  line-height: 0.95;
  letter-spacing: 0;
  font-variant-numeric: tabular-nums;
  text-shadow: 0 16px 34px rgba(15, 23, 42, 0.12);
}

.redeem-concurrency-metric {
  position: relative;
  display: flex;
  min-width: 11.5rem;
  align-items: center;
  gap: 0.85rem;
  padding-left: 1.9rem;
  color: rgba(255, 255, 255, 0.82);
}

.redeem-concurrency-metric::before {
  content: '';
  position: absolute;
  bottom: 0.15rem;
  left: 0;
  top: 0.15rem;
  width: 1px;
  background: linear-gradient(
    180deg,
    transparent,
    rgba(255, 255, 255, 0.34),
    transparent
  );
}

.redeem-concurrency-icon {
  position: relative;
  display: flex;
  height: 3rem;
  width: 3rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.16);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.24) inset,
    0 14px 28px rgba(15, 23, 42, 0.1);
  backdrop-filter: blur(10px);
}

.redeem-concurrency-label,
.redeem-concurrency-row {
  position: relative;
  z-index: 1;
}

.redeem-concurrency-label {
  font-size: 0.78rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.72);
}

.redeem-concurrency-row {
  margin-top: 0.18rem;
  display: flex;
  align-items: baseline;
  gap: 0.45rem;
}

.redeem-concurrency-value {
  color: white;
  font-size: 2.25rem;
  font-weight: 800;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.redeem-concurrency-unit {
  font-size: 0.78rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.72);
}

.redeem-code-panel,
.redeem-info-panel,
.redeem-history-panel,
.redeem-feedback-panel {
  border-radius: 1.25rem;
}

.redeem-form {
  display: grid;
  gap: 1.25rem;
}

.redeem-panel-header {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.redeem-panel-icon {
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
}

.redeem-input-wrap {
  position: relative;
}

.redeem-input-icon {
  pointer-events: none;
  position: absolute;
  bottom: 0;
  left: 0;
  top: 0;
  display: flex;
  align-items: center;
  padding-left: 1rem;
  color: var(--brand-500);
}

.redeem-input {
  width: 100%;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.88);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(239, 246, 255, 0.74)),
    white;
  padding: 0.95rem 1rem 0.95rem 3rem;
  font-size: 1rem;
  font-weight: 600;
  color: rgb(15, 23, 42);
  outline: none;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background-color 180ms ease;
}

.redeem-input::placeholder {
  color: rgb(148, 163, 184);
  font-weight: 500;
}

.redeem-input:focus {
  border-color: rgba(var(--brand-rgb), 0.58);
  box-shadow:
    0 0 0 3px rgba(var(--brand-rgb), 0.12),
    0 14px 34px rgba(37, 99, 235, 0.1);
}

.redeem-input:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.redeem-submit {
  min-height: 3.15rem;
  border-radius: 1rem;
}

.redeem-transfer-panel {
  border-radius: 1.25rem;
}

.redeem-transfer-form {
  margin-top: 1.25rem;
  display: grid;
  gap: 1rem;
}

.redeem-transfer-label {
  display: block;
  font-size: 0.78rem;
  font-weight: 700;
  color: rgb(71, 85, 105);
}

.redeem-transfer-field {
  width: 100%;
  border-radius: 0.9rem;
  border: 1px solid rgba(191, 219, 254, 0.88);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.88)),
    white;
  padding: 0.78rem 0.9rem;
  font-size: 0.95rem;
  font-weight: 600;
  color: rgb(15, 23, 42);
  outline: none;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background-color 180ms ease;
}

.redeem-transfer-field::placeholder {
  color: rgb(148, 163, 184);
  font-weight: 500;
}

.redeem-transfer-field:focus {
  border-color: rgba(var(--brand-rgb), 0.58);
  box-shadow:
    0 0 0 3px rgba(var(--brand-rgb), 0.12),
    0 12px 28px rgba(37, 99, 235, 0.08);
}

.redeem-transfer-toggle {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(191, 219, 254, 0.76);
  background: rgba(248, 250, 252, 0.78);
  padding: 0.8rem 0.9rem;
  color: rgb(71, 85, 105);
}

.redeem-transfer-toggle input {
  margin-top: 0.2rem;
  height: 1rem;
  width: 1rem;
  flex-shrink: 0;
  accent-color: var(--brand-600);
}

.redeem-transfer-toggle strong,
.redeem-transfer-toggle small {
  display: block;
}

.redeem-transfer-toggle strong {
  font-size: 0.88rem;
  font-weight: 700;
  color: rgb(30, 41, 59);
}

.redeem-transfer-toggle small {
  margin-top: 0.15rem;
  font-size: 0.78rem;
  line-height: 1.35;
  color: rgb(100, 116, 139);
}

.redeem-transfer-field:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.redeem-transfer-submit {
  min-height: 2.75rem;
  border-radius: 0.9rem;
}

.redeem-generated-list {
  margin-top: 1.25rem;
  border-top: 1px solid rgba(191, 219, 254, 0.48);
  padding-top: 1rem;
}

.redeem-generated-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.redeem-generated-toolbar-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}

.redeem-generated-select-all {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  gap: 0.45rem;
  border: 1px solid rgba(191, 219, 254, 0.86);
  border-radius: 0.85rem;
  background: rgba(248, 250, 252, 0.92);
  padding: 0 0.75rem;
  font-size: 0.8rem;
  font-weight: 700;
  color: rgb(51, 65, 85);
}

.redeem-generated-select-all input,
.redeem-generated-row-select input {
  height: 1rem;
  width: 1rem;
  flex-shrink: 0;
  accent-color: var(--brand-500);
}

.redeem-generated-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgba(191, 219, 254, 0.82);
  border-radius: 0.95rem;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.9)),
    white;
  padding: 0.8rem 0.9rem;
}

.redeem-generated-row-select,
.redeem-generated-row-select-spacer {
  display: flex;
  width: 1rem;
  flex-shrink: 0;
  justify-content: center;
  padding-top: 0.15rem;
}

.redeem-generated-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.5rem;
}

.redeem-generated-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.46);
  padding: 1rem;
  backdrop-filter: blur(10px);
}

.redeem-generated-modal {
  width: min(100%, 36rem);
  overflow: hidden;
  border: 1px solid rgba(191, 219, 254, 0.86);
  border-radius: 1.25rem;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(239, 246, 255, 0.96)),
    white;
  box-shadow:
    0 28px 64px rgba(15, 23, 42, 0.2),
    0 1px 0 rgba(255, 255, 255, 0.9) inset;
}

.redeem-generated-modal-header,
.redeem-generated-modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.15rem;
}

.redeem-generated-modal-header {
  border-bottom: 1px solid rgba(191, 219, 254, 0.62);
}

.redeem-generated-modal-footer {
  justify-content: flex-end;
  border-top: 1px solid rgba(191, 219, 254, 0.62);
}

.redeem-generated-modal-icon,
.redeem-generated-modal-close {
  display: flex;
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
}

.redeem-generated-modal-icon {
  background: linear-gradient(135deg, var(--brand-500), var(--brand-cyan-500));
  color: white;
}

.redeem-generated-modal-close {
  border: 1px solid rgba(191, 219, 254, 0.72);
  background: rgba(255, 255, 255, 0.82);
  color: rgb(71, 85, 105);
}

.redeem-subscription-guide-modal {
  width: min(100%, 34rem);
  max-height: calc(100vh - 2rem);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(125, 211, 252, 0.8);
  border-radius: 1.25rem;
  background:
    radial-gradient(circle at 12% 0%, rgba(14, 165, 233, 0.13), transparent 32%),
    linear-gradient(135deg, rgba(255, 255, 255, 0.99), rgba(240, 249, 255, 0.96)),
    white;
  box-shadow:
    0 30px 70px rgba(15, 23, 42, 0.22),
    0 1px 0 rgba(255, 255, 255, 0.9) inset;
}

.redeem-subscription-guide-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid rgba(191, 219, 254, 0.66);
  padding: 1.15rem;
}

.redeem-subscription-guide-hero {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 0.95rem;
}

.redeem-subscription-guide-icon {
  display: flex;
  height: 3.2rem;
  width: 3.2rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 1.05rem;
  color: white;
  border: 1px solid rgba(147, 197, 253, 0.7);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.94), rgba(239, 246, 255, 0.88)),
    linear-gradient(135deg, var(--brand-600, #2563eb), var(--brand-cyan-500, #06b6d4));
  box-shadow: 0 14px 30px rgba(37, 99, 235, 0.2);
}

.redeem-subscription-guide-icon > svg {
  color: rgb(37, 99, 235);
}

.redeem-subscription-guide-logo {
  height: 100%;
  width: 100%;
  object-fit: contain;
  padding: 0.32rem;
}

.redeem-subscription-guide-eyebrow {
  font-size: 0.76rem;
  font-weight: 800;
  letter-spacing: 0;
  color: rgb(37, 99, 235);
}

.redeem-subscription-guide-title {
  margin-top: 0.1rem;
  font-size: 1.1rem;
  font-weight: 800;
  line-height: 1.25;
  color: rgb(15, 23, 42);
}

.redeem-subscription-guide-subtitle {
  margin-top: 0.35rem;
  font-size: 0.88rem;
  line-height: 1.55;
  color: rgb(71, 85, 105);
}

.redeem-subscription-guide-body {
  min-height: 0;
  overflow-y: auto;
  padding: 1rem 1.15rem 1.15rem;
}

.redeem-subscription-guide-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
  margin-bottom: 1rem;
}

.redeem-subscription-guide-chip {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  border-radius: 0.8rem;
  border: 1px solid rgba(191, 219, 254, 0.82);
  background: rgba(255, 255, 255, 0.78);
  padding: 0.55rem 0.7rem;
  font-size: 0.8rem;
  color: rgb(71, 85, 105);
}

.redeem-subscription-guide-chip strong {
  min-width: 0;
  word-break: break-word;
  color: rgb(15, 23, 42);
}

.redeem-subscription-guide-steps {
  display: grid;
  gap: 0.75rem;
}

.redeem-subscription-guide-steps li {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 0.8rem;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.78);
  background: rgba(255, 255, 255, 0.82);
  padding: 0.85rem;
}

.redeem-subscription-guide-step-icon {
  display: flex;
  height: 2.15rem;
  width: 2.15rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.75rem;
  color: rgb(37, 99, 235);
  background: rgba(219, 234, 254, 0.84);
}

.redeem-subscription-guide-steps strong,
.redeem-subscription-guide-steps small {
  display: block;
  min-width: 0;
  word-break: break-word;
}

.redeem-subscription-guide-steps strong {
  font-size: 0.9rem;
  font-weight: 800;
  color: rgb(30, 41, 59);
}

.redeem-subscription-guide-steps small {
  margin-top: 0.25rem;
  font-size: 0.82rem;
  line-height: 1.55;
  color: rgb(71, 85, 105);
}

.redeem-subscription-guide-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  border-top: 1px solid rgba(191, 219, 254, 0.66);
  padding: 1rem 1.15rem;
}

.redeem-generated-modal-body {
  padding: 1rem 1.15rem;
}

.redeem-generated-modal-code-list {
  display: grid;
  max-height: min(42vh, 18rem);
  gap: 0.55rem;
  overflow-y: auto;
}

.redeem-generated-modal-code-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgba(191, 219, 254, 0.78);
  border-radius: 0.85rem;
  background: rgba(255, 255, 255, 0.84);
  padding: 0.75rem 0.85rem;
}

.redeem-info-icon {
  display: flex;
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.74);
  background: rgba(239, 246, 255, 0.86);
}

.redeem-history-header {
  border-bottom: 1px solid rgba(191, 219, 254, 0.48);
  padding: 1rem 1.5rem;
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.72), rgba(236, 254, 255, 0.5));
}

.redeem-history-row {
  display: block;
  border: 0 !important;
  outline: 0;
  outline-offset: 0;
  background: transparent;
  box-shadow: none !important;
}

.redeem-history-row:hover,
.redeem-history-row:focus,
.redeem-history-row:focus-within,
.redeem-history-row:focus-visible,
.redeem-history-row:active {
  border: 0 !important;
  outline: 0 !important;
  box-shadow: none !important;
}

.redeem-history-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.88);
  border-radius: 1rem;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.88)),
    white;
  padding: 0.75rem 1rem;
  outline: 0;
  box-shadow:
    0 10px 24px rgba(37, 99, 235, 0.04),
    0 1px 0 rgba(255, 255, 255, 0.9) inset;
  transition:
    background-color 180ms ease,
    background 180ms ease,
    border-color 180ms ease,
    transform 180ms ease,
    box-shadow 180ms ease;
}

.redeem-history-row:hover .redeem-history-card,
.redeem-history-row:focus-within .redeem-history-card,
.redeem-history-card:hover,
.redeem-history-card:focus-within,
.redeem-history-card:active {
  transform: translateY(-1px);
  border-color: rgba(var(--brand-rgb), 0.52);
  outline: 0;
  background:
    linear-gradient(135deg, rgba(var(--brand-rgb), 0.06), rgba(var(--brand-cyan-rgb), 0.16)),
    rgba(255, 255, 255, 0.96);
  box-shadow:
    inset 0 0 0 1px rgba(var(--brand-rgb), 0.32),
    0 0 0 3px rgba(var(--brand-cyan-rgb), 0.16),
    0 12px 28px rgba(37, 99, 235, 0.1);
}

.redeem-feedback-success {
  border-color: rgba(16, 185, 129, 0.3);
  background:
    radial-gradient(circle at 0% 0%, rgba(16, 185, 129, 0.1), transparent 34%),
    linear-gradient(180deg, rgba(240, 253, 244, 0.98), rgba(255, 255, 255, 0.96));
}

.redeem-feedback-error {
  border-color: rgba(239, 68, 68, 0.28);
  background:
    radial-gradient(circle at 0% 0%, rgba(239, 68, 68, 0.1), transparent 34%),
    linear-gradient(180deg, rgba(254, 242, 242, 0.98), rgba(255, 255, 255, 0.96));
}

.dark .redeem-input {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.94), rgba(8, 13, 28, 0.9)),
    rgba(15, 23, 42, 0.86);
  color: white;
}

.dark .redeem-input::placeholder {
  color: rgb(100, 116, 139);
}

.dark .redeem-transfer-label {
  color: rgb(203, 213, 225);
}

.dark .redeem-transfer-field {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.94), rgba(8, 13, 28, 0.9)),
    rgba(15, 23, 42, 0.86);
  color: white;
}

.dark .redeem-transfer-field::placeholder {
  color: rgb(100, 116, 139);
}

.dark .redeem-transfer-toggle {
  border-color: rgba(96, 165, 250, 0.2);
  background: rgba(15, 23, 42, 0.74);
  color: rgb(148, 163, 184);
}

.dark .redeem-transfer-toggle strong {
  color: rgb(226, 232, 240);
}

.dark .redeem-transfer-toggle small {
  color: rgb(148, 163, 184);
}

.dark .redeem-generated-list {
  border-color: rgba(96, 165, 250, 0.16);
}

.dark .redeem-generated-select-all {
  border-color: rgba(96, 165, 250, 0.22);
  background: rgba(15, 23, 42, 0.76);
  color: rgb(203, 213, 225);
}

.dark .redeem-generated-row {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.9), rgba(8, 13, 28, 0.84)),
    rgba(15, 23, 42, 0.86);
}

.dark .redeem-generated-modal {
  border-color: rgba(96, 165, 250, 0.24);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.98), rgba(8, 13, 28, 0.96)),
    rgb(15, 23, 42);
  box-shadow: 0 28px 64px rgba(0, 0, 0, 0.42);
}

.dark .redeem-generated-modal-header,
.dark .redeem-generated-modal-footer {
  border-color: rgba(96, 165, 250, 0.16);
}

.dark .redeem-generated-modal-close,
.dark .redeem-generated-modal-code-row {
  border-color: rgba(96, 165, 250, 0.22);
  background: rgba(15, 23, 42, 0.72);
}

.dark .redeem-generated-modal-close {
  color: rgb(203, 213, 225);
}

.dark .redeem-subscription-guide-modal {
  border-color: rgba(96, 165, 250, 0.24);
  background:
    radial-gradient(circle at 12% 0%, rgba(14, 165, 233, 0.18), transparent 32%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.98), rgba(8, 13, 28, 0.96)),
    rgb(15, 23, 42);
  box-shadow: 0 30px 70px rgba(0, 0, 0, 0.44);
}

.dark .redeem-subscription-guide-header,
.dark .redeem-subscription-guide-footer {
  border-color: rgba(96, 165, 250, 0.16);
}

.dark .redeem-subscription-guide-eyebrow {
  color: rgb(125, 211, 252);
}

.dark .redeem-subscription-guide-title {
  color: white;
}

.dark .redeem-subscription-guide-subtitle,
.dark .redeem-subscription-guide-chip,
.dark .redeem-subscription-guide-steps small {
  color: rgb(148, 163, 184);
}

.dark .redeem-subscription-guide-chip,
.dark .redeem-subscription-guide-steps li {
  border-color: rgba(96, 165, 250, 0.22);
  background: rgba(15, 23, 42, 0.72);
}

.dark .redeem-subscription-guide-chip strong,
.dark .redeem-subscription-guide-steps strong {
  color: rgb(226, 232, 240);
}

.dark .redeem-subscription-guide-step-icon {
  color: rgb(147, 197, 253);
  background: rgba(37, 99, 235, 0.18);
}

.dark .redeem-info-icon {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.16);
}

.dark .redeem-history-header {
  border-color: rgba(96, 165, 250, 0.16);
  background: linear-gradient(135deg, rgba(30, 64, 175, 0.16), rgba(8, 145, 178, 0.08));
}

.dark .redeem-history-row {
  background: transparent;
  box-shadow: none !important;
}

.dark .redeem-history-card {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.9), rgba(8, 13, 28, 0.84)),
    rgba(15, 23, 42, 0.86);
  box-shadow:
    0 12px 28px rgba(0, 0, 0, 0.16),
    0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

.dark .redeem-history-row:hover .redeem-history-card,
.dark .redeem-history-row:focus-within .redeem-history-card,
.dark .redeem-history-card:hover,
.dark .redeem-history-card:focus-within,
.dark .redeem-history-card:active {
  border-color: rgba(96, 165, 250, 0.5);
  background:
    linear-gradient(135deg, rgba(var(--brand-rgb), 0.2), rgba(var(--brand-cyan-rgb), 0.12)),
    rgba(15, 23, 42, 0.82);
  box-shadow:
    inset 0 0 0 1px rgba(96, 165, 250, 0.34),
    0 0 0 3px rgba(6, 182, 212, 0.12),
    0 14px 30px rgba(0, 0, 0, 0.22);
}

.dark .redeem-feedback-success {
  border-color: rgba(52, 211, 153, 0.24);
  background:
    radial-gradient(circle at 0% 0%, rgba(16, 185, 129, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(6, 78, 59, 0.2), rgba(2, 6, 23, 0.9));
}

.dark .redeem-feedback-error {
  border-color: rgba(248, 113, 113, 0.24);
  background:
    radial-gradient(circle at 0% 0%, rgba(239, 68, 68, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(127, 29, 29, 0.2), rgba(2, 6, 23, 0.9));
}

@media (max-width: 640px) {
  .redeem-balance-card {
    border-radius: 1.25rem;
    padding: 1.35rem;
  }

  .redeem-balance-content {
    grid-template-columns: 1fr;
    gap: 1.35rem;
    min-height: 0;
  }

  .redeem-balance-main {
    align-items: flex-start;
    gap: 1rem;
  }

  .redeem-balance-icon {
    height: 3.75rem;
    width: 3.75rem;
    border-radius: 1.1rem;
  }

  .redeem-concurrency-metric {
    width: 100%;
    min-width: 0;
    padding-left: 0;
    padding-top: 1rem;
  }

  .redeem-concurrency-metric::before {
    bottom: auto;
    right: 0;
    top: 0;
    width: auto;
    height: 1px;
    background: linear-gradient(
      90deg,
      rgba(255, 255, 255, 0.42),
      rgba(255, 255, 255, 0.08),
      transparent
    );
  }

  .redeem-history-row {
    align-items: flex-start;
  }

  .redeem-history-card {
    align-items: flex-start;
  }

  .redeem-generated-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .redeem-generated-toolbar-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .redeem-generated-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: start;
  }

  .redeem-generated-actions {
    grid-column: 2;
    width: 100%;
    justify-content: flex-end;
  }

  .redeem-generated-modal {
    max-height: calc(100vh - 2rem);
  }

  .redeem-subscription-guide-modal {
    width: 100%;
    max-height: calc(100vh - 1.5rem);
    border-radius: 1.1rem;
  }

  .redeem-subscription-guide-header {
    padding: 1rem;
  }

  .redeem-subscription-guide-hero {
    gap: 0.75rem;
  }

  .redeem-subscription-guide-icon {
    height: 2.75rem;
    width: 2.75rem;
    border-radius: 0.9rem;
  }

  .redeem-subscription-guide-title {
    font-size: 1rem;
  }

  .redeem-subscription-guide-body {
    padding: 0.9rem 1rem 1rem;
  }

  .redeem-subscription-guide-summary {
    display: grid;
    grid-template-columns: 1fr;
  }

  .redeem-subscription-guide-chip {
    justify-content: space-between;
    align-items: flex-start;
  }

  .redeem-subscription-guide-steps li {
    gap: 0.7rem;
    padding: 0.8rem;
  }

  .redeem-subscription-guide-footer {
    flex-direction: column-reverse;
    padding: 0.9rem 1rem 1rem;
  }

  .redeem-subscription-guide-footer .btn {
    width: 100%;
    justify-content: center;
  }

  .redeem-generated-modal-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .redeem-generated-modal-footer .btn {
    width: 100%;
    justify-content: center;
  }
}

@media (prefers-reduced-motion: reduce) {
  .redeem-history-card,
  .redeem-input {
    transition: none;
  }

  .redeem-history-row:hover .redeem-history-card,
  .redeem-history-row:focus-within .redeem-history-card,
  .redeem-history-card:hover,
  .redeem-history-card:focus-within,
  .redeem-history-card:active {
    transform: none;
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
