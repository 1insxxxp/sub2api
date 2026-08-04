<template>
  <div class="mx-auto max-w-6xl space-y-6 px-4 py-6 sm:px-6">
    <section class="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
      <div class="flex flex-col gap-4 border-b border-gray-100 px-6 py-5 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.2em] text-primary-600">Model workspace</p>
          <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">自定义分组</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">把不同来源分组的模型汇集到一个 API Key。</p>
        </div>
        <button class="btn btn-primary" type="button" @click="startCreate">新建自定义分组</button>
      </div>

      <div v-if="loading" class="px-6 py-14 text-center text-sm text-gray-500">正在加载…</div>
      <div v-else-if="groups.length === 0" class="px-6 py-14 text-center">
        <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-primary-50 text-xl text-primary-600 dark:bg-primary-950/40">◇</div>
        <p class="font-medium text-gray-900 dark:text-white">还没有自定义分组</p>
        <p class="mt-1 text-sm text-gray-500">创建后即可用一个 Key 调用多个来源分组的模型。</p>
      </div>
      <div v-else class="grid gap-4 p-5 md:grid-cols-2">
        <article v-for="group in groups" :key="group.id" class="rounded-xl border border-gray-200 p-5 transition hover:-translate-y-0.5 hover:shadow-md dark:border-dark-600">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ group.name }}</h2>
              <p class="mt-1 text-xs text-gray-500">{{ group.models.length }} 个模型</p>
            </div>
            <span :class="group.status === 'active' ? 'badge badge-success' : 'badge'">{{ group.status === 'active' ? '启用' : '停用' }}</span>
          </div>
          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="model in group.models.slice(0, 8)" :key="model.id" class="rounded-lg bg-gray-100 px-2.5 py-1 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-200" :title="model.source_group?.name">
              {{ model.public_model }}
            </span>
            <span v-if="group.models.length > 8" class="px-2 py-1 text-xs text-gray-500">+{{ group.models.length - 8 }}</span>
          </div>
          <div class="mt-5 flex gap-2">
            <button class="btn btn-secondary btn-sm" type="button" @click="startEdit(group)">编辑</button>
            <button class="btn btn-secondary btn-sm" type="button" @click="toggle(group)">{{ group.status === 'active' ? '停用' : '启用' }}</button>
            <button class="btn btn-danger btn-sm ml-auto" type="button" @click="remove(group)">删除</button>
          </div>
        </article>
      </div>
    </section>

    <BaseDialog :show="dialogOpen" :title="editing ? '编辑自定义分组' : '新建自定义分组'" width="wide" @close="dialogOpen = false">
      <form id="custom-group-form" class="space-y-5" @submit.prevent="save">
        <div><label class="input-label">名称</label><input v-model.trim="name" class="input" maxlength="100" required placeholder="例如：酒馆统一模型" /></div>
        <div>
          <div class="mb-3 flex items-center justify-between"><label class="input-label mb-0">选择模型及来源</label><span class="text-xs text-gray-500">已选 {{ selected.size }}</span></div>
          <div class="max-h-[52vh] space-y-4 overflow-y-auto pr-1">
            <section v-for="source in candidates" :key="source.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
              <div class="mb-3 flex items-center gap-2"><strong class="text-sm text-gray-900 dark:text-white">{{ source.name }}</strong><span class="badge">{{ source.platform }}</span></div>
              <div class="grid gap-2 sm:grid-cols-2">
                <label v-for="model in source.models" :key="`${source.id}:${model}`" class="flex cursor-pointer items-center gap-3 rounded-lg border border-transparent px-3 py-2 text-sm hover:border-primary-200 hover:bg-primary-50/50 dark:hover:border-primary-900 dark:hover:bg-primary-950/20">
                  <input type="checkbox" class="checkbox" :checked="isSelected(source.id, model)" @change="selectModel(source.id, model)" />
                  <span class="min-w-0 truncate">{{ model }}</span>
                </label>
              </div>
              <p v-if="source.models.length === 0" class="text-sm text-amber-600">该分组未配置可选模型列表。</p>
            </section>
          </div>
        </div>
      </form>
      <template #footer><button class="btn btn-secondary" type="button" @click="dialogOpen = false">取消</button><button class="btn btn-primary" form="custom-group-form" :disabled="saving || selected.size === 0">{{ saving ? '保存中…' : '保存' }}</button></template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { customGroupsAPI, type CustomGroupModelInput } from '@/api/customGroups'
import type { CustomGroupCandidate, UserCustomGroup } from '@/types'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const groups = ref<UserCustomGroup[]>([])
const candidates = ref<CustomGroupCandidate[]>([])
const loading = ref(true)
const saving = ref(false)
const dialogOpen = ref(false)
const editing = ref<UserCustomGroup | null>(null)
const name = ref('')
const selected = ref(new Map<string, CustomGroupModelInput>())
const modelKey = (model: string) => model.toLowerCase()

const load = async () => { loading.value = true; try { [groups.value, candidates.value] = await Promise.all([customGroupsAPI.list(), customGroupsAPI.candidates()]) } finally { loading.value = false } }
const startCreate = () => { editing.value = null; name.value = ''; selected.value = new Map(); dialogOpen.value = true }
const startEdit = (group: UserCustomGroup) => { editing.value = group; name.value = group.name; selected.value = new Map(group.models.map(m => [modelKey(m.public_model), { public_model: m.public_model, source_group_id: m.source_group_id, source_model: m.source_model }])); dialogOpen.value = true }
const isSelected = (sourceId: number, model: string) => selected.value.get(modelKey(model))?.source_group_id === sourceId
const selectModel = (sourceId: number, model: string) => { const next = new Map(selected.value); const key = modelKey(model); if (next.get(key)?.source_group_id === sourceId) next.delete(key); else next.set(key, { public_model: model, source_group_id: sourceId, source_model: model }); selected.value = next }
const save = async () => { saving.value = true; try { const models = [...selected.value.values()]; if (editing.value) await customGroupsAPI.update(editing.value.id, { name: name.value, models }); else await customGroupsAPI.create(name.value, models); dialogOpen.value = false; await load(); app.showSuccess('自定义分组已保存') } catch (e: any) { app.showError(e?.message || '保存失败') } finally { saving.value = false } }
const toggle = async (group: UserCustomGroup) => { await customGroupsAPI.update(group.id, { status: group.status === 'active' ? 'disabled' : 'active' }); await load() }
const remove = async (group: UserCustomGroup) => { if (!window.confirm(`确定删除“${group.name}”吗？`)) return; try { await customGroupsAPI.delete(group.id); await load() } catch (e: any) { app.showError(e?.message || '仍有 API Key 绑定该分组，无法删除') } }
onMounted(load)
</script>
