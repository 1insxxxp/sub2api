<template>
  <div v-if="ordered.length" class="mt-1 space-y-1 break-words">
    <p :class="color(ordered[0].level)">{{ label(ordered[0]) }}</p>
    <details v-if="ordered.length > 1">
      <summary class="cursor-pointer text-gray-500">{{ t('admin.accounts.acceptance.moreFindings', { count: ordered.length - 1 }) }}</summary>
      <p v-for="finding in ordered.slice(1)" :key="finding.code" class="mt-1" :class="color(finding.level)">{{ label(finding) }}</p>
    </details>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AcceptanceResult } from '@/api/admin/channelAcceptance'

type Finding = AcceptanceResult['findings'][number]
const props = defineProps<{ findings: Map<string, Finding> }>()
const { t, te } = useI18n()
const rank = (level: string) => level === 'error' ? 0 : level === 'warning' ? 1 : 2
const ordered = computed(() => [...props.findings.values()].sort((a, b) => rank(a.level) - rank(b.level)))
const color = (level: string) => level === 'error' ? 'text-red-600' : level === 'warning' ? 'text-amber-700' : ''
function label(finding: Finding) {
  const key = `admin.accounts.acceptance.verdicts.${finding.code}`
  return te?.(key) ? t(key) : finding.message
}
</script>
