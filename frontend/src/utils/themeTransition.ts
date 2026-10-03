import { nextTick } from 'vue'

let transitioning = false

/** Keep the old theme visible while the new theme expands over it. */
export async function revealTheme(button: HTMLElement | null, apply: () => void): Promise<void> {
  if (transitioning) return
  if (!document.startViewTransition || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    apply()
    return
  }

  const root = document.documentElement
  const rect = button?.getBoundingClientRect()
  const x = Math.max(0, Math.min(window.innerWidth, rect ? rect.left + rect.width / 2 : window.innerWidth / 2))
  const y = Math.max(0, Math.min(window.innerHeight, rect ? rect.top + rect.height / 2 : window.innerHeight / 2))
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  let applied = false
  const update = async () => {
    if (applied) return
    applied = true
    apply()
    await nextTick()
  }

  transitioning = true
  root.style.setProperty('--theme-reveal-x', `${x}px`)
  root.style.setProperty('--theme-reveal-y', `${y}px`)
  root.style.setProperty('--theme-reveal-radius', `${Math.ceil(radius)}px`)
  root.classList.add('theme-reveal')
  try {
    const transition = document.startViewTransition(update)
    // A hidden tab or another transition can skip the animation.
    void transition.ready.catch(() => {})
    await transition.finished
  } catch {
    await update()
  } finally {
    root.classList.remove('theme-reveal')
    for (const property of ['x', 'y', 'radius']) root.style.removeProperty(`--theme-reveal-${property}`)
    transitioning = false
  }
}
