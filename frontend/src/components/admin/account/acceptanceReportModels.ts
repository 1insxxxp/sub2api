import type { AcceptanceResult, AcceptanceSample } from '@/api/admin/channelAcceptance'

type Finding = AcceptanceResult['findings'][number]

interface ModelOverview {
  key: string
  target: AcceptanceSample['target']
  anomalies: number
  generations: number
  measured: number
  sources: Map<string, AcceptanceResult['source']>
  sourceFindings: Map<string, Finding>
  findings: Map<string, Finding>
}

export function reportTargetKey(target: AcceptanceSample['target']): string {
  return JSON.stringify([target.account_id, target.model, target.group_id ?? null])
}

export function summarizeAcceptanceModels(samples: AcceptanceSample[]): ModelOverview[] {
  const models = new Map<string, ModelOverview>()
  for (const sample of samples) {
    const { target, result } = sample
    const key = reportTargetKey(target)
    let row = models.get(key)
    if (!row) {
      row = {
        key, target, anomalies: 0, generations: 0, measured: 0,
        sources: new Map(), sourceFindings: new Map(), findings: new Map()
      }
      models.set(key, row)
    }
    if (!result) continue
    if (result.status === 'anomaly' || result.status === 'error'
      || result.findings?.some(finding => finding.level === 'warning' || finding.level === 'error')) row.anomalies++
    if (sample.protocol !== 'count_tokens') {
      row.generations++
      if (result.upstream_usage_final && result.raw_usage?.input != null && result.raw_usage.output != null) row.measured++
    }
    const source = result.source
    if (source?.kind) row.sources.set(JSON.stringify([source.kind, source.channel]), source)
    for (const finding of result.findings || []) {
      const bucket = finding.code.startsWith('source_') || sample.category === 'source' ? row.sourceFindings : row.findings
      if (!bucket.has(finding.code)) bucket.set(finding.code, finding)
    }
  }
  return [...models.values()]
}
