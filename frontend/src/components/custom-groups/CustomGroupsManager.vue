<template>
  <div class="flex min-h-0 flex-1 flex-col" data-test="custom-groups-manager">
    <template v-if="mode === 'list'">
      <div class="flex flex-col gap-4 border-b border-gray-100 px-1 pb-5 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.18em] text-amber-600 dark:text-amber-400">Model workspace</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">把不同来源分组的模型汇集到同一个 API Key。</p>
        </div>
        <button class="btn btn-primary min-h-11 w-full sm:w-auto" type="button" @click="startCreate">
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
              <button class="btn btn-secondary btn-sm" type="button" @click="startEdit(group)">编辑</button>
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
          <p class="text-xs text-gray-500">每个模型只能选择一个来源分组</p>
        </div>
      </div>

      <form id="custom-group-inline-form" class="flex min-h-0 flex-1 flex-col" @submit.prevent="save">
        <div class="min-h-0 flex-1 overflow-y-auto py-5 pr-1">
          <div class="mb-5">
            <label class="input-label">名称</label>
            <input v-model.trim="name" class="input min-h-11" maxlength="100" required placeholder="例如：酒馆统一模型" />
          </div>
          <div class="mb-3 flex items-center justify-between gap-3">
            <label class="input-label mb-0">选择模型及来源</label>
            <span class="shrink-0 rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">已选 {{ selected.size }}</span>
          </div>
          <div class="space-y-4">
            <section v-for="source in candidates" :key="source.id" class="rounded-2xl border border-gray-200 p-4 dark:border-dark-600">
              <div class="mb-3 flex flex-wrap items-center gap-2">
                <strong class="text-sm text-gray-900 dark:text-white">{{ source.name }}</strong>
                <span class="badge">{{ source.platform }}</span>
              </div>
              <div class="grid gap-2 md:grid-cols-2">
                <label v-for="model in source.models" :key="`${source.id}:${model}`" class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-transparent px-3 py-2 text-sm hover:border-amber-200 hover:bg-amber-50/60 dark:hover:border-amber-900/60 dark:hover:bg-amber-950/20">
                  <input type="checkbox" class="checkbox" :checked="isSelected(source.id, model)" @change="selectModel(source.id, model)" />
                  <span class="min-w-0 break-all">{{ model }}</span>
                </label>
              </div>
              <p v-if="source.models.length === 0" class="text-sm text-amber-600">该分组未配置可选模型列表。</p>
            </section>
          </div>
        </div>
        <div class="flex shrink-0 flex-col-reverse gap-3 border-t border-gray-100 bg-white pt-4 sm:flex-row sm:justify-end dark:border-dark-700 dark:bg-dark-900">
          <button class="btn btn-secondary min-h-11 w-full sm:w-auto" type="button" @click="backToList">取消</button>
          <button class="btn btn-primary min-h-11 w-full sm:w-auto" type="submit" :disabled="saving || selected.size === 0">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { customGroupsAPI, type CustomGroupModelInput } from '@/api/customGroups'
import type { CustomGroupCandidate, UserCustomGroup } from '@/types'
import { useAppStore } from '@/stores/app'

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
const modelKey = (model: string) => model.toLowerCase()

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
  mode.value = 'form'
}
const startEdit = (group: UserCustomGroup) => {
  editing.value = group
  name.value = group.name
  selected.value = new Map(group.models.map(model => [modelKey(model.public_model), { public_model: model.public_model, source_group_id: model.source_group_id, source_model: model.source_model }]))
  mode.value = 'form'
}
const backToList = () => { mode.value = 'list' }
const isSelected = (sourceId: number, model: string) => selected.value.get(modelKey(model))?.source_group_id === sourceId
const selectModel = (sourceId: number, model: string) => {
  const next = new Map(selected.value)
  const key = modelKey(model)
  if (next.get(key)?.source_group_id === sourceId) next.delete(key)
  else next.set(key, { public_model: model, source_group_id: sourceId, source_model: model })
  selected.value = next
}
const save = async () => {
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
