import { afterEach, describe, expect, it, vi } from 'vitest'
import { revealTheme } from '../themeTransition'

afterEach(() => {
  vi.restoreAllMocks()
  delete (document as any).startViewTransition
  document.documentElement.classList.remove('theme-reveal')
})

function button() {
  const el = document.createElement('button')
  vi.spyOn(el, 'getBoundingClientRect').mockReturnValue({ left: 10, top: 20, width: 80, height: 40 } as DOMRect)
  return el
}

function motion(reduce = false) {
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: reduce } as MediaQueryList)
}

describe('theme reveal', () => {
  it('switches immediately without browser support', async () => {
    motion()
    const apply = vi.fn()
    await revealTheme(button(), apply)
    expect(apply).toHaveBeenCalledTimes(1)
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(false)
  })

  it('respects reduced motion', async () => {
    motion(true)
    const start = vi.fn()
    ;(document as any).startViewTransition = start
    const apply = vi.fn()
    await revealTheme(button(), apply)
    expect(start).not.toHaveBeenCalled()
    expect(apply).toHaveBeenCalledOnce()
  })

  it('reveals from the button center, prevents overlap and cleans up', async () => {
    motion()
    let finish!: () => void
    const finished = new Promise<void>(resolve => { finish = resolve })
    ;(document as any).startViewTransition = (update: () => Promise<void>) => {
      void update()
      return { finished, ready: Promise.resolve() }
    }
    const apply = vi.fn()
    const pending = revealTheme(button(), apply)
    expect(document.documentElement.style.getPropertyValue('--theme-reveal-x')).toBe('50px')
    expect(document.documentElement.style.getPropertyValue('--theme-reveal-y')).toBe('40px')
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(true)
    await revealTheme(button(), apply)
    expect(apply).toHaveBeenCalledOnce()
    finish()
    await pending
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(false)
  })

  it('still switches when snapshot creation throws', async () => {
    motion()
    ;(document as any).startViewTransition = () => { throw new Error('snapshot unavailable') }
    const apply = vi.fn()
    await revealTheme(button(), apply)
    expect(apply).toHaveBeenCalledOnce()
    expect(document.documentElement.classList.contains('theme-reveal')).toBe(false)
  })
})
