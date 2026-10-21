import { act } from '@testing-library/react'

// jsdom has no IntersectionObserver and no layout, so "is the element
// visible?" is a single switch a test flips. Like the real thing, a freshly
// observed element reports its current visibility on its own, which is what
// lets a sentinel that stays visible after a load trigger the next one.
let sentinelVisible = false
const liveObservers = new Set<MockIntersectionObserver>()

class MockIntersectionObserver implements IntersectionObserver {
  readonly root = null
  readonly rootMargin = ''
  readonly scrollMargin = ''
  readonly thresholds = []
  private targets = new Set<Element>()

  constructor(private readonly callback: IntersectionObserverCallback) {
    liveObservers.add(this)
  }

  observe(target: Element) {
    this.targets.add(target)
    queueMicrotask(() => {
      if (sentinelVisible && liveObservers.has(this) && this.targets.has(target)) {
        this.report(target, true)
      }
    })
  }

  unobserve(target: Element) {
    this.targets.delete(target)
  }

  disconnect() {
    this.targets.clear()
    liveObservers.delete(this)
  }

  takeRecords() {
    return []
  }

  report(target: Element, isIntersecting: boolean) {
    this.callback([{ target, isIntersecting } as IntersectionObserverEntry], this)
  }

  reportAll(isIntersecting: boolean) {
    this.targets.forEach((target) => this.report(target, isIntersecting))
  }
}

export function installIntersectionObserverMock() {
  globalThis.IntersectionObserver = MockIntersectionObserver
}

export function resetIntersectionObserverMock() {
  sentinelVisible = false
  liveObservers.clear()
}

// Sets whether observed elements are in view and notifies the observers that
// already watch one.
export function setSentinelVisible(visible: boolean) {
  sentinelVisible = visible
  act(() => {
    liveObservers.forEach((observer) => observer.reportAll(visible))
  })
}

// Makes elements observed from now on report as in view, without notifying
// the ones already watched — the state "visible before the screen mounted".
export function setSentinelVisibleOnObserve(visible: boolean) {
  sentinelVisible = visible
}
