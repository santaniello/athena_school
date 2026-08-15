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
