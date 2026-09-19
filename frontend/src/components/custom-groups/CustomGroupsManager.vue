<template>
  <div class="custom-groups-manager flex min-h-0 flex-1 flex-col" :class="{ 'custom-groups--neutral': appearance === 'neutral', 'is-editing': mode === 'form' }" data-test="custom-groups-manager">
    <template v-if="mode === 'list'">
      <div class="custom-groups-toolbar flex flex-col gap-3 border-b border-gray-100 px-1 pb-4 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
        <div class="custom-groups-toolbar-meta">
          <span class="custom-groups-count">{{ groups.length }} 个分组</span>
          <span class="custom-groups-toolbar-hint">按模型来源组合为一个 API Key</span>
        </div>
        <button data-test="custom-groups-create" class="btn btn-primary min-h-10 w-full sm:w-auto" type="button" @click="startCreate">
          <Icon name="plus" size="sm" class="mr-2" />
          新建分组
        </button>
      </div>

      <div class="custom-groups-list min-h-0 flex-1 overflow-y-auto py-5 pr-1">
        <div v-if="loading" class="py-14 text-center text-sm text-gray-500">正在加载…</div>
        <div v-else-if="groups.length === 0" class="custom-groups-empty rounded-2xl border border-dashed border-amber-200 bg-amber-50/50 px-5 py-14 text-center dark:border-amber-900/50 dark:bg-amber-950/10">
          <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
            <Icon name="grid" size="lg" />
          </div>
          <p class="font-medium text-gray-900 dark:text-white">还没有自定义分组</p>
          <p class="mt-1 text-sm text-gray-500">创建后即可用一个 Key 调用多个来源分组的模型。</p>
        </div>
        <div v-else class="custom-groups-grid grid gap-4 lg:grid-cols-2">
          <article v-for="group in groups" :key="group.id" class="custom-group-card rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition hover:border-amber-200 hover:shadow-md dark:border-dark-600 dark:bg-dark-800 dark:hover:border-amber-900/60">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate font-semibold text-gray-900 dark:text-white">{{ group.name }}</h3>
                <p class="mt-1 text-xs text-gray-500">{{ group.models.length }} 个模型</p>
                <p v-if="staleSourceCount(group) > 0" :data-test="`custom-group-stale-summary-${group.id}`" class="mt-1 text-xs font-medium text-red-600 dark:text-red-400">
                  {{ staleSourceCount(group) }} 个来源失效，请编辑修复
                </p>
              </div>
              <span class="custom-group-status" :class="group.status === 'active' ? 'badge badge-success is-active' : 'badge'">{{ group.status === 'active' ? '启用' : '停用' }}</span>
            </div>
            <div class="custom-group-models mt-4 flex max-h-24 flex-wrap gap-2 overflow-hidden">
              <span
                v-for="model in group.models.slice(0, 3)"
                :key="model.id"
                :data-test="model.source_available === false ? `custom-group-stale-model-${model.id}` : undefined"
                :class="[
                  'max-w-full truncate rounded-lg px-2.5 py-1 text-xs',
                  model.source_available === false
                    ? 'border border-red-200 bg-red-50 font-semibold text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300'
                    : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200',
                ]"
                :title="model.source_available === false ? '来源分组已失效' : model.source_group?.name"
              >
                {{ model.public_model }}
              </span>
              <span v-if="group.models.length > 3" class="custom-group-model-count px-2 py-1 text-xs text-gray-500">+{{ group.models.length - 3 }}</span>
            </div>
            <div class="custom-group-actions mt-5 flex flex-wrap gap-2">
              <button :data-test="`custom-groups-edit-${group.id}`" class="btn btn-secondary btn-sm" type="button" @click="startEdit(group)"><Icon name="edit" size="sm" class="mr-1.5" />编辑</button>
              <button class="btn btn-secondary btn-sm" type="button" @click="toggle(group)"><Icon :name="group.status === 'active' ? 'ban' : 'checkCircle'" size="sm" class="mr-1.5" />{{ group.status === 'active' ? '停用' : '启用' }}</button>
              <button :data-test="`custom-groups-delete-${group.id}`" class="btn btn-danger btn-sm ml-auto" type="button" @click="remove(group)"><Icon name="trash" size="sm" class="mr-1.5" />删除</button>
            </div>
          </article>
        </div>
      </div>
    </template>

    <template v-else-if="mode === 'form'">
      <div class="custom-group-form-header flex items-center gap-3 border-b border-gray-100 pb-4 dark:border-dark-700">
        <button data-test="custom-groups-back" class="btn btn-secondary h-11 w-11 shrink-0 p-0" type="button" aria-label="返回分组列表" @click="backToList">
          <Icon name="arrowLeft" size="md" />
        </button>
        <div class="min-w-0">
          <h3 class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ editing ? '编辑自定义分组' : '新建自定义分组' }}</h3>
          <p class="custom-group-form-hint text-xs text-gray-500">同一个真实模型可添加多个来源，请为每条线路设置不同调用名称</p>
        </div>
      </div>

      <form id="custom-group-inline-form" class="flex min-h-0 flex-1 flex-col" @submit.prevent="save">
        <div class="custom-group-form-scroll min-h-0 flex-1 overflow-y-auto py-5 pr-1">
          <div class="custom-group-name-field mb-5">
            <label class="input-label">名称</label>
            <input v-model.trim="name" class="input min-h-11" maxlength="100" required placeholder="例如：酒馆统一模型" />
          </div>
          <div class="custom-group-selection-toolbar mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <label class="input-label mb-0">选择模型及来源</label>
            <div class="flex flex-wrap items-center gap-2 sm:justify-end">
              <label
                data-test="custom-group-sources-select-all"
                :class="[
                  'inline-flex min-h-11 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-xs font-semibold text-gray-700 shadow-sm transition sm:min-h-9 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200',
                  totalSelectableModelCount === 0 ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:border-blue-200 hover:bg-blue-50 dark:hover:border-blue-800 dark:hover:bg-blue-950/30',
                ]"
              >
                <input
                  type="checkbox"
                  class="checkbox h-4 w-4 shrink-0"
                  :checked="allSelectableModelsSelected"
                  :indeterminate="someSelectableModelsSelected"
                  :disabled="totalSelectableModelCount === 0"
                  :aria-label="isSearching ? '全选搜索结果' : '全选全部模型'"
                  @change="toggleAllModels"
                />
                <span>{{ isSearching ? '全选结果' : '全选全部' }}</span>
              </label>
              <button data-test="custom-group-sources-toggle-all" class="btn btn-secondary min-h-11 px-3 text-xs sm:min-h-9" type="button" :disabled="filteredCandidates.length === 0" @click="allSourcesExpanded ? collapseAllSources() : expandAllSources()">
                {{ allSourcesExpanded ? '全部折叠' : '全部展开' }}
              </button>
              <span class="custom-group-selection-count shrink-0 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">已选 {{ selected.size }}</span>
            </div>
          </div>
          <div class="custom-group-search mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
            <div class="relative min-w-0 flex-1">
              <Icon name="search" size="md" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                ref="searchInput"
                v-model="searchQuery"
                type="search"
                class="input min-h-11 w-full pl-10 pr-11 [&::-webkit-search-cancel-button]:appearance-none"
                placeholder="搜索模型、来源分组或平台"
                aria-label="搜索模型及来源"
                autocomplete="off"
                @keydown.enter.prevent
              />
              <button
                v-if="searchQuery"
                type="button"
                class="absolute right-0 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-lg text-gray-400 transition-colors hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:hover:text-gray-200"
                aria-label="清空搜索"
                title="清空搜索"
                @click="clearSearch"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
            <span v-if="isSearching" class="shrink-0 text-xs text-gray-500 dark:text-gray-400" role="status">
              {{ filteredCandidates.length }} 个来源 · {{ totalSelectableModelCount }} 个模型
            </span>
          </div>
          <section v-if="staleSelectedModels.length > 0" data-test="custom-group-stale-sources" role="alert" class="mb-4 rounded-2xl border border-red-200 bg-red-50/80 p-4 dark:border-red-900/60 dark:bg-red-950/20">
            <div class="flex items-start gap-3">
              <Icon name="exclamationTriangle" size="md" class="mt-0.5 shrink-0 text-red-600 dark:text-red-400" />
              <div class="min-w-0 flex-1">
                <p class="text-sm font-semibold text-red-800 dark:text-red-200">存在失效来源线路</p>
                <p class="mt-1 text-xs leading-5 text-red-700 dark:text-red-300">这些线路已无法调度。保存时会移除失效线路，请从下方重新选择可用来源。</p>
              </div>
            </div>
            <div class="mt-3 space-y-2">
              <div v-for="model in staleSelectedModels" :key="model.id" class="flex flex-col gap-3 rounded-xl border border-red-100 bg-white/80 p-3 sm:flex-row sm:items-center sm:justify-between dark:border-red-900/50 dark:bg-dark-800/70">
                <div class="min-w-0 text-xs">
                  <p class="break-all font-mono font-semibold text-gray-900 dark:text-white">{{ model.public_model }}</p>
                  <p class="mt-1 font-semibold text-red-700 dark:text-red-300">{{ sourceIssueLabel(model) }}</p>
                  <p class="mt-1 break-all text-gray-500 dark:text-gray-400">真实模型：{{ model.source_model }} · 来源分组：{{ model.source_group?.name || `#${model.source_group_id}` }}</p>
                </div>
                <button :data-test="`custom-group-remove-stale-${model.id}`" class="btn btn-danger min-h-11 w-full shrink-0 sm:w-auto" type="button" @click="removeStaleModel(model)">移除失效线路</button>
              </div>
            </div>
          </section>
          <div v-if="isSearching && filteredCandidates.length === 0" data-test="custom-group-search-empty" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400" role="status">
            未找到匹配的模型或来源
          </div>
          <div class="custom-group-source-list space-y-4">
            <section v-for="source in filteredCandidates" :key="source.id" class="custom-group-source overflow-hidden rounded-2xl border border-gray-200 bg-white transition-colors dark:border-dark-600 dark:bg-dark-800">
              <div class="custom-group-source-header flex min-h-12 items-center gap-2 px-4 py-3 transition hover:bg-gray-50 dark:hover:bg-dark-700/60">
                <label
                  :data-test="`custom-group-source-select-all-${source.id}`"
                  :class="[
                    'inline-flex min-h-8 shrink-0 items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 text-xs font-semibold text-gray-700 shadow-sm transition dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200',
                    source.models.length === 0 ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:border-blue-200 hover:bg-blue-50 dark:hover:border-blue-800 dark:hover:bg-blue-950/30',
                  ]"
                  @click.stop
                >
                  <input
                    type="checkbox"
                    class="checkbox h-4 w-4 shrink-0"
                    :checked="allModelsForSourceSelected(source)"
                    :indeterminate="someModelsForSourceSelected(source)"
                    :disabled="source.models.length === 0"
                    :aria-label="`全选 ${source.name} 的${isSearching ? '匹配' : ''}模型`"
                    @change="toggleAllModelsForSource(source)"
                  />
                  <span>{{ isSearching ? '全选匹配' : '全选本组' }}</span>
                </label>
                <button
                  :data-test="`custom-group-source-toggle-${source.id}`"
                  class="flex w-full min-w-0 flex-1 items-start justify-between gap-3 rounded-lg text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:items-center"
                  type="button"
                  :aria-expanded="isSourceExpanded(source.id)"
                  :aria-controls="`custom-group-source-models-${source.id}`"
                  @click="toggleSource(source.id)"
                >
                  <span class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
                    <strong class="min-w-0 break-words text-sm text-gray-900 dark:text-white">{{ source.name }}</strong>
                    <span class="badge">{{ source.platform }}</span>
                    <span class="text-xs text-gray-500 dark:text-gray-400">{{ source.models.length }} 个模型</span>
                    <span v-if="selectedCountForSource(source.id) > 0" class="rounded-full bg-blue-50 px-2.5 py-1 text-xs font-semibold text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">已选 {{ selectedCountForSource(source.id) }}</span>
                  </span>
                  <Icon name="chevronDown" size="sm" :class="['shrink-0 text-gray-400 transition-transform duration-200', isSourceExpanded(source.id) ? 'rotate-180' : '']" />
                </button>
              </div>
              <div
                v-if="isSourceExpanded(source.id)"
                :id="`custom-group-source-models-${source.id}`"
                :data-test="`custom-group-source-models-${source.id}`"
                class="border-t border-gray-100 p-4 dark:border-dark-700"
              >
                <div v-if="source.models.length > 0" class="grid grid-cols-1 gap-2 xl:grid-cols-2">
                  <article v-for="model in source.models" :key="sourceMappingKey(source.id, model)" :class="['custom-group-source-model rounded-xl border p-3 transition', isSelected(source.id, model) ? 'border-blue-300 bg-blue-50/60 dark:border-blue-800 dark:bg-blue-950/20' : 'border-gray-100 hover:border-amber-200 dark:border-dark-700 dark:hover:border-amber-900/60']">
                    <label class="flex min-h-8 cursor-pointer items-start gap-3 text-sm">
                      <input type="checkbox" class="checkbox mt-0.5 shrink-0" :checked="isSelected(source.id, model)" @change="selectModel(source.id, model, source.name)" />
                      <span class="min-w-0">
                        <span class="block text-[11px] font-medium text-gray-400">真实模型</span>
                        <span class="block break-all font-medium text-gray-800 dark:text-gray-100">{{ model }}</span>
                        <span class="mt-1 block text-xs text-gray-500">来源分组 · {{ source.name }}</span>
                      </span>
                    </label>
                    <div v-if="isSelected(source.id, model)" class="mt-3 border-t border-blue-100 pt-3 dark:border-blue-900/50">
                      <label class="mb-1.5 block text-xs font-semibold text-gray-600 dark:text-gray-300">调用名称</label>
                      <input :value="selectedItem(source.id, model)?.public_model" class="input min-h-11 w-full font-mono text-sm" maxlength="200" autocomplete="off" :aria-invalid="Boolean(aliasErrors.get(sourceMappingKey(source.id, model)))" @input="updateCallName(source.id, model, ($event.target as HTMLInputElement).value)" />
                      <p v-if="aliasErrors.get(sourceMappingKey(source.id, model))" class="mt-1.5 text-xs text-red-600">{{ aliasErrors.get(sourceMappingKey(source.id, model)) }}</p>
                      <p v-else class="mt-1.5 text-xs text-gray-400">请求时使用该名称，实际转发与计费仍按上面的真实模型和来源分组。</p>
                    </div>
                  </article>
                </div>
                <p v-else class="text-sm text-amber-600">该分组未配置可选模型列表。</p>
              </div>
            </section>
          </div>
        </div>
        <div class="custom-group-form-footer flex shrink-0 gap-3 border-t border-gray-100 bg-white pt-4 sm:justify-end dark:border-dark-700 dark:bg-dark-900">
          <button class="btn btn-secondary min-h-11 w-full sm:w-auto" type="button" @click="backToList">取消</button>
          <button class="btn btn-primary min-h-11 w-full sm:w-auto" type="submit" :disabled="saving || saveableModels.length === 0 || aliasErrors.size > 0">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { customGroupsAPI, type CustomGroupModelInput } from '@/api/customGroups'
import type { CustomGroupCandidate, UserCustomGroup, UserCustomGroupModel } from '@/types'
import { useAppStore } from '@/stores/app'
import { sourceMappingKey, suggestCallName, validateCallNames } from './modelAliases'

const MAX_CUSTOM_GROUP_MODELS = 500
const props = withDefaults(defineProps<{ appearance?: 'default' | 'neutral' }>(), { appearance: 'default' })
const appearance = computed(() => props.appearance)
const emit = defineEmits<{ (e: 'changed'): void }>()
const app = useAppStore()
const mode = ref<'list' | 'form'>('list')
const groups = ref<UserCustomGroup[]>([])
const candidates = ref<CustomGroupCandidate[]>([])
const loading = ref(true)
const saving = ref(false)
const editing = ref<UserCustomGroup | null>(null)
const name = ref('')
const searchInput = ref<HTMLInputElement | null>(null)
const searchQuery = ref('')
const searchTokens = computed(() => searchQuery.value.trim().toLowerCase().split(/\s+/).filter(Boolean))
const isSearching = computed(() => searchTokens.value.length > 0)
const filteredCandidates = computed(() => {
  if (!isSearching.value) return candidates.value
  return candidates.value.flatMap(source => {
    const sourceText = `${source.name} ${source.platform}`.toLowerCase()
    if (searchTokens.value.every(token => sourceText.includes(token))) return [source]
    const models = source.models.filter(model => {
      const text = `${sourceText} ${model.toLowerCase()}`
      return searchTokens.value.every(token => text.includes(token))
    })
    return models.length > 0 ? [{ ...source, models }] : []
  })
})
const selected = ref(new Map<string, CustomGroupModelInput>())
const expandedSourceIds = ref(new Set<number>())
const searchExpandedSourceIds = ref(new Set<number>())
const visibleExpandedSourceIds = computed(() => isSearching.value ? searchExpandedSourceIds.value : expandedSourceIds.value)
watch(filteredCandidates, sources => {
  searchExpandedSourceIds.value = new Set(sources.map(source => source.id))
})
const selectedEntries = computed(() => [...selected.value.entries()])
const staleSelectedModels = computed(() => {
  if (!editing.value) return []
  return editing.value.models.filter(model => model.source_available === false && selected.value.has(sourceMappingKey(model.source_group_id, model.source_model)))
})
const staleSelectedKeys = computed(() => new Set(staleSelectedModels.value.map(model => sourceMappingKey(model.source_group_id, model.source_model))))
const saveableModelEntries = computed(() => selectedEntries.value.filter(([key]) => !staleSelectedKeys.value.has(key)))
const saveableModels = computed(() => saveableModelEntries.value.map(([, item]) => item))
const aliasErrors = computed(() => validateCallNames(saveableModelEntries.value.map(([key, item]) => ({ key, callName: item.public_model }))))
const allSourcesExpanded = computed(() => filteredCandidates.value.length > 0 && filteredCandidates.value.every(source => visibleExpandedSourceIds.value.has(source.id)))
const selectableModelEntries = computed(() => filteredCandidates.value.flatMap(source =>
  source.models.map(model => ({ key: sourceMappingKey(source.id, model), sourceId: source.id, sourceName: source.name, model }))
))
const totalSelectableModelCount = computed(() => selectableModelEntries.value.length)
const selectedSelectableModelCount = computed(() => selectableModelEntries.value.filter(entry => selected.value.has(entry.key)).length)
const allSelectableModelsSelected = computed(() => totalSelectableModelCount.value > 0 && selectableModelEntries.value.every(entry => selected.value.has(entry.key)))
const someSelectableModelsSelected = computed(() => selectedSelectableModelCount.value > 0 && !allSelectableModelsSelected.value)

const load = async () => {
  loading.value = true
  try {
    const [loadedGroups, loadedCandidates] = await Promise.all([customGroupsAPI.list(), customGroupsAPI.candidates()])
    groups.value = loadedGroups
    candidates.value = loadedCandidates.map(source => ({
      ...source,
      models: Array.isArray(source.models) ? source.models : [],
    }))
  } finally {
    loading.value = false
  }
}
const startCreate = () => {
  editing.value = null
  name.value = ''
  selected.value = new Map()
  resetExpandedSources()
  mode.value = 'form'
}
const startEdit = (group: UserCustomGroup) => {
  editing.value = group
  name.value = group.name
  selected.value = new Map(group.models.map(model => [sourceMappingKey(model.source_group_id, model.source_model), { public_model: model.public_model, source_group_id: model.source_group_id, source_model: model.source_model }]))
  resetExpandedSources()
  mode.value = 'form'
}
const backToList = () => { mode.value = 'list' }
const clearSearch = () => {
  searchQuery.value = ''
  searchInput.value?.focus()
}
const resetExpandedSources = () => {
  searchQuery.value = ''
  expandedSourceIds.value = new Set()
}
const setExpandedSources = (ids: Set<number>) => {
  if (isSearching.value) searchExpandedSourceIds.value = ids
  else expandedSourceIds.value = ids
}
const isSourceExpanded = (sourceId: number) => visibleExpandedSourceIds.value.has(sourceId)
const toggleSource = (sourceId: number) => {
  const next = new Set(visibleExpandedSourceIds.value)
  if (next.has(sourceId)) next.delete(sourceId)
  else next.add(sourceId)
  setExpandedSources(next)
}
const expandAllSources = () => { setExpandedSources(new Set(filteredCandidates.value.map(source => source.id))) }
const collapseAllSources = () => { setExpandedSources(new Set()) }
const toggleAllModels = () => {
  if (allSelectableModelsSelected.value) {
    const next = new Map(selected.value)
    for (const entry of selectableModelEntries.value) next.delete(entry.key)
    selected.value = next
    return
  }
  const next = new Map(selected.value)
  for (const entry of selectableModelEntries.value) {
    if (next.has(entry.key)) continue
    next.set(entry.key, {
      public_model: suggestCallName(entry.model, entry.sourceName, [...next.values()].map(item => item.public_model)),
      source_group_id: entry.sourceId,
      source_model: entry.model,
    })
  }
  selected.value = next
}
const allModelsForSourceSelected = (source: CustomGroupCandidate) =>
  source.models.length > 0 && source.models.every(model => selected.value.has(sourceMappingKey(source.id, model)))
const someModelsForSourceSelected = (source: CustomGroupCandidate) =>
  source.models.some(model => selected.value.has(sourceMappingKey(source.id, model))) && !allModelsForSourceSelected(source)
const toggleAllModelsForSource = (source: CustomGroupCandidate) => {
  const next = new Map(selected.value)
  if (allModelsForSourceSelected(source)) {
    for (const model of source.models) {
      next.delete(sourceMappingKey(source.id, model))
    }
    selected.value = next
    return
  }
  for (const model of source.models) {
    const key = sourceMappingKey(source.id, model)
    if (next.has(key)) continue
    next.set(key, {
      public_model: suggestCallName(model, source.name, [...next.values()].map(item => item.public_model)),
      source_group_id: source.id,
      source_model: model,
    })
  }
  selected.value = next
}
const selectedCountForSource = (sourceId: number) => [...selected.value.values()].filter(item => item.source_group_id === sourceId).length
const staleSourceCount = (group: UserCustomGroup) => group.models.filter(model => model.source_available === false).length
const sourceIssueLabel = (model: UserCustomGroupModel) => {
  if (model.source_issue === 'source_group_not_allowed') return '已无权使用该来源分组'
  if (model.source_issue === 'source_model_unavailable') return '来源模型已下架或改名'
  return '来源分组已下架或停用'
}
const selectedItem = (sourceId: number, model: string) => selected.value.get(sourceMappingKey(sourceId, model))
const isSelected = (sourceId: number, model: string) => selected.value.has(sourceMappingKey(sourceId, model))
const selectModel = (sourceId: number, model: string, sourceName: string) => {
  const next = new Map(selected.value)
  const key = sourceMappingKey(sourceId, model)
  if (next.has(key)) next.delete(key)
  else next.set(key, { public_model: suggestCallName(model, sourceName, [...next.values()].map(item => item.public_model)), source_group_id: sourceId, source_model: model })
  selected.value = next
}
const updateCallName = (sourceId: number, model: string, publicModel: string) => {
  const key = sourceMappingKey(sourceId, model)
  const item = selected.value.get(key)
  if (!item) return
  const next = new Map(selected.value)
  next.set(key, { ...item, public_model: publicModel })
  selected.value = next
}
const removeStaleModel = (model: UserCustomGroupModel) => {
  const next = new Map(selected.value)
  next.delete(sourceMappingKey(model.source_group_id, model.source_model))
  selected.value = next
}
const save = async () => {
  const models = saveableModels.value
  if (models.length === 0) {
    app.showError('请选择至少一个可用来源模型')
    return
  }
  if (aliasErrors.value.size > 0) {
    app.showError('请先修正调用名称')
    return
  }
  if (models.length > MAX_CUSTOM_GROUP_MODELS) {
    app.showError(`一个自定义分组最多选择 ${MAX_CUSTOM_GROUP_MODELS} 个模型，请先取消部分模型`)
    return
  }
  const removedStaleCount = staleSelectedModels.value.length
  saving.value = true
  try {
    if (editing.value) await customGroupsAPI.update(editing.value.id, { name: name.value, models })
    else await customGroupsAPI.create(name.value, models)
    await load()
    mode.value = 'list'
    emit('changed')
    app.showSuccess(removedStaleCount > 0 ? `自定义分组已保存，已移除 ${removedStaleCount} 个失效线路` : '自定义分组已保存')
  } catch (error: any) {
    app.showError(error?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
const toggle = async (group: UserCustomGroup) => {
  await customGroupsAPI.update(group.id, { status: group.status === 'active' ? 'disabled' : 'active' })
  await load()
  emit('changed')
}
const remove = async (group: UserCustomGroup) => {
  if (!window.confirm(`确定删除“${group.name}”吗？`)) return
  try {
    await customGroupsAPI.delete(group.id)
    await load()
    emit('changed')
    app.showSuccess('自定义分组已删除')
  } catch (error: any) {
    if (error?.reason !== 'CUSTOM_GROUP_IN_USE') {
      app.showError(error?.message || '删除失败')
      return
    }

    const rawCount = error?.metadata?.bound_api_key_count
    const parsedCount = Number.parseInt(String(rawCount ?? ''), 10)
    const affectedLabel = Number.isFinite(parsedCount) && parsedCount > 0
      ? `${parsedCount} 个 API 密钥`
      : '绑定该分组的 API 密钥'
    if (!window.confirm(`该分组仍绑定 ${affectedLabel}。继续删除会解除这些绑定，密钥将恢复使用原分组。确定继续吗？`)) return

    try {
      const result = await customGroupsAPI.delete(group.id, true)
      await load()
      emit('changed')
      const unboundCount = Number(result?.unbound_api_key_count) || parsedCount || 0
      app.showSuccess(unboundCount > 0 ? `自定义分组已删除，已解除 ${unboundCount} 个密钥绑定` : '自定义分组已删除')
    } catch (forceError: any) {
      app.showError(forceError?.message || '删除失败，请重试')
    }
  }
}

onMounted(load)
</script>

<style scoped>
.custom-groups-toolbar-meta {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 0.625rem;
}
.custom-groups-count { flex-shrink: 0; color: #374151; font-size: 13px; font-weight: 600; white-space: nowrap; }
.custom-groups-toolbar-hint {
  overflow: hidden;
  color: #9ca3af;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dark .custom-groups-count { color: #e5e7eb; }
.custom-groups-manager .custom-groups-list { padding-top: 0.875rem; }
.custom-groups-manager .custom-groups-grid { gap: 0.75rem; }
.custom-groups-manager .custom-group-card {
  border-radius: 8px;
  border-color: rgb(100 116 139 / 18%);
  background: linear-gradient(125deg, rgb(255 255 255 / 90%), rgb(248 249 251 / 70%));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 5px rgb(15 23 42 / 4%);
}
.dark .custom-groups-manager .custom-group-card {
  border-color: rgb(255 255 255 / 10%);
  background: linear-gradient(125deg, rgb(255 255 255 / 4%), transparent), #27292e;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%), 0 2px 5px rgb(0 0 0 / 12%);
}
.custom-groups-manager .custom-group-card h3 { font-size: 14px; }
.custom-groups-manager .custom-group-models { max-height: 3.25rem; margin-top: 0.75rem; gap: 0.375rem; overflow: hidden; }
.custom-groups-manager .custom-group-models > span { border-radius: 4px; padding: 0.125rem 0.375rem; font-size: 11px; }
.custom-groups-manager .custom-group-actions { margin-top: 0.75rem; padding-top: 0.375rem; border-top: 1px solid rgb(100 116 139 / 14%); }
.dark .custom-groups-manager .custom-group-actions { border-color: rgb(255 255 255 / 8%); }
.custom-groups-manager .custom-group-form-hint { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.custom-groups-manager .custom-group-form-header { padding-bottom: 0.75rem; }
.custom-groups-manager .custom-group-form-header > button { width: 2.25rem; height: 2.25rem; border-radius: 6px; }
.custom-groups-manager .custom-group-status {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.375rem;
  padding: 0;
  border: 0;
  background: transparent;
  color: #6b7280;
  font-size: 11px;
  box-shadow: none;
}
.custom-groups-manager .custom-group-status::before { content: ''; width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.custom-groups-manager .custom-group-status.is-active { color: #059669; }
.dark .custom-groups-manager .custom-group-status.is-active { color: #6ee7b7; }
.custom-groups-manager .custom-group-selection-toolbar > label,
.custom-groups-manager .custom-group-selection-toolbar > button {
  min-height: 2.25rem;
  border-radius: 6px;
  box-shadow: none;
}
.custom-groups-manager .custom-group-selection-toolbar > label { padding: 0 0.625rem; }
.custom-groups-manager .custom-group-selection-toolbar > button { padding: 0.375rem 0.625rem; }
.custom-groups-manager .custom-group-search .input { min-height: 2.5rem; }
.custom-groups-manager .custom-group-source-header > label { min-height: 2.25rem; padding: 0 0.5rem; }
.custom-groups-manager .custom-group-source-header > button { min-height: 2.25rem; align-items: center; }
.custom-groups-manager .custom-group-source-list { gap: 0.625rem; }
.custom-groups-manager .custom-group-form-footer { flex-direction: row; justify-content: flex-end; gap: 0.5rem; border-color: rgb(100 116 139 / 14%); background: transparent; }
.custom-groups-manager .custom-group-form-footer > button { width: auto; min-width: 4.5rem; }
.dark .custom-groups-manager .custom-group-form-footer { border-color: rgb(255 255 255 / 8%); }
.custom-groups-manager .custom-group-source { border-radius: 8px; border-color: rgb(100 116 139 / 18%); }
.custom-groups-manager .custom-group-source-model { border-radius: 6px; }
.custom-groups-manager .custom-group-form-footer { padding-top: 0.75rem; }
.custom-groups-manager .custom-group-form-footer > button { min-height: 2.5rem; }
.custom-groups--neutral { max-height: inherit; }
.custom-groups--neutral.is-editing {
  /* Older WebKit ignores this height when the flex basis is zero. */
  flex-basis: auto;
  height: min(72dvh, 42rem);
}
.custom-groups--neutral .custom-groups-toolbar {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0 0 0.875rem;
  border-color: var(--dialog-rule);
}
.custom-groups--neutral .custom-groups-toolbar > button { width: auto; flex-shrink: 0; }
.custom-groups--neutral .custom-groups-list { padding: 0.875rem 0.125rem 0.125rem; overscroll-behavior: contain; }
.custom-groups--neutral .custom-groups-grid { gap: 0.75rem; }
.custom-groups--neutral .custom-group-card {
  min-width: 0;
  padding: 0.875rem 0.875rem 0.375rem;
  border-color: var(--dialog-rule);
  border-radius: 8px;
  background: linear-gradient(125deg, rgb(255 255 255 / 90%), rgb(248 249 251 / 70%));
  box-shadow: inset 0 1px 0 var(--dialog-highlight), 0 2px 4px rgb(0 0 0 / 3%);
  transition: border-color 160ms ease;
}
.dark .custom-groups--neutral .custom-group-card { background: linear-gradient(125deg, rgb(255 255 255 / 4%), transparent), #27292e; }
.custom-groups--neutral .custom-group-card h3 { font-size: 14px; }
.custom-groups--neutral .custom-group-status {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.375rem;
  padding: 0;
  border: 0;
  border-radius: 0;
  color: #6b7280;
  background: none;
  font-size: 11px;
  box-shadow: none;
}
.custom-groups--neutral .custom-group-status::before { content: ''; width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.custom-groups--neutral .custom-group-status.is-active { color: #059669; }
.dark .custom-groups--neutral .custom-group-status.is-active { color: #6ee7b7; }
.custom-groups--neutral .custom-group-models { max-height: none; margin-top: 0.75rem; gap: 0.375rem; }
.custom-groups--neutral .custom-group-models > span { border-radius: 4px; padding: 0.125rem 0.375rem; font-size: 11px; }
.custom-groups--neutral .custom-group-model-count { align-self: center; }
.custom-groups--neutral .custom-group-actions { gap: 0.25rem; margin-top: 0.75rem; padding-top: 0.375rem; border-top: 1px solid var(--dialog-rule); }
.custom-groups--neutral .custom-group-actions > button {
  min-height: 2.5rem;
  padding: 0.375rem 0.5rem;
  border: 0;
  color: #6b7280;
  background: transparent;
  box-shadow: none;
  font-size: 12px;
  transform: none;
}
.custom-groups--neutral .custom-group-actions > button:hover { background: rgb(148 163 184 / 10%); }
.custom-groups--neutral .custom-group-actions > .btn-danger:hover { color: #dc2626; background: rgb(239 68 68 / 7%); }
.dark .custom-groups--neutral .custom-group-actions > button { color: #9ca3af; }
.custom-groups--neutral .custom-groups-empty { border: 0; background: transparent; }
.custom-groups--neutral .custom-groups-empty > div:first-child { border-radius: 8px; color: #6b7280; background: rgb(148 163 184 / 10%); }
.custom-groups--neutral .custom-group-form-header { padding-bottom: 0.75rem; border-color: var(--dialog-rule); }
.custom-groups--neutral .custom-group-form-header h3 { font-size: 14px; }
.custom-groups--neutral .custom-group-selection-count { padding: 0; border-radius: 0; color: #6b7280; background: transparent; }
.dark .custom-groups--neutral .custom-group-selection-count { color: #9ca3af; }
.custom-groups--neutral .custom-group-source { border-color: var(--dialog-rule); border-radius: 6px; background: transparent; }
.custom-groups--neutral .custom-group-source-model { border-radius: 4px; box-shadow: none; }
.custom-groups--neutral .custom-group-form-footer { flex-direction: row; justify-content: flex-end; padding-top: 0.75rem; gap: 0.5rem; border-color: var(--dialog-rule); background: transparent; }
.custom-groups--neutral .custom-group-form-footer > button { width: auto; min-width: 4.5rem; }
@media (prefers-reduced-motion: reduce) {
  .custom-groups--neutral .custom-group-card { transition: none; }
}
</style>
