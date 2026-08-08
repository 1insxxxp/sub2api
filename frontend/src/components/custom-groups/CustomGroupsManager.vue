<template>
  <div class="flex min-h-0 flex-1 flex-col" data-test="custom-groups-manager">
    <template v-if="mode === 'list'">
      <div class="flex flex-col gap-4 border-b border-gray-100 px-1 pb-5 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.18em] text-amber-600 dark:text-amber-400">Model workspace</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">把不同来源分组的模型汇集到同一个 API Key。</p>
        </div>
        <button data-test="custom-groups-create" class="btn btn-primary min-h-11 w-full sm:w-auto" type="button" @click="startCreate">
          <Icon name="plus" size="sm" class="mr-2" />
          新建自定义分组
        </button>
      </div>

      <div class="min-h-0 flex-1 overflow-y-auto py-5 pr-1">
        <div v-if="loading" class="py-14 text-center text-sm text-gray-500">正在加载…</div>
        <div v-else-if="groups.length === 0" class="rounded-2xl border border-dashed border-amber-200 bg-amber-50/50 px-5 py-14 text-center dark:border-amber-900/50 dark:bg-amber-950/10">
          <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
            <Icon name="grid" size="lg" />
          </div>
          <p class="font-medium text-gray-900 dark:text-white">还没有自定义分组</p>
          <p class="mt-1 text-sm text-gray-500">创建后即可用一个 Key 调用多个来源分组的模型。</p>
        </div>
        <div v-else class="grid gap-4 lg:grid-cols-2">
          <article v-for="group in groups" :key="group.id" class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition hover:border-amber-200 hover:shadow-md dark:border-dark-600 dark:bg-dark-800 dark:hover:border-amber-900/60">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3 class="truncate font-semibold text-gray-900 dark:text-white">{{ group.name }}</h3>
                <p class="mt-1 text-xs text-gray-500">{{ group.models.length }} 个模型</p>
              </div>
              <span :class="group.status === 'active' ? 'badge badge-success' : 'badge'">{{ group.status === 'active' ? '启用' : '停用' }}</span>
            </div>
            <div class="mt-4 flex max-h-24 flex-wrap gap-2 overflow-hidden">
              <span v-for="model in group.models.slice(0, 8)" :key="model.id" class="max-w-full truncate rounded-lg bg-gray-100 px-2.5 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-200" :title="model.source_group?.name">
                {{ model.public_model }}
              </span>
              <span v-if="group.models.length > 8" class="px-2 py-1 text-xs text-gray-500">+{{ group.models.length - 8 }}</span>
            </div>
            <div class="mt-5 flex flex-wrap gap-2">
              <button :data-test="`custom-groups-edit-${group.id}`" class="btn btn-secondary btn-sm" type="button" @click="startEdit(group)">编辑</button>
              <button class="btn btn-secondary btn-sm" type="button" @click="toggle(group)">{{ group.status === 'active' ? '停用' : '启用' }}</button>
              <button class="btn btn-danger btn-sm ml-auto" type="button" @click="remove(group)">删除</button>
            </div>
          </article>
        </div>
      </div>
    </template>

    <template v-else-if="mode === 'form'">
      <div class="flex items-center gap-3 border-b border-gray-100 pb-4 dark:border-dark-700">
        <button data-test="custom-groups-back" class="btn btn-secondary h-11 w-11 shrink-0 p-0" type="button" aria-label="返回分组列表" @click="backToList">
          <Icon name="arrowLeft" size="md" />
        </button>
        <div class="min-w-0">
          <h3 class="truncate text-lg font-semibold text-gray-900 dark:text-white">{{ editing ? '编辑自定义分组' : '新建自定义分组' }}</h3>
          <p class="text-xs text-gray-500">同一个真实模型可添加多个来源，请为每条线路设置不同调用名称</p>
        </div>
      </div>

      <form id="custom-group-inline-form" class="flex min-h-0 flex-1 flex-col" @submit.prevent="save">
        <div class="min-h-0 flex-1 overflow-y-auto py-5 pr-1">
          <div class="mb-5">
            <label class="input-label">名称</label>
            <input v-model.trim="name" class="input min-h-11" maxlength="100" required placeholder="例如：酒馆统一模型" />
          </div>
          <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <label class="input-label mb-0">选择模型及来源</label>
            <div class="flex items-center justify-between gap-2 sm:justify-end">
              <button data-test="custom-group-sources-toggle-all" class="btn btn-secondary min-h-11 px-3 text-xs sm:min-h-9" type="button" :disabled="candidates.length === 0" @click="allSourcesExpanded ? collapseAllSources() : expandAllSources()">
                {{ allSourcesExpanded ? '全部折叠' : '全部展开' }}
              </button>
              <span class="shrink-0 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">已选 {{ selected.size }}</span>
            </div>
          </div>
          <div class="space-y-4">
            <section v-for="source in candidates" :key="source.id" class="overflow-hidden rounded-2xl border border-gray-200 bg-white transition-colors dark:border-dark-600 dark:bg-dark-800">
              <button
                :data-test="`custom-group-source-toggle-${source.id}`"
                class="flex min-h-12 w-full items-center justify-between gap-3 px-4 py-3 text-left transition hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500 dark:hover:bg-dark-700/60"
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
              <div
                v-if="isSourceExpanded(source.id)"
                :id="`custom-group-source-models-${source.id}`"
                :data-test="`custom-group-source-models-${source.id}`"
                class="border-t border-gray-100 p-4 dark:border-dark-700"
              >
                <div v-if="source.models.length > 0" class="grid grid-cols-1 gap-2 xl:grid-cols-2">
                  <article v-for="model in source.models" :key="sourceMappingKey(source.id, model)" :class="['rounded-xl border p-3 transition', isSelected(source.id, model) ? 'border-blue-300 bg-blue-50/60 dark:border-blue-800 dark:bg-blue-950/20' : 'border-gray-100 hover:border-amber-200 dark:border-dark-700 dark:hover:border-amber-900/60']">
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
        <div class="flex shrink-0 flex-col-reverse gap-3 border-t border-gray-100 bg-white pt-4 sm:flex-row sm:justify-end dark:border-dark-700 dark:bg-dark-900">
          <button class="btn btn-secondary min-h-11 w-full sm:w-auto" type="button" @click="backToList">取消</button>
          <button class="btn btn-primary min-h-11 w-full sm:w-auto" type="submit" :disabled="saving || selected.size === 0 || aliasErrors.size > 0">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { customGroupsAPI, type CustomGroupModelInput } from '@/api/customGroups'
import type { CustomGroupCandidate, UserCustomGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { sourceMappingKey, suggestCallName, validateCallNames } from './modelAliases'

const emit = defineEmits<{ (e: 'changed'): void }>()
const app = useAppStore()
const mode = ref<'list' | 'form'>('list')
const groups = ref<UserCustomGroup[]>([])
const candidates = ref<CustomGroupCandidate[]>([])
const loading = ref(true)
const saving = ref(false)
const editing = ref<UserCustomGroup | null>(null)
const name = ref('')
const selected = ref(new Map<string, CustomGroupModelInput>())
const expandedSourceIds = ref(new Set<number>())
const aliasErrors = computed(() => validateCallNames([...selected.value.entries()].map(([key, item]) => ({ key, callName: item.public_model }))))
const allSourcesExpanded = computed(() => candidates.value.length > 0 && candidates.value.every(source => expandedSourceIds.value.has(source.id)))

const load = async () => {
  loading.value = true
  try {
    const [loadedGroups, loadedCandidates] = await Promise.all([customGroupsAPI.list(), customGroupsAPI.candidates()])
    groups.value = loadedGroups
    candidates.value = loadedCandidates
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
const resetExpandedSources = () => { expandedSourceIds.value = new Set() }
const isSourceExpanded = (sourceId: number) => expandedSourceIds.value.has(sourceId)
const toggleSource = (sourceId: number) => {
  const next = new Set(expandedSourceIds.value)
  if (next.has(sourceId)) next.delete(sourceId)
  else next.add(sourceId)
  expandedSourceIds.value = next
}
const expandAllSources = () => { expandedSourceIds.value = new Set(candidates.value.map(source => source.id)) }
const collapseAllSources = () => { expandedSourceIds.value = new Set() }
const selectedCountForSource = (sourceId: number) => [...selected.value.values()].filter(item => item.source_group_id === sourceId).length
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
const save = async () => {
	if (aliasErrors.value.size > 0) {
		app.showError('请先修正调用名称')
		return
	}
  saving.value = true
  try {
    const models = [...selected.value.values()]
    if (editing.value) await customGroupsAPI.update(editing.value.id, { name: name.value, models })
    else await customGroupsAPI.create(name.value, models)
    await load()
    mode.value = 'list'
    emit('changed')
    app.showSuccess('自定义分组已保存')
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
  } catch (error: any) {
    app.showError(error?.message || '仍有 API Key 绑定该分组，无法删除')
  }
}

onMounted(load)
</script>
