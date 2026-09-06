<template>
  <div class="relative" ref="containerRef">
    <button
      ref="triggerRef"
      type="button"
      @click="toggle"
      :disabled="disabled"
      :aria-expanded="isOpen"
      :aria-haspopup="true"
      :id="id"
      :aria-label="ariaLabel ?? 'Select option'"
      :aria-describedby="ariaDescribedby"
      :class="[
        'select-trigger',
        isOpen && 'select-trigger-open',
        error && 'select-trigger-error',
        disabled && 'select-trigger-disabled'
      ]"
      @keydown.down.prevent="onTriggerKeyDown"
      @keydown.up.prevent="onTriggerKeyDown"
    >
      <span class="select-value">
        <slot name="selected" :option="selectedOption">
          {{ selectedLabel }}
        </slot>
      </span>
      <span
        v-if="clearable && hasValue && !disabled"
        class="select-clear"
        role="button"
        tabindex="-1"
        aria-label="Clear selection"
        @click.stop="clearSelection"
        @mousedown.stop
        @keydown.enter.stop.prevent="clearSelection"
      >
        <Icon name="x" size="sm" />
      </span>
      <span class="select-icon">
        <Icon
          name="chevronDown"
          size="md"
          :class="['transition-transform duration-200', isOpen && 'rotate-180']"
        />
      </span>
    </button>

    <!-- Teleport dropdown to body to escape stacking context -->
    <Teleport to="body">
      <Transition name="select-dropdown">
        <div
          v-if="isOpen"
          class="select-dropdown-layer"
          :class="[instanceId, mobileSheetEnabled && 'select-dropdown-layer-mobile']"
          @click.self="closeDropdown"
        >
          <div
            ref="dropdownRef"
            class="select-dropdown-portal"
            :style="dropdownStyle"
            role="listbox"
            tabindex="-1"
            @click.stop
            @mousedown.stop
            @keydown="onDropdownKeyDown"
          >
            <div v-if="mobileSheetEnabled" class="select-sheet-handle" aria-hidden="true" />
            <div v-if="mobileSheetEnabled" class="select-sheet-header">
              <span class="select-sheet-title">{{ ariaLabel ?? placeholderText }}</span>
              <button type="button" class="select-sheet-close" :aria-label="t('common.close')" @click="closeDropdown">
                <Icon name="x" size="sm" />
              </button>
            </div>

            <!-- Search input -->
            <div v-if="isSearchable" class="select-search">
              <Icon name="search" size="sm" class="text-gray-400" />
              <input
                ref="searchInputRef"
                v-model="searchQuery"
                type="text"
                :placeholder="searchPlaceholderText"
                :aria-label="searchPlaceholderText"
                class="select-search-input"
                @click.stop
              />
            </div>

            <!-- Options list -->
            <div class="select-options" ref="optionsListRef">
              <div
                v-for="(option, index) in filteredOptions"
                :key="`${typeof getOptionValue(option)}:${String(getOptionValue(option) ?? '')}`"
                role="option"
                :aria-selected="isSelected(option)"
                :aria-disabled="isOptionDisabled(option)"
                @click.stop="!isOptionDisabled(option) && selectOption(option)"
                @mouseenter="handleOptionMouseEnter(option, index)"
                :class="[
                  'select-option',
                  isGroupHeaderOption(option) && 'select-option-group',
                  isSelected(option) && 'select-option-selected',
                  isOptionDisabled(option) && !isGroupHeaderOption(option) && 'select-option-disabled',
                  focusedIndex === index && !isGroupHeaderOption(option) && 'select-option-focused'
                ]"
              >
                <span
                  v-if="mobileSheetEnabled"
                  class="select-option-icon"
                  :class="getOptionPlatform(option)
                    ? platformBadgeLightClass(getOptionPlatform(option) ?? '')
                    : 'select-option-icon-generic'"
                  aria-hidden="true"
                >
                  <PlatformIcon
                    v-if="getOptionPlatform(option)"
                    :platform="getOptionPlatform(option) as any"
                    size="md"
                  />
                  <Icon v-else :name="getOptionIcon(option)" size="sm" />
                </span>
                <slot name="option" :option="option" :selected="isSelected(option)">
                  <Icon
                    v-if="option._creatable"
                    name="search"
                    size="sm"
                    class="flex-shrink-0 text-gray-400"
                  />
                  <span class="select-option-label" :class="option._creatable && 'italic text-gray-500 dark:text-dark-300'">{{ getOptionLabel(option) }}</span>
                  <Icon
                    v-if="isSelected(option)"
                    name="check"
                    size="sm"
                    class="text-primary-500"
                    :stroke-width="2"
                  />
                </slot>
              </div>

              <!-- Empty state -->
              <div v-if="filteredOptions.length === 0" class="select-empty">
                {{ props.loading ? t('common.loading') : emptyTextDisplay }}
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformBadgeLightClass } from '@/utils/platformColors'

const { t } = useI18n()

// Instance ID for unique click-outside detection
const instanceId = `select-${Math.random().toString(36).substring(2, 9)}`

export interface SelectOption {
  value: string | number | boolean | null
  label: string
  disabled?: boolean
  [key: string]: unknown
}

interface Props {
  modelValue: string | number | boolean | null | undefined
  options: SelectOption[] | Array<Record<string, unknown>>
  placeholder?: string
  disabled?: boolean
  error?: boolean
  searchable?: boolean | 'auto'
  searchPlaceholder?: string
  emptyText?: string
  valueKey?: string
  labelKey?: string
  creatable?: boolean
  creatablePrefix?: string
  clearable?: boolean
  id?: string
  ariaLabel?: string
  ariaDescribedby?: string
  /** 远程搜索模式：输入不在本地过滤 options，而是防抖后 emit('search', query)，由父组件请求数据更新 options */
  remote?: boolean
  /** 远程搜索模式下的加载态：options 为空时下拉显示 loading 文案 */
  loading?: boolean
  /** 在小屏幕上使用底部弹窗样式，桌面端仍使用原下拉菜单；默认开启 */
  mobileSheet?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: string | number | boolean | null): void
  (e: 'change', value: string | number | boolean | null, option: SelectOption | null): void
  (e: 'search', query: string): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  error: false,
  searchable: 'auto',
  creatable: false,
  creatablePrefix: '',
  clearable: false,
  valueKey: 'value',
  labelKey: 'label',
  remote: false,
  loading: false,
  mobileSheet: true
})

const emit = defineEmits<Emits>()

const isOpen = ref(false)
const searchQuery = ref('')
const focusedIndex = ref(-1)
const containerRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const optionsListRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<'bottom' | 'top'>('bottom')
const triggerRect = ref<DOMRect | null>(null)
const isMobileViewport = ref(false)
const dropdownViewportPadding = 8
const dropdownMinimumWidth = 200
const mobileBreakpoint = 767
let previousBodyOverflow = ''
let bodyScrollLocked = false

// i18n placeholders
const placeholderText = computed(() => props.placeholder ?? t('common.selectOption'))
const searchPlaceholderText = computed(() => props.searchPlaceholder ?? t('common.searchPlaceholder'))
const emptyTextDisplay = computed(() => props.emptyText ?? t('common.noOptionsFound'))

// 远程搜索的防抖间隔（对齐 OpenAIFastPolicyUserSelector 的 300ms 惯例）。
const REMOTE_SEARCH_DEBOUNCE_MS = 300
let remoteSearchTimer: ReturnType<typeof setTimeout> | null = null

const isSearchable = computed(() => {
  // 远程搜索模式始终显示搜索框（选项只是服务端结果的一页）。
  if (props.remote) return true
  if (props.searchable === 'auto') return props.options.length > 5
  return props.searchable
})

const mobileSheetEnabled = computed(() => props.mobileSheet && isMobileViewport.value)

// Computed style for teleported dropdown
const dropdownStyle = computed(() => {
  if (mobileSheetEnabled.value) return { zIndex: '100000020' }
  if (!triggerRect.value) return {}

  const rect = triggerRect.value
  const viewportRight = Math.max(dropdownViewportPadding, window.innerWidth - dropdownViewportPadding)
  const left = Math.min(
    Math.max(dropdownViewportPadding, rect.left),
    viewportRight
  )
  const availableWidth = Math.max(0, viewportRight - left)
  const preferredMinWidth = Math.max(dropdownMinimumWidth, rect.width)
  const minWidth = Math.min(preferredMinWidth, availableWidth)
  const style: Record<string, string> = {
    position: 'fixed',
    left: `${left}px`,
    minWidth: `${minWidth}px`,
    maxWidth: `${availableWidth}px`,
    zIndex: '100000020'
  }

  if (dropdownPosition.value === 'top') {
    style.bottom = `${window.innerHeight - rect.top + 4}px`
  } else {
    style.top = `${rect.bottom + 4}px`
  }

  return style
})

const getOptionValue = (option: any): any => {
  if (typeof option === 'object' && option !== null) {
    return option[props.valueKey]
  }
  return option
}

const getOptionLabel = (option: any): string => {
  if (typeof option === 'object' && option !== null) {
    return String(option[props.labelKey] ?? '')
  }
  return String(option ?? '')
}

const getOptionPlatform = (option: any): string | undefined => {
  if (typeof option === 'object' && option !== null && typeof option.platform === 'string') return option.platform
  return undefined
}

const getOptionIcon = (option: any): 'filter' | 'clock' => {
  return option?.icon === 'clock' ? 'clock' : 'filter'
}

const isOptionDisabled = (option: any): boolean => {
  if (typeof option === 'object' && option !== null) {
    return !!option.disabled
  }
  return false
}

const isGroupHeaderOption = (option: any): boolean => {
  if (typeof option === 'object' && option !== null) {
    return option.kind === 'group'
  }
  return false
}

const selectedOption = computed(() => {
  return props.options.find((opt) => getOptionValue(opt) === props.modelValue) || null
})

const selectedLabel = computed(() => {
  if (selectedOption.value) {
    return getOptionLabel(selectedOption.value)
  }
  // In creatable mode, show the raw value if no matching option
  if (props.creatable && props.modelValue) {
    return String(props.modelValue)
  }
  return placeholderText.value
})

const hasValue = computed(
  () => props.modelValue !== null && props.modelValue !== undefined && props.modelValue !== ''
)

const filteredOptions = computed(() => {
  let opts = props.options as any[]
  // 远程搜索模式不在本地过滤（选项即服务端搜索结果的一页）。
  if (isSearchable.value && searchQuery.value && !props.remote) {
    const query = searchQuery.value.toLowerCase()
    opts = opts.filter((opt) => {
      // Match label
      if (getOptionLabel(opt).toLowerCase().includes(query)) return true
      // Also match description if present
      if (opt.description && String(opt.description).toLowerCase().includes(query)) return true
      return false
    })
    // In creatable mode, always prepend a fuzzy search option
    if (props.creatable && searchQuery.value.trim()) {
      const trimmed = searchQuery.value.trim()
      const prefix = props.creatablePrefix || t('common.search')
      opts = [{ [props.valueKey]: trimmed, [props.labelKey]: `${prefix} "${trimmed}"`, _creatable: true }, ...opts]
    }
  }
  return opts
})

const isSelected = (option: any): boolean => {
  return getOptionValue(option) === props.modelValue
}

const findNextEnabledIndex = (startIndex: number): number => {
  const opts = filteredOptions.value
  if (opts.length === 0) return -1
  for (let offset = 0; offset < opts.length; offset++) {
    const idx = (startIndex + offset) % opts.length
    if (!isOptionDisabled(opts[idx])) return idx
  }
  return -1
}

const findPrevEnabledIndex = (startIndex: number): number => {
  const opts = filteredOptions.value
  if (opts.length === 0) return -1
  for (let offset = 0; offset < opts.length; offset++) {
    const idx = (startIndex - offset + opts.length) % opts.length
    if (!isOptionDisabled(opts[idx])) return idx
  }
  return -1
}

const handleOptionMouseEnter = (option: any, index: number) => {
  if (isOptionDisabled(option) || isGroupHeaderOption(option)) return
  focusedIndex.value = index
}

// Update trigger rect periodically while open to follow scroll/resize
const updateTriggerRect = () => {
  if (containerRef.value) {
    triggerRect.value = containerRef.value.getBoundingClientRect()
  }
}

const calculateDropdownPosition = () => {
  if (!containerRef.value) return
  updateTriggerRect()

  nextTick(() => {
    if (!dropdownRef.value || !triggerRect.value) return
    const dropdownHeight = dropdownRef.value.offsetHeight || 240
    const spaceBelow = window.innerHeight - triggerRect.value.bottom
    const spaceAbove = triggerRect.value.top

    if (spaceBelow < dropdownHeight && spaceAbove > dropdownHeight) {
      dropdownPosition.value = 'top'
    } else {
      dropdownPosition.value = 'bottom'
    }
  })
}

const toggle = () => {
  if (props.disabled) return
  isOpen.value = !isOpen.value
}

const closeDropdown = () => {
  if (!isOpen.value) return
  isOpen.value = false
  triggerRef.value?.focus()
}

const updateViewportMode = () => {
  isMobileViewport.value = window.innerWidth <= mobileBreakpoint
}

const updateBodyScrollLock = () => {
  if (mobileSheetEnabled.value && isOpen.value) {
    if (bodyScrollLocked) return
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    bodyScrollLocked = true
    return
  }
  if (bodyScrollLocked) {
    document.body.style.overflow = previousBodyOverflow
    bodyScrollLocked = false
  }
}

const handleGlobalKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && isOpen.value) {
    event.preventDefault()
    closeDropdown()
  }
}

watch(isOpen, (open) => {
  if (open) {
    if (!mobileSheetEnabled.value) calculateDropdownPosition()
    updateBodyScrollLock()
    // Reset focused index to current selection or first item
    if (filteredOptions.value.length === 0) {
      focusedIndex.value = -1
    } else {
      const selectedIdx = filteredOptions.value.findIndex(isSelected)
      const initialIdx = selectedIdx >= 0 ? selectedIdx : 0
      focusedIndex.value = isOptionDisabled(filteredOptions.value[initialIdx])
        ? findNextEnabledIndex(initialIdx + 1)
        : initialIdx
    }

    if (mobileSheetEnabled.value) {
      nextTick(() => dropdownRef.value?.focus({ preventScroll: true }))
    } else if (isSearchable.value) {
      nextTick(() => searchInputRef.value?.focus())
    }
    window.addEventListener('keydown', handleGlobalKeydown)
    // Add scroll listener to update position
    window.addEventListener('scroll', updateTriggerRect, { capture: true, passive: true })
    window.addEventListener('resize', calculateDropdownPosition)
  } else {
    searchQuery.value = ''
    focusedIndex.value = -1
    updateBodyScrollLock()
    // 关闭时取消仍在排队的远程搜索（避免关闭后尾随 emit 一次 search(''))。
    if (remoteSearchTimer) {
      clearTimeout(remoteSearchTimer)
      remoteSearchTimer = null
    }
    window.removeEventListener('scroll', updateTriggerRect, { capture: true })
    window.removeEventListener('resize', calculateDropdownPosition)
    window.removeEventListener('keydown', handleGlobalKeydown)
  }
})

watch(mobileSheetEnabled, () => {
  updateBodyScrollLock()
  if (isOpen.value && !mobileSheetEnabled.value) calculateDropdownPosition()
})

// 远程搜索：输入防抖后交给父组件请求（!isOpen 抑制关闭重置 searchQuery 触发的空 query）。
watch(searchQuery, (query) => {
  if (!props.remote || !isOpen.value) return
  if (remoteSearchTimer) clearTimeout(remoteSearchTimer)
  remoteSearchTimer = setTimeout(() => {
    remoteSearchTimer = null
    emit('search', query.trim())
  }, REMOTE_SEARCH_DEBOUNCE_MS)
})

const selectOption = (option: any) => {
  const value = getOptionValue(option) ?? null
  emit('update:modelValue', value)
  emit('change', value, option)
  isOpen.value = false
  triggerRef.value?.focus()
}

const clearSelection = () => {
  if (props.disabled) return
  emit('update:modelValue', null)
  emit('change', null, null)
}

// Keyboards
const onTriggerKeyDown = () => {
  if (!isOpen.value) {
    isOpen.value = true
  }
}

const onDropdownKeyDown = (e: KeyboardEvent) => {
  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      focusedIndex.value = findNextEnabledIndex(focusedIndex.value + 1)
      if (focusedIndex.value >= 0) scrollToFocused()
      break
    case 'ArrowUp':
      e.preventDefault()
      focusedIndex.value = findPrevEnabledIndex(focusedIndex.value - 1)
      if (focusedIndex.value >= 0) scrollToFocused()
      break
    case 'Enter':
      e.preventDefault()
      if (focusedIndex.value >= 0 && focusedIndex.value < filteredOptions.value.length) {
        const opt = filteredOptions.value[focusedIndex.value]
        if (!isOptionDisabled(opt)) selectOption(opt)
      }
      break
    case 'Escape':
      e.preventDefault()
      closeDropdown()
      break
    case 'Tab':
      isOpen.value = false
      break
  }
}

const scrollToFocused = () => {
  nextTick(() => {
    const list = optionsListRef.value
    if (!list) return
    const focusedEl = list.children[focusedIndex.value] as HTMLElement
    if (!focusedEl) return

    if (focusedEl.offsetTop < list.scrollTop) {
      list.scrollTop = focusedEl.offsetTop
    } else if (focusedEl.offsetTop + focusedEl.offsetHeight > list.scrollTop + list.offsetHeight) {
      list.scrollTop = focusedEl.offsetTop + focusedEl.offsetHeight - list.offsetHeight
    }
  })
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  // Check if click is inside THIS specific instance's dropdown or trigger
  const isInDropdown = !!target.closest(`.${instanceId}`)
  const isInTrigger = containerRef.value?.contains(target)

  if (!isInDropdown && !isInTrigger && isOpen.value) {
    isOpen.value = false
  }
}

onMounted(() => {
  updateViewportMode()
  document.addEventListener('click', handleClickOutside)
  window.addEventListener('resize', updateViewportMode)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('resize', updateViewportMode)
  window.removeEventListener('keydown', handleGlobalKeydown)
  if (bodyScrollLocked) document.body.style.overflow = previousBodyOverflow
  window.removeEventListener('scroll', updateTriggerRect, { capture: true })
  window.removeEventListener('resize', calculateDropdownPosition)
  if (remoteSearchTimer) {
    clearTimeout(remoteSearchTimer)
    remoteSearchTimer = null
  }
})
</script>

<style scoped>
.select-trigger {
  @apply flex w-full items-center justify-between gap-2;
  @apply rounded-xl px-4 py-2.5 text-sm;
  @apply bg-white dark:bg-dark-800;
  @apply border border-gray-200 dark:border-dark-600;
  @apply text-gray-900 dark:text-gray-100;
  @apply transition-all duration-200;
  @apply focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30;
  @apply hover:border-primary-300 dark:hover:border-primary-500/40;
  @apply cursor-pointer;
}

.select-trigger-open {
  @apply border-primary-500 ring-2 ring-primary-500/30;
}

.select-trigger-error {
  @apply border-red-500 focus:border-red-500 focus:ring-red-500/30;
}

.select-trigger-disabled {
  @apply cursor-not-allowed bg-gray-100 opacity-60 dark:bg-dark-900;
}

.select-value {
  @apply flex-1 truncate text-left;
}

.select-icon {
  @apply flex-shrink-0 text-gray-400 dark:text-dark-400;
}

.select-clear {
  @apply flex flex-shrink-0 cursor-pointer items-center justify-center;
  @apply rounded text-gray-400 transition-colors;
  @apply hover:text-gray-600 dark:hover:text-gray-200;
}
</style>

<style>
.select-dropdown-portal {
  @apply w-max min-w-[200px];
  @apply bg-white dark:bg-dark-800;
  @apply rounded-xl;
  @apply border border-gray-200 dark:border-dark-700;
  @apply shadow-lg shadow-black/10 dark:shadow-black/30;
  @apply overflow-hidden;
  pointer-events: auto !important;
}

.select-dropdown-layer {
  position: fixed;
  inset: 0;
  z-index: 100000020;
  pointer-events: none;
}

.select-dropdown-layer .select-dropdown-portal {
  pointer-events: auto;
}

.select-sheet-handle,
.select-sheet-header {
  display: none;
}

.select-dropdown-portal .select-search {
  @apply flex items-center gap-2 px-3 py-2;
  @apply border-b border-gray-100 dark:border-dark-700;
}

.select-dropdown-portal .select-search-input {
  @apply flex-1 bg-transparent text-sm;
  @apply text-gray-900 dark:text-gray-100;
  @apply placeholder:text-gray-400 dark:placeholder:text-dark-400;
  @apply focus:outline-none;
}

.select-dropdown-portal .select-options {
  @apply max-h-80 overflow-y-auto py-1 outline-none;
}

.select-dropdown-portal .select-option {
  @apply flex items-center justify-between gap-2;
  @apply px-4 py-2.5 text-sm;
  @apply text-gray-700 dark:text-gray-300;
  @apply cursor-pointer transition-colors duration-150;
  @apply hover:bg-gray-50 dark:hover:bg-dark-700;
  pointer-events: auto !important;
}

.select-dropdown-portal .select-option-selected {
  @apply bg-primary-50 dark:bg-primary-900/20;
  @apply text-primary-700 dark:text-primary-300;
}

.select-dropdown-portal .select-option-focused {
  @apply bg-gray-100 dark:bg-dark-700;
}

.select-dropdown-portal .select-option-disabled {
  @apply cursor-not-allowed opacity-40;
}

.select-dropdown-portal .select-option-group {
  @apply cursor-default select-none;
  @apply bg-gray-50 dark:bg-dark-900;
  @apply text-[11px] font-bold uppercase tracking-wider;
  @apply text-gray-500 dark:text-gray-400;
}

.select-dropdown-portal .select-option-group:hover {
  @apply bg-gray-50 dark:bg-dark-900;
}

.select-dropdown-portal .select-option-label {
  @apply flex-1 min-w-0 truncate text-left;
}

.select-dropdown-portal .select-option-icon {
  @apply flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg;
}

.select-dropdown-portal .select-option-icon-generic {
  @apply bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-300;
}

.select-dropdown-portal .select-empty {
  @apply px-4 py-8 text-center text-sm;
  @apply text-gray-500 dark:text-dark-400;
}

.select-dropdown-enter-active,
.select-dropdown-leave-active {
  transition: opacity 0.2s ease;
}

.select-dropdown-enter-from,
.select-dropdown-leave-to {
  opacity: 0;
}

.select-dropdown-enter-active .select-dropdown-portal,
.select-dropdown-leave-active .select-dropdown-portal {
  transition: transform 0.2s ease;
}

.select-dropdown-enter-from .select-dropdown-portal,
.select-dropdown-leave-to .select-dropdown-portal {
  transform: translateY(-8px);
}

@media (max-width: 767px) {
  .select-dropdown-layer-mobile {
    display: flex;
    align-items: flex-end;
    justify-content: center;
    padding: 0;
    background: rgb(15 23 42 / 42%);
    pointer-events: auto;
  }

  .select-dropdown-layer-mobile .select-dropdown-portal {
    width: 100%;
    min-width: 0;
    max-width: none;
    max-height: min(78dvh, 42rem);
    padding-bottom: max(0.5rem, env(safe-area-inset-bottom));
    border: 0;
    border-radius: 1.25rem 1.25rem 0 0;
    box-shadow: 0 -10px 34px rgb(15 23 42 / 18%);
    overscroll-behavior: contain;
  }

  .select-dropdown-layer-mobile .select-sheet-handle {
    display: block;
    width: 2.5rem;
    height: 0.25rem;
    margin: 0.7rem auto 0.35rem;
    border-radius: 999px;
    background: rgb(148 163 184 / 65%);
  }

  .select-dropdown-layer-mobile .select-sheet-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.35rem 1rem 0.7rem;
    border-bottom: 1px solid rgb(226 232 240 / 80%);
  }

  .select-dropdown-layer-mobile .select-sheet-title {
    min-width: 0;
    overflow: hidden;
    color: #0f172a;
    font-size: 0.95rem;
    font-weight: 650;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .select-dropdown-layer-mobile .select-sheet-close {
    display: grid;
    width: 2.25rem;
    height: 2.25rem;
    flex: 0 0 auto;
    place-items: center;
    border: 0;
    border-radius: 0.75rem;
    color: #64748b;
    background: transparent;
  }

  .select-dropdown-layer-mobile .select-sheet-close:active,
  .select-dropdown-layer-mobile .select-sheet-close:hover {
    background: #f1f5f9;
  }

  .select-dropdown-layer-mobile .select-search {
    margin: 0.75rem 1rem 0.25rem;
    padding: 0.7rem 0.75rem;
    border: 1px solid #e2e8f0;
    border-radius: 0.75rem;
    background: #f8fafc;
  }

  .select-dropdown-layer-mobile .select-options {
    max-height: min(52dvh, 28rem);
    padding: 0.35rem 0.75rem 0.5rem;
  }

  .select-dropdown-layer-mobile .select-option {
    min-height: 2.75rem;
    padding: 0.7rem 0.75rem;
    border-radius: 0.7rem;
  }

  .select-dropdown-layer-mobile .select-option-icon {
    width: 2.25rem;
    height: 2.25rem;
    margin-right: 0.15rem;
  }

  .select-dropdown-layer-mobile.select-dropdown-enter-from .select-dropdown-portal,
  .select-dropdown-layer-mobile.select-dropdown-leave-to .select-dropdown-portal {
    transform: translateY(100%);
  }

  .dark .select-dropdown-layer-mobile .select-sheet-title { color: #f8fafc; }
  .dark .select-dropdown-layer-mobile .select-sheet-header { border-bottom-color: rgb(71 85 105 / 80%); }
  .dark .select-dropdown-layer-mobile .select-sheet-close { color: #94a3b8; }
  .dark .select-dropdown-layer-mobile .select-sheet-close:active,
  .dark .select-dropdown-layer-mobile .select-sheet-close:hover { background: #334155; }
  .dark .select-dropdown-layer-mobile .select-search { border-color: #475569; background: #1e293b; }
}

@media (prefers-reduced-motion: reduce) {
  .select-dropdown-enter-active,
  .select-dropdown-leave-active,
  .select-dropdown-enter-active .select-dropdown-portal,
  .select-dropdown-leave-active .select-dropdown-portal {
    transition-duration: 0.01ms;
  }
}
</style>
