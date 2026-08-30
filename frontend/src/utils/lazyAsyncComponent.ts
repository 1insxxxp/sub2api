import { defineAsyncComponent, type Component } from 'vue'

const LAZY_CHUNK_RELOAD_KEY = 'lazy_chunk_reload_attempted'
const RELOAD_RETRY_WINDOW_MS = 10_000

export function isLazyChunkLoadError(error: unknown): boolean {
  const name = error instanceof Error ? error.name : ''
  const message = error instanceof Error ? error.message : String(error ?? '')

  return (
    name === 'ChunkLoadError' ||
    message.includes('Failed to fetch dynamically imported module') ||
    message.includes('Loading chunk') ||
    message.includes('Loading CSS chunk')
  )
}

function shouldReloadForLazyChunk(): boolean {
  if (typeof sessionStorage === 'undefined') {
    return true
  }

  const lastReload = sessionStorage.getItem(LAZY_CHUNK_RELOAD_KEY)
  const now = Date.now()
  return !lastReload || now - Number.parseInt(lastReload, 10) > RELOAD_RETRY_WINDOW_MS
}

function markLazyChunkReloadAttempt(): void {
  if (typeof sessionStorage === 'undefined') {
    return
  }
  sessionStorage.setItem(LAZY_CHUNK_RELOAD_KEY, Date.now().toString())
}

export function lazyAsyncComponent(loader: () => Promise<Component | { default: Component }>): Component {
  return defineAsyncComponent({
    loader,
    onError(error, _retry, fail) {
      if (isLazyChunkLoadError(error) && shouldReloadForLazyChunk()) {
        markLazyChunkReloadAttempt()
        globalThis.location?.reload()
        return
      }

      fail()
    }
  })
}
