import createDOMPurify from 'dompurify'
import { describe, expect, it } from 'vitest'
import { sanitizeSvg } from '@/utils/sanitize'

describe('DOMPurify security regressions', () => {
  // GHSA-cg2g-xh64-v4q7: removing disallowed nodes must not reassemble
  // a template expression inside an attached shadow tree.
  it.each(['{{7*7}}', '${7*7}', '<%7*7%>'])(
    'scrubs split shadow-root template expression %s',
    (expression) => {
      const purify = createDOMPurify(window)
      const host = document.createElement('div')
      const shadow = host.attachShadow({ mode: 'open' })
      for (const character of expression) {
        shadow.append(document.createTextNode(character), document.createElement('script'))
      }

      purify.sanitize(host, { IN_PLACE: true, SAFE_FOR_TEMPLATES: true })

      expect(shadow.textContent).not.toContain(expression)
      expect(shadow.querySelector('script')).toBeNull()
    }
  )

  // GHSA-mrv2-3pqr-j5jr: hooks may detach a parent or an unvisited sibling;
  // retained references must still have their event handlers neutralized.
  it.each(['parent', 'sibling'] as const)('neutralizes a hook-detached %s', (shape) => {
    const purify = createDOMPurify(window)
    const root = document.createElement('div')
    const section = document.createElement('section')
    const trigger = document.createElement('i')
    const image = document.createElement('img')
    image.setAttribute('onerror', 'window.__unexpectedExecution = true')
    section.append(trigger, image)
    root.append(section)
    purify.addHook('afterSanitizeAttributes', (node) => {
      if (node === trigger) (shape === 'parent' ? section : image).remove()
    })

    purify.sanitize(root, { IN_PLACE: true })

    expect(image.getAttribute('onerror')).toBeNull()
    expect(root.contains(image)).toBe(false)
  })

  it('keeps SVG drawing and filter content while removing executable content', () => {
    const clean = sanitizeSvg(
      '<svg viewBox="0 0 24 24"><defs><filter id="blur"><feGaussianBlur stdDeviation="1"/></filter></defs>' +
      '<path d="M0 0h24v24H0z" filter="url(#blur)"/><script>window.__unexpectedExecution=true</script>' +
      '<circle r="2" onload="window.__unexpectedExecution=true"/></svg>'
    )
    const rendered = document.createElement('div')
    rendered.innerHTML = clean

    expect(rendered.querySelector('svg')?.getAttribute('viewBox')).toBe('0 0 24 24')
    expect(rendered.querySelector('path')?.getAttribute('filter')).toBe('url(#blur)')
    expect(rendered.querySelector('feGaussianBlur')?.getAttribute('stdDeviation')).toBe('1')
    expect(rendered.querySelector('circle')).not.toBeNull()
    expect(rendered.querySelector('script, [onload]')).toBeNull()
  })
})
