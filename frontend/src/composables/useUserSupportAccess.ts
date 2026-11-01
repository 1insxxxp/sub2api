import { computed, ref } from 'vue'
import { getAccess } from '@/api/admin/userSupport'

interface SupportActor {
  user?: { id: number; role?: string } | null
  isAdmin: boolean
  isSubAdmin?: boolean
  isAuthenticated?: boolean
  token?: string | null
}

// Memory only. A permission is bound to one actor AND login session.
const allowedActor = ref('')
const loadingActor = ref('')
let version = 0
let pending: { actor: string; promise: Promise<boolean> } | null = null

export function useUserSupportAccess(auth: SupportActor) {
  const identity = () => {
    const role = auth.user?.role
    if (!auth.user?.id || auth.isAuthenticated === false || !['admin', 'sub_admin'].includes(role ?? '')) return ''
    return `${auth.user.id}:${role}:${auth.token ?? ''}`
  }
  const canAccess = computed(() => {
    const actor = identity()
    return actor !== '' && (auth.user?.role === 'admin' || allowedActor.value === actor)
  })
  const refreshing = computed(() => loadingActor.value !== '' && loadingActor.value === identity())

  function revoke() {
    version++
    allowedActor.value = ''
    loadingActor.value = ''
    pending = null
  }

  async function refresh(): Promise<boolean> {
    const actor = identity()
    if (!actor) { revoke(); return false }
    if (pending?.actor === actor) return pending.promise
    const currentVersion = ++version
    loadingActor.value = actor
    const promise = (async () => {
      try {
        const result = await getAccess()
        if (actor !== identity() || currentVersion !== version) return false
        allowedActor.value = result.enabled === true ? actor : ''
        return result.enabled === true
      } catch {
        if (actor === identity() && currentVersion === version) allowedActor.value = ''
        return false
      } finally {
        if (currentVersion === version) {
          loadingActor.value = ''
          pending = null
        }
      }
    })()
    pending = { actor, promise }
    return promise
  }

  return { canAccess, refreshing, refresh, revoke }
}
