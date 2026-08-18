import { afterEach, describe, expect, it, vi } from 'vitest'

// Each polyfill in setup.ts is guarded by `if (!Element.prototype.X)`, so it
// only installs when the environment doesn't already provide the API.
// setup.ts itself already ran once (via vitest's setupFiles) before any test
// runs, so these tests force it to re-run via vi.resetModules() + a fresh
// dynamic import, to exercise both the "already present" (skip) and "absent"
// (install) branches of each guard.
describe('test environment setup polyfills', () => {
  afterEach(() => {
    vi.resetModules()
  })

  it('installs a no-op hasPointerCapture only when the environment does not already provide one', async () => {
    const existing = () => true
    Element.prototype.hasPointerCapture = existing
    vi.resetModules()
    await import('./setup')
    expect(Element.prototype.hasPointerCapture).toBe(existing)

    Reflect.deleteProperty(Element.prototype, 'hasPointerCapture')
    vi.resetModules()
    await import('./setup')
    expect(Element.prototype.hasPointerCapture.call(document.createElement('div'), 1)).toBe(false)
  })

  it('installs a no-op setPointerCapture only when the environment does not already provide one', async () => {
    const existing = () => {}
    Element.prototype.setPointerCapture = existing
    vi.resetModules()
    await import('./setup')
    expect(Element.prototype.setPointerCapture).toBe(existing)

    Reflect.deleteProperty(Element.prototype, 'setPointerCapture')
    vi.resetModules()
    await import('./setup')
    expect(() =>
      Element.prototype.setPointerCapture.call(document.createElement('div'), 1),
    ).not.toThrow()
  })

  it('installs a no-op releasePointerCapture only when the environment does not already provide one', async () => {
    const existing = () => {}
    Element.prototype.releasePointerCapture = existing
    vi.resetModules()
    await import('./setup')
    expect(Element.prototype.releasePointerCapture).toBe(existing)

    Reflect.deleteProperty(Element.prototype, 'releasePointerCapture')
    vi.resetModules()
    await import('./setup')
    expect(() =>
      Element.prototype.releasePointerCapture.call(document.createElement('div'), 1),
    ).not.toThrow()
  })

  it('installs a no-op scrollIntoView only when the environment does not already provide one', async () => {
    const existing = () => {}
    Element.prototype.scrollIntoView = existing
    vi.resetModules()
    await import('./setup')
    expect(Element.prototype.scrollIntoView).toBe(existing)

    Reflect.deleteProperty(Element.prototype, 'scrollIntoView')
    vi.resetModules()
    await import('./setup')
    expect(() => Element.prototype.scrollIntoView.call(document.createElement('div'))).not.toThrow()
  })

  it('installs a no-op ResizeObserver only when the environment does not already provide one', async () => {
    Reflect.deleteProperty(globalThis, 'ResizeObserver')
    vi.resetModules()
    await import('./setup')
    expect(globalThis.ResizeObserver).toBeDefined()
    const observer = new globalThis.ResizeObserver(() => {})
    expect(() => observer.observe(document.createElement('div'))).not.toThrow()
    expect(() => observer.unobserve(document.createElement('div'))).not.toThrow()
    expect(() => observer.disconnect()).not.toThrow()
  })

  it('overrides getBoundingClientRect for panel and separator elements, but leaves other elements alone', () => {
    // setup.ts is already loaded globally by this point, so its
    // getBoundingClientRect override is already active — no reimport needed.
    const panel = document.createElement('div')
    panel.setAttribute('data-panel', '')
    expect(panel.getBoundingClientRect().x).toBe(99999)

    const separator = document.createElement('div')
    separator.setAttribute('data-separator', '')
    expect(separator.getBoundingClientRect().x).toBe(99999)

    const other = document.createElement('div')
    expect(other.getBoundingClientRect().x).not.toBe(99999)
  })
})
