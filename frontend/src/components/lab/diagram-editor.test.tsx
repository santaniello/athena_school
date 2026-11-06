import { createRef } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, render } from '@testing-library/react'
import { DiagramEditor, type DiagramEditorHandle } from './diagram-editor'

const ORIGIN = 'https://embed.diagrams.net'

function currentIframe(container: HTMLElement): HTMLIFrameElement {
  const iframe = container.querySelector('iframe')
  if (!iframe) throw new Error('no iframe rendered')
  return iframe
}

// Delivers a message as the draw.io embed would, from its origin and window.
function fromEditor(iframe: HTMLIFrameElement, message: object, origin = ORIGIN) {
  act(() => {
    window.dispatchEvent(
      new MessageEvent('message', {
        data: JSON.stringify(message),
        origin,
        source: iframe.contentWindow,
      }),
    )
  })
}

// Spies on what the wrapper sends to the editor, as parsed messages.
function sentTo(iframe: HTMLIFrameElement) {
  const spy = vi.spyOn(iframe.contentWindow!, 'postMessage')
  return {
    spy,
    messages: () =>
      spy.mock.calls.map(([data]) => JSON.parse(data as string) as Record<string, unknown>),
  }
}

function setup() {
  const callbacks = {
    onReady: vi.fn(),
    onChange: vi.fn(),
    onConnectionLost: vi.fn(),
    onConnectionRestored: vi.fn(),
  }
  const ref = createRef<DiagramEditorHandle>()
  const view = render(<DiagramEditor ref={ref} {...callbacks} />)
  const iframe = currentIframe(view.container)
  const sent = sentTo(iframe)
  return { ...callbacks, ref, view, iframe, sent }
}

// Moves the clock inside act, so the state updates timers cause are rendered.
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

// Brings the editor to the ready state.
function makeReady(iframe: HTMLIFrameElement) {
  fromEditor(iframe, { event: 'configure' })
  fromEditor(iframe, { event: 'init' })
  fromEditor(iframe, { event: 'load' })
}

describe('DiagramEditor', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('embeds draw.io with the fixed configuration', () => {
    const { iframe } = setup()

    const url = new URL(iframe.src)
    expect(url.origin).toBe(ORIGIN)
    expect(Object.fromEntries(url.searchParams)).toEqual({
      embed: '1',
      proto: 'json',
      spin: '1',
      configure: '1',
      ui: 'min',
      noSaveBtn: '1',
      noExitBtn: '1',
      saveAndExit: '0',
      libraries: '1',
      lang: 'en',
    })
  })

  it('answers configure with the fixed configuration, to the embed origin only', () => {
    const { iframe, sent } = setup()

    fromEditor(iframe, { event: 'configure' })

    expect(sent.messages()).toEqual([
      {
        action: 'configure',
        config: {
          compressXml: false,
          defaultLibraries: 'general;aws4;gcp2;azure2;kubernetes;er;flowchart;network',
        },
      },
    ])
    expect(sent.spy.mock.calls[0][1]).toBe(ORIGIN)
  })

  it('answers init by loading an empty diagram with autosave on', () => {
    const { iframe, sent } = setup()

    fromEditor(iframe, { event: 'init' })

    const [load] = sent.messages()
    expect(load).toMatchObject({ action: 'load', autosave: 1 })
    expect(load.xml).toContain('<mxGraphModel>')
  })

  it('reports ready once the editor acknowledges the load', () => {
    const { iframe, onReady } = setup()

    makeReady(iframe)

    expect(onReady).toHaveBeenCalledTimes(1)
  })

  it('reports each autosave with the full XML', () => {
    const { iframe, onChange } = setup()
    makeReady(iframe)

    fromEditor(iframe, { event: 'autosave', xml: '<mxGraphModel>one</mxGraphModel>' })

    expect(onChange).toHaveBeenCalledWith('<mxGraphModel>one</mxGraphModel>')
  })

  it.each([
    ['another origin', 'https://evil.example', true],
    ['another window', ORIGIN, false],
  ])('ignores messages from %s', (_name, origin, fromIframe) => {
    const { iframe, onReady, onChange, sent } = setup()

    act(() => {
      for (const message of [
        { event: 'configure' },
        { event: 'load' },
        { event: 'autosave', xml: 'x' },
      ]) {
        window.dispatchEvent(
          new MessageEvent('message', {
            data: JSON.stringify(message),
            origin,
            source: fromIframe ? iframe.contentWindow : window,
          }),
        )
      }
    })

    expect(onReady).not.toHaveBeenCalled()
    expect(onChange).not.toHaveBeenCalled()
    expect(sent.messages()).toEqual([])
  })

  it('ignores a message that is not JSON', () => {
    const { iframe, onReady } = setup()

    act(() => {
      window.dispatchEvent(
        new MessageEvent('message', {
          data: 'not json',
          origin: ORIGIN,
          source: iframe.contentWindow,
        }),
      )
    })

    expect(onReady).not.toHaveBeenCalled()
  })

  it('pulls the XML with an export request and returns the answer', async () => {
    const { iframe, ref, sent } = setup()
    makeReady(iframe)

    const pending = ref.current!.getXml()
    expect(sent.messages().at(-1)).toEqual({ action: 'export', format: 'xml', xml: 1 })
    fromEditor(iframe, { event: 'export', xml: '<mxGraphModel>pulled</mxGraphModel>' })

    await expect(pending).resolves.toBe('<mxGraphModel>pulled</mxGraphModel>')
  })

  it('falls back to the latest autosave when the export is not answered in 1.5 s', async () => {
    const { iframe, ref } = setup()
    makeReady(iframe)
    fromEditor(iframe, { event: 'autosave', xml: '<mxGraphModel>saved</mxGraphModel>' })

    const pending = ref.current!.getXml()
    await advance(1500)

    await expect(pending).resolves.toBe('<mxGraphModel>saved</mxGraphModel>')
  })

  it('does not report a lost connection while the editor keeps answering', async () => {
    const { iframe, onConnectionLost, sent } = setup()
    makeReady(iframe)

    for (let i = 0; i < 4; i++) {
      await advance(5000)
      fromEditor(iframe, { event: 'export', xml: '<mxGraphModel/>' })
    }

    expect(sent.messages().filter((m) => m.action === 'export')).toHaveLength(4)
    expect(onConnectionLost).not.toHaveBeenCalled()
  })

  it('reports a lost connection after two probes go unanswered', async () => {
    const { iframe, onConnectionLost } = setup()
    makeReady(iframe)

    await advance(5000 + 1500)
    expect(onConnectionLost).not.toHaveBeenCalled()
    await advance(5000)

    expect(onConnectionLost).toHaveBeenCalledTimes(1)
  })

  it('counts only consecutive missed probes', async () => {
    const { iframe, onConnectionLost } = setup()
    makeReady(iframe)

    await advance(5000 + 1500)
    await advance(3500)
    fromEditor(iframe, { event: 'export', xml: '<mxGraphModel/>' })
    await advance(5000 + 1500)

    expect(onConnectionLost).not.toHaveBeenCalled()
  })

  it('recreates the iframe, reloads the latest XML and reports the recovery', async () => {
    const { view, iframe, onConnectionLost, onConnectionRestored, onReady } = setup()
    makeReady(iframe)
    fromEditor(iframe, { event: 'autosave', xml: '<mxGraphModel>kept</mxGraphModel>' })

    await advance(5000 + 1500 + 5000)
    expect(onConnectionLost).toHaveBeenCalledTimes(1)
    const fresh = currentIframe(view.container)
    expect(fresh).not.toBe(iframe)
    const sent = sentTo(fresh)
    fromEditor(fresh, { event: 'configure' })
    fromEditor(fresh, { event: 'init' })
    expect(sent.messages().at(-1)).toMatchObject({
      action: 'load',
      xml: '<mxGraphModel>kept</mxGraphModel>',
    })
    expect(onConnectionRestored).not.toHaveBeenCalled()
    fromEditor(fresh, { event: 'load' })

    expect(onConnectionRestored).toHaveBeenCalledTimes(1)
    expect(onReady).toHaveBeenCalledTimes(1)
  })

  it('ignores the old iframe once it was replaced', async () => {
    const { view, iframe, onChange } = setup()
    makeReady(iframe)
    await advance(5000 + 1500 + 5000)
    expect(currentIframe(view.container)).not.toBe(iframe)

    fromEditor(iframe, { event: 'autosave', xml: 'stale' })

    expect(onChange).not.toHaveBeenCalled()
  })

  it('recreates the iframe again when the recovery does not load in 15 s', async () => {
    const { view, iframe } = setup()
    makeReady(iframe)
    await advance(5000 + 1500 + 5000)
    const first = currentIframe(view.container)

    await advance(15000)

    expect(currentIframe(view.container)).not.toBe(first)
  })

  it('does not recreate the iframe once the recovery loaded', async () => {
    const { view, iframe } = setup()
    makeReady(iframe)
    await advance(5000 + 1500 + 5000)
    const fresh = currentIframe(view.container)
    // The recovered editor answers every probe.
    sentTo(fresh).spy.mockImplementation((data) => {
      if (JSON.parse(data as string).action === 'export') {
        fromEditor(fresh, { event: 'export', xml: '<mxGraphModel/>' })
      }
    })
    makeReady(fresh)

    await advance(30000)

    expect(currentIframe(view.container)).toBe(fresh)
  })
})
