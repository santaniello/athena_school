import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup, configure } from '@testing-library/react'

afterEach(cleanup)

// App's boot splash holds the initial view for a minimum of 1.4s (see
// App.tsx's MIN_SPLASH_MS) before any post-auth screen appears, which
// exceeds testing-library's default 1000ms findBy/waitFor budget.
configure({ asyncUtilTimeout: 3000 })

// jsdom does not implement the Pointer Events capture API or scrollIntoView,
// both of which Radix UI's Select relies on. Without these no-op polyfills,
// interacting with a Select in tests throws "hasPointerCapture is not a
// function" instead of exercising the component.
if (!Element.prototype.hasPointerCapture) {
  Element.prototype.hasPointerCapture = () => false
}
if (!Element.prototype.setPointerCapture) {
  Element.prototype.setPointerCapture = () => {}
}
if (!Element.prototype.releasePointerCapture) {
  Element.prototype.releasePointerCapture = () => {}
}
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {}
}

// jsdom does not implement ResizeObserver, which react-resizable-panels
// (the sidebar/chat split in app-shell.tsx) uses to track panel dimensions.
// Without this no-op polyfill, mounting a ResizablePanelGroup throws instead
// of exercising the component.
if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}

// jsdom has no layout engine, so getBoundingClientRect() always returns a
// zero-size rect at (0, 0) for every element. react-resizable-panels hit-tests
// pointerdown events against panel/separator edges (to support grabbing near
// the divider, not just exactly on it) using those rects — with everything
// collapsed to the same point, every click anywhere in the app falsely counts
// as a divider grab, and the library calls preventDefault() on it, silently
// swallowing focus on inputs like the Settings name field. Moving panel and
// separator rects away from the origin keeps real clicks out of that false
// match.
const getBoundingClientRect = Element.prototype.getBoundingClientRect
Element.prototype.getBoundingClientRect = function (this: Element) {
  if (this.hasAttribute('data-panel') || this.hasAttribute('data-separator')) {
    return {
      x: 99999,
      y: 99999,
      left: 99999,
      top: 99999,
      right: 99999,
      bottom: 99999,
      width: 0,
      height: 0,
      toJSON() {},
    } as DOMRect
  }
  return getBoundingClientRect.call(this)
}
