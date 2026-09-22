<template>
  <section
    class="model-first-output-timeout-settings space-y-6"
    data-test="model-first-output-timeout-panel"
  >
    <div class="admin-toolbar-surface">
      <div class="admin-toolbar">
        <div class="admin-toolbar-group flex-1">
          <span
            class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="form.enabled
              ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
              : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'"
          >
            {{ form.enabled ? t('admin.settings.modelFirstOutputTimeout.enabledStatus') : t('admin.settings.modelFirstOutputTimeout.disabledStatus') }}
          </span>
          <span class="admin-page-meta-chip">
            <span>{{ t('admin.settings.modelFirstOutputTimeout.profiles') }}</span>
            <strong>{{ profileKeys.length }}</strong>
          </span>
          <span class="admin-page-meta-chip">
            <span>{{ t('admin.settings.modelFirstOutputTimeout.overrides') }}</span>
            <strong>{{ Object.keys(form.platforms).length + Object.keys(form.models).length }}</strong>
          </span>
        </div>
        <button
          type="button"
          class="btn btn-primary inline-flex items-center gap-2"
          data-test="model-first-output-timeout-save"
          :disabled="loading || saving"
          @click="handleSave"
        >
          <Icon name="check" size="sm" />
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="admin-surface flex items-center justify-center p-10">
      <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
    </div>

    <template v-else>
      <p
        v-if="loadError"
        class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200"
        data-test="model-first-output-timeout-load-error"
      >
        {{ loadError }}
      </p>

      <section class="admin-surface overflow-hidden">
        <div class="admin-panel-header">
          <div class="flex items-center gap-3">
            <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
              <Icon name="clock" size="sm" />
            </div>
              <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.settings.modelFirstOutputTimeout.title') }}
              </h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.settings.modelFirstOutputTimeout.description') }}
              </p>
            </div>
          </div>
        </div>
        <div class="space-y-4 p-5">
          <label class="admin-form-section flex cursor-pointer items-center justify-between gap-4 !space-y-0 px-4 py-3">
            <span>
              <span class="block text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.modelFirstOutputTimeout.enabled') }}</span>
              <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.settings.modelFirstOutputTimeout.enabledHint') }}</span>
            </span>
            <input v-model="form.enabled" type="checkbox" class="h-5 w-5 rounded border-gray-300 text-primary-600 focus:ring-primary-500" data-test="model-first-output-timeout-enabled" />
          </label>
          <PolicyEditor
            :policy="form.default"
            :label="t('admin.settings.modelFirstOutputTimeout.globalPolicy')"
            :test-prefix="'model-first-output-timeout-default'"
            @update="updatePolicy(form.default, $event)"
          />
          <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.settings.modelFirstOutputTimeout.retryHint') }}</p>
        </div>
      </section>

      <section class="admin-surface overflow-hidden">
        <div class="admin-panel-header">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.modelFirstOutputTimeout.profileTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.settings.modelFirstOutputTimeout.profileHint') }}</p>
          </div>
        </div>
        <div class="grid gap-4 p-5 lg:grid-cols-3">
          <div v-for="profile in profileKeys" :key="profile" class="rounded-xl border border-gray-200/80 p-4 dark:border-dark-700">
            <PolicyEditor
              :policy="form.profiles[profile]"
              :label="t(`admin.settings.modelFirstOutputTimeout.profile.${profile}`)"
              :test-prefix="`model-first-output-timeout-profile-${profile}`"
              @update="updatePolicy(form.profiles[profile], $event)"
            />
          </div>
        </div>
      </section>

      <OverrideSection
        scope="platforms"
        :title="t('admin.settings.modelFirstOutputTimeout.platformTitle')"
        :hint="t('admin.settings.modelFirstOutputTimeout.platformHint')"
        :entries="form.platforms"
        :new-key="newPlatformKey"
        :test-prefix="'model-first-output-timeout-platform'"
        @update:new-key="newPlatformKey = $event"
        @add="addOverride('platforms')"
        @remove="removeOverride('platforms', $event)"
        @update-policy="updatePolicy($event.policy, $event.patch)"
      />

      <OverrideSection
        scope="models"
        :title="t('admin.settings.modelFirstOutputTimeout.modelTitle')"
        :hint="t('admin.settings.modelFirstOutputTimeout.modelHint')"
        :entries="form.models"
        :new-key="newModelKey"
        :test-prefix="'model-first-output-timeout-model'"
        @update:new-key="newModelKey = $event"
        @add="addOverride('models')"
        @remove="removeOverride('models', $event)"
        @update-policy="updatePolicy($event.policy, $event.patch)"
      />

      <p
        v-if="validationError || saveError"
        class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200"
        data-test="model-first-output-timeout-error"
      >
        {{ validationError || saveError }}
      </p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type {
  ModelFirstOutputTimeoutPolicy,
  ModelFirstOutputTimeoutSettings,
} from '@/api/admin/settings'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const profileKeys = ['gemini_flash', 'gemini_pro', 'gemini_thinking'] as const
const policyFields = [
  { key: 'target_seconds', label: 'targetSeconds' },
  { key: 'switch_seconds', label: 'switchSeconds' },
  { key: 'hard_cap_seconds', label: 'hardCapSeconds' },
] as const
type Scope = 'platforms' | 'models'

const makePolicy = (): ModelFirstOutputTimeoutPolicy => ({
  enabled: true,
  target_seconds: 20,
  switch_seconds: 20,
  hard_cap_seconds: 60,
})

const makeProfilePolicy = (profile: typeof profileKeys[number]): ModelFirstOutputTimeoutPolicy => {
  const values = {
    gemini_flash: [10, 10, 30],
    gemini_pro: [20, 20, 60],
    gemini_thinking: [30, 30, 90],
  }[profile]
  return { enabled: true, target_seconds: values[0], switch_seconds: values[1], hard_cap_seconds: values[2] }
}

const makeDefaultSettings = (): ModelFirstOutputTimeoutSettings => ({
  enabled: true,
  default: makePolicy(),
  profiles: {
    gemini_flash: makeProfilePolicy('gemini_flash'),
    gemini_pro: makeProfilePolicy('gemini_pro'),
    gemini_thinking: makeProfilePolicy('gemini_thinking'),
  },
  platforms: {},
  models: {},
})

const form = reactive<ModelFirstOutputTimeoutSettings>(makeDefaultSettings())
const loading = ref(true)
const saving = ref(false)
const loadError = ref('')
const saveError = ref('')
const validationError = ref('')
const newPlatformKey = ref('')
const newModelKey = ref('')

function clonePolicy(policy?: Partial<ModelFirstOutputTimeoutPolicy> | null): ModelFirstOutputTimeoutPolicy {
  const defaults = makePolicy()
  return {
    enabled: policy?.enabled !== false,
    target_seconds: Number(policy?.target_seconds) || defaults.target_seconds,
    switch_seconds: Number(policy?.switch_seconds) || defaults.switch_seconds,
    hard_cap_seconds: Number(policy?.hard_cap_seconds) || defaults.hard_cap_seconds,
  }
}

function applySettings(settings: ModelFirstOutputTimeoutSettings): void {
  const defaults = makeDefaultSettings()
  form.enabled = settings.enabled === true
  Object.assign(form.default, clonePolicy(settings.default))
  for (const profile of profileKeys) {
    Object.assign(form.profiles[profile], clonePolicy(settings.profiles?.[profile]))
  }
  for (const scope of ['platforms', 'models'] as const) {
    form[scope] = {}
    for (const [key, policy] of Object.entries(settings[scope] || {})) {
      form[scope][key] = clonePolicy(policy)
    }
  }
  // Keep reactive records initialized even when the backend returns a partial document.
  if (!form.default) form.default = defaults.default
}

function updatePolicy(policy: ModelFirstOutputTimeoutPolicy, patch: Partial<ModelFirstOutputTimeoutPolicy>): void {
  Object.assign(policy, patch)
}

function addOverride(scope: Scope): void {
  const keyRef = scope === 'platforms' ? newPlatformKey : newModelKey
  const key = keyRef.value.trim()
  if (!key || form[scope][key]) return
  form[scope][key] = makePolicy()
  keyRef.value = ''
}

function removeOverride(scope: Scope, key: string): void {
  delete form[scope][key]
}

function allPolicies(): ModelFirstOutputTimeoutPolicy[] {
  return [
    form.default,
    ...profileKeys.map((profile) => form.profiles[profile]),
    ...Object.values(form.platforms),
    ...Object.values(form.models),
  ]
}

function validate(): boolean {
  for (const policy of allPolicies()) {
    const values = [policy.target_seconds, policy.switch_seconds, policy.hard_cap_seconds]
    if (values.some((value) => !Number.isFinite(Number(value)) || Number(value) <= 0 || Number(value) > 900)) {
      validationError.value = t('admin.settings.modelFirstOutputTimeout.validationPositive')
      return false
    }
    if (!(policy.target_seconds <= policy.switch_seconds && policy.switch_seconds <= policy.hard_cap_seconds)) {
      validationError.value = t('admin.settings.modelFirstOutputTimeout.validationOrder')
      return false
    }
  }
  validationError.value = ''
  return true
}

function buildPayload(): ModelFirstOutputTimeoutSettings {
  return {
    enabled: Boolean(form.enabled),
    default: clonePolicy(form.default),
    profiles: Object.fromEntries(profileKeys.map((profile) => [profile, clonePolicy(form.profiles[profile])])) as ModelFirstOutputTimeoutSettings['profiles'],
    platforms: Object.fromEntries(Object.entries(form.platforms).map(([key, policy]) => [key, clonePolicy(policy)])),
    models: Object.fromEntries(Object.entries(form.models).map(([key, policy]) => [key, clonePolicy(policy)])),
  }
}

async function loadSettings(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    applySettings(await adminAPI.settings.getModelFirstOutputTimeoutSettings())
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.settings.modelFirstOutputTimeout.loadFailed'))
    appStore.showError(loadError.value)
  } finally {
    loading.value = false
  }
}

async function handleSave(): Promise<void> {
  saveError.value = ''
  if (!validate()) return
  saving.value = true
  try {
    const updated = await adminAPI.settings.updateModelFirstOutputTimeoutSettings(buildPayload())
    applySettings(updated)
    appStore.showSuccess(t('admin.settings.modelFirstOutputTimeout.saved'))
  } catch (error) {
    saveError.value = extractApiErrorMessage(error, t('admin.settings.modelFirstOutputTimeout.saveFailed'))
    appStore.showError(saveError.value)
  } finally {
    saving.value = false
  }
}

const PolicyEditor = defineComponent({
  name: 'ModelFirstOutputTimeoutPolicyEditor',
  props: {
    policy: { type: Object as () => ModelFirstOutputTimeoutPolicy, required: true },
    label: { type: String, required: true },
    testPrefix: { type: String, required: true },
  },
  emits: ['update'],
  setup(props, { emit }) {
    return () => h('div', { class: 'space-y-3' }, [
      h('div', { class: 'flex items-center justify-between gap-3' }, [
        h('span', { class: 'text-sm font-semibold text-gray-900 dark:text-white' }, props.label),
        h('input', {
          type: 'checkbox', checked: props.policy.enabled,
          class: 'h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500',
          'data-test': `${props.testPrefix}-enabled`,
          onChange: (event: Event) => emit('update', { enabled: (event.target as HTMLInputElement).checked }),
        }),
      ]),
      h('div', { class: 'grid gap-3 sm:grid-cols-3' }, policyFields.map((field) => h('label', { class: 'block' }, [
        h('span', { class: 'mb-1 block text-xs font-medium text-gray-600 dark:text-dark-300' }, t(`admin.settings.modelFirstOutputTimeout.${field.label}`)),
        h('input', {
          type: 'number', min: 1, max: 900, step: 1,
          value: props.policy[field.key],
          class: 'input w-full',
          'data-test': `${props.testPrefix}-${field.key}`,
          onInput: (event: Event) => emit('update', { [field.key]: Number((event.target as HTMLInputElement).value) }),
        }),
      ]))),
    ])
  },
})

const OverrideSection = defineComponent({
  name: 'ModelFirstOutputTimeoutOverrideSection',
  components: { PolicyEditor },
  props: {
    scope: { type: String as () => Scope, required: true },
    title: { type: String, required: true },
    hint: { type: String, required: true },
    entries: { type: Object as () => Record<string, ModelFirstOutputTimeoutPolicy>, required: true },
    newKey: { type: String, required: true },
    testPrefix: { type: String, required: true },
  },
  emits: ['update:newKey', 'add', 'remove', 'update-policy'],
  setup(props, { emit }) {
    return () => h('section', { class: 'admin-surface overflow-hidden' }, [
      h('div', { class: 'admin-panel-header' }, [
        h('div', {}, [h('h3', { class: 'text-base font-semibold text-gray-900 dark:text-white' }, props.title), h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-dark-400' }, props.hint)]),
      ]),
      h('div', { class: 'space-y-4 p-5' }, [
        h('div', { class: 'flex flex-col gap-2 sm:flex-row' }, [
          h('input', {
            value: props.newKey, type: 'text', class: 'input flex-1',
            placeholder: t(`admin.settings.modelFirstOutputTimeout.${props.scope === 'models' ? 'modelPlaceholder' : 'platformPlaceholder'}`),
            'data-test': `${props.testPrefix}-new-key`,
            onInput: (event: Event) => emit('update:newKey', (event.target as HTMLInputElement).value),
            onKeydown: (event: KeyboardEvent) => { if (event.key === 'Enter') emit('add') },
          }),
          h('button', { type: 'button', class: 'btn btn-secondary', 'data-test': `${props.testPrefix}-add`, onClick: () => emit('add') }, `+ ${t('admin.settings.modelFirstOutputTimeout.addOverride')}`),
        ]),
        Object.entries(props.entries).length
          ? Object.entries(props.entries).map(([key, policy]) => h('div', { key, class: 'rounded-xl border border-gray-200/80 p-4 dark:border-dark-700', 'data-test': `${props.testPrefix}-override-${key}` }, [
              h('div', { class: 'mb-3 flex items-center justify-between gap-3' }, [h('code', { class: 'truncate text-sm font-semibold text-gray-800 dark:text-dark-100' }, key), h('button', { type: 'button', class: 'btn btn-secondary btn-sm text-red-600 dark:text-red-400', 'data-test': `${props.testPrefix}-remove-${key}`, onClick: () => emit('remove', key) }, t('common.delete'))]),
              h(PolicyEditor, { policy, label: key, testPrefix: `${props.testPrefix}-${key}`, onUpdate: (patch: Partial<ModelFirstOutputTimeoutPolicy>) => emit('update-policy', { policy, patch }) }),
            ]))
          : h('p', { class: 'text-sm text-gray-500 dark:text-dark-400' }, t('admin.settings.modelFirstOutputTimeout.noOverrides')),
      ]),
    ])
  },
})

onMounted(loadSettings)
</script>
