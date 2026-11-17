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

  it('loads the starter diagram it was given instead of an empty one', () => {
    const callbacks = {
      onReady: vi.fn(),
      onChange: vi.fn(),
      onConnectionLost: vi.fn(),
      onConnectionRestored: vi.fn(),
    }
    const starter = '<mxGraphModel><root><mxCell id="0"/></root></mxGraphModel>'
    const view = render(<DiagramEditor xml={starter} {...callbacks} />)
    const iframe = currentIframe(view.container)
    const sent = sentTo(iframe)

    fromEditor(iframe, { event: 'init' })

    expect(sent.messages()).toEqual([{ action: 'load', xml: starter, autosave: 1 }])
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

  it('ignores an autosave or export that carries no XML', async () => {
    const { iframe, ref, onChange } = setup()
    makeReady(iframe)
    fromEditor(iframe, { event: 'autosave', xml: '<mxGraphModel>kept</mxGraphModel>' })
    onChange.mockClear()

    fromEditor(iframe, { event: 'autosave' })
    fromEditor(iframe, { event: 'export' })
    const pending = ref.current!.getXml()
    await advance(1500)

    expect(onChange).not.toHaveBeenCalled()
    await expect(pending).resolves.toBe('<mxGraphModel>kept</mxGraphModel>')
  })

  it('ignores messages that are JSON but not an event', () => {
    const { iframe, onReady } = setup()

    act(() => {
      for (const data of ['null', '5', '"load"']) {
        window.dispatchEvent(
          new MessageEvent('message', { data, origin: ORIGIN, source: iframe.contentWindow }),
        )
      }
    })

    expect(onReady).not.toHaveBeenCalled()
  })

  it('calls the callbacks of the latest render', () => {
    const first = vi.fn()
    const second = vi.fn()
    const props = { onReady: vi.fn(), onConnectionLost: vi.fn(), onConnectionRestored: vi.fn() }
    const view = render(<DiagramEditor {...props} onChange={first} />)
    const iframe = currentIframe(view.container)
    view.rerender(<DiagramEditor {...props} onChange={second} />)

    fromEditor(iframe, { event: 'autosave', xml: '<mxGraphModel/>' })

    expect(second).toHaveBeenCalledWith('<mxGraphModel/>')
    expect(first).not.toHaveBeenCalled()
  })

  it('does not probe an editor that has not loaded yet', async () => {
    const { sent, onConnectionLost } = setup()

    await advance(60000)

    expect(sent.messages()).toEqual([])
    expect(onConnectionLost).not.toHaveBeenCalled()
  })

  it('keeps recreating the iframe while the recovery does not load', async () => {
    const { view, iframe } = setup()
    makeReady(iframe)
    await advance(5000 + 1500 + 5000)
    const seen = new Set([iframe, currentIframe(view.container)])

    for (let i = 0; i < 3; i++) {
      await advance(15000)
      seen.add(currentIframe(view.container))
    }

    expect(seen.size).toBe(5)
  })

  it('stops listening and probing once unmounted', async () => {
    const { view, iframe, onChange, sent } = setup()
    makeReady(iframe)
    view.unmount()
    sent.spy.mockClear()

    fromEditor(iframe, { event: 'autosave', xml: '<mxGraphModel/>' })
    await advance(30000)

    expect(onChange).not.toHaveBeenCalled()
    expect(sent.spy).not.toHaveBeenCalled()
  })
})

describe('DiagramEditor read-only', () => {
  const STORED =
    '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel>'

  function setupReadOnly(online = true) {
    vi.spyOn(window.navigator, 'onLine', 'get').mockReturnValue(online)
    const callbacks = {
      onReady: vi.fn(),
      onChange: vi.fn(),
      onConnectionLost: vi.fn(),
      onConnectionRestored: vi.fn(),
    }
    const view = render(<DiagramEditor readOnly xml={STORED} {...callbacks} />)
    const iframe = currentIframe(view.container)
    return { ...callbacks, view, iframe, sent: sentTo(iframe) }
  }

  const MESSAGE = 'The diagram needs an internet connection to display.'

  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('embeds draw.io with no chrome and no editing configuration', () => {
    const { iframe } = setupReadOnly()

    const url = new URL(iframe.src)
    expect(url.origin).toBe(ORIGIN)
    expect(Object.fromEntries(url.searchParams)).toEqual({
      embed: '1',
      proto: 'json',
      spin: '1',
      chrome: '0',
      lang: 'en',
    })
  })

  it('answers init by loading the stored diagram with autosave off', () => {
    const { iframe, sent } = setupReadOnly()

    fromEditor(iframe, { event: 'init' })

    expect(sent.messages()).toEqual([{ action: 'load', xml: STORED, autosave: 0 }])
  })

  it('never probes the editor, so a slow one is not replaced', async () => {
    const { iframe, sent, onConnectionLost } = setupReadOnly()
    fromEditor(iframe, { event: 'init' })
    fromEditor(iframe, { event: 'load' })
    sent.spy.mockClear()

    await advance(60000)

    expect(sent.spy).not.toHaveBeenCalled()
    expect(onConnectionLost).not.toHaveBeenCalled()
  })

  it('shows no message while the diagram loads in time', async () => {
    const { iframe, view } = setupReadOnly()

    await advance(9000)
    fromEditor(iframe, { event: 'load' })
    await advance(60000)

    expect(view.queryByText(MESSAGE)).not.toBeInTheDocument()
  })

  it('says the diagram needs a connection when it does not load in time', async () => {
    const { view } = setupReadOnly()

    await advance(10000)

    expect(view.getByText(MESSAGE)).toBeInTheDocument()
  })

  it('says it at once when the machine is offline', () => {
    const { view } = setupReadOnly(false)

    expect(view.getByText(MESSAGE)).toBeInTheDocument()
  })

  it('takes the message back when the diagram loads late', async () => {
    const { iframe, view } = setupReadOnly()
    await advance(10000)

    fromEditor(iframe, { event: 'load' })

    expect(view.queryByText(MESSAGE)).not.toBeInTheDocument()
  })

  it('does not show the message in the editing mode', async () => {
    const { view } = setup()

    await advance(60000)

    expect(view.queryByText(MESSAGE)).not.toBeInTheDocument()
  })
})
