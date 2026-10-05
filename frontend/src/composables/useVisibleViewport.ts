import { onMounted, onUnmounted, ref } from 'vue'

// Fixed overlays use layout-viewport coordinates; Safari can pan the visible
// viewport independently when its browser chrome or keyboard changes.
export function useVisibleViewport() {
  const viewport = ref({ top: 0, left: 0, width: window.innerWidth, height: window.innerHeight })
  const update = () => {
    const visual = window.visualViewport
    viewport.value = {
      top: visual?.offsetTop ?? 0,
      left: visual?.offsetLeft ?? 0,
      width: visual?.width ?? window.innerWidth,
      height: visual?.height ?? window.innerHeight,
    }
  }
  update()
  onMounted(() => {
    window.addEventListener('resize', update)
    window.visualViewport?.addEventListener('resize', update)
    window.visualViewport?.addEventListener('scroll', update)
  })
  onUnmounted(() => {
    window.removeEventListener('resize', update)
    window.visualViewport?.removeEventListener('resize', update)
    window.visualViewport?.removeEventListener('scroll', update)
  })
  return viewport
}
