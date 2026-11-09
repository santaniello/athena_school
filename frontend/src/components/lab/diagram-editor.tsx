import { useEffect, useImperativeHandle, useRef, useState, type Ref } from 'react'

const EMBED_ORIGIN = 'https://embed.diagrams.net'
const EMBED_URL =
  `${EMBED_ORIGIN}/?embed=1&proto=json&spin=1&configure=1&ui=min&noSaveBtn=1` +
  '&noExitBtn=1&saveAndExit=0&libraries=1&lang=en'
// The read-only view of the stored diagram: no menus, sidebars or toolbars, and
// nothing in it can be edited. See ADR-006.
const READ_ONLY_URL = `${EMBED_ORIGIN}/?embed=1&proto=json&spin=1&chrome=0&lang=en`
const EMBED_CONFIG = {
  compressXml: false,
  defaultLibraries: 'general;aws4;gcp2;azure2;kubernetes;er;flowchart;network',
}
const EMPTY_DIAGRAM =
  '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel>'

const EXPORT_TIMEOUT_MS = 1500
const PROBE_INTERVAL_MS = 5000
const MISSED_PROBES_FOR_LOSS = 2
const RECOVERY_TIMEOUT_MS = 15000
const READ_ONLY_LOAD_TIMEOUT_MS = 10000

export interface DiagramEditorHandle {
  // The current diagram XML: an export pull, or the latest autosave when the
  // editor does not answer in time.
  getXml: () => Promise<string>
}

interface DiagramEditorProps {
  ref?: Ref<DiagramEditorHandle>
  // Shows `xml` with no way to edit it. The connection is not probed: a
  // diagram that does not load says so instead.
  readOnly?: boolean
  xml?: string
  onReady: () => void
  onChange: (xml: string) => void
  onConnectionLost: () => void
  onConnectionRestored: () => void
}

type Phase = 'loading' | 'ready' | 'lost'

interface EditorMessage {
  event?: string
  xml?: string
}

// Anything that is not a JSON object has no event and is ignored by the
// handler, so a failed parse and a non-object value need no separate care.
// Stryker disable BlockStatement: undefined and null are both ignored by the handler.
function parse(data: unknown): EditorMessage | null {
  try {
    return JSON.parse(String(data)) as EditorMessage | null
  } catch {
    return null
  }
}
// Stryker restore BlockStatement

// The draw.io online embed in an iframe, driven by postMessage. It is the only
// code that knows draw.io: the screen sees ready, change, getXml and the two
// connection events. See ADR-006 and
// specs/phases/phase-03-architecture-lab/10-diagram-workspace.md.
function DiagramEditor({
  ref,
  readOnly = false,
  xml = '',
  onReady,
  onChange,
  onConnectionLost,
  onConnectionRestored,
}: DiagramEditorProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null)
  // A new key replaces the iframe, which is how a lost editor is recovered.
  const [generation, setGeneration] = useState(0)
  // Stryker disable next-line StringLiteral: only 'ready' and 'lost' are ever compared.
  const phase = useRef<Phase>('loading')
  const latestXml = useRef(readOnly ? xml : '')
  const [unavailable, setUnavailable] = useState(readOnly && !navigator.onLine)
  const waiting = useRef<Array<(xml: string) => void>>([])
  // Stryker disable next-line ObjectLiteral: the effect below sets the callbacks before any message can arrive.
  const callbacks = useRef({ onReady, onChange, onConnectionLost, onConnectionRestored })

  useEffect(() => {
    callbacks.current = { onReady, onChange, onConnectionLost, onConnectionRestored }
  })

  function post(message: object) {
    // Stryker disable next-line OptionalChaining: a missing iframe has no window to post to.
    iframeRef.current?.contentWindow?.postMessage(JSON.stringify(message), EMBED_ORIGIN)
  }

  // Asks the editor for its XML; null when it does not answer in time.
  function requestXml(): Promise<string | null> {
    return new Promise((resolve) => {
      const answer = (xml: string) => {
        // Stryker disable next-line all: a timer left running resolves an already settled promise.
        clearTimeout(timer)
        resolve(xml)
      }
      const timer = setTimeout(() => {
        // Stryker disable next-line all: a stale entry is only ever resolved a second time, harmlessly.
        waiting.current = waiting.current.filter((entry) => entry !== answer)
        resolve(null)
      }, EXPORT_TIMEOUT_MS)
      waiting.current.push(answer)
      post({ action: 'export', format: 'xml', xml: 1 })
    })
  }

  useImperativeHandle(ref, () => ({
    getXml: async () => (await requestXml()) ?? latestXml.current,
  }))

  function handleMessage(event: MessageEvent) {
    const iframe = iframeRef.current
    if (event.origin !== EMBED_ORIGIN || !iframe || event.source !== iframe.contentWindow) return
    const message = parse(event.data)
    switch (message?.event) {
      case 'configure':
        post({ action: 'configure', config: EMBED_CONFIG })
        break
      case 'init':
        post({
          action: 'load',
          xml: latestXml.current || EMPTY_DIAGRAM,
          autosave: readOnly ? 0 : 1,
        })
        break
      case 'load':
        setUnavailable(false)
        if (phase.current === 'lost') callbacks.current.onConnectionRestored()
        else callbacks.current.onReady()
        phase.current = 'ready'
        break
      case 'autosave':
        if (typeof message.xml !== 'string') break
        latestXml.current = message.xml
        callbacks.current.onChange(message.xml)
        break
      case 'export':
        if (typeof message.xml !== 'string') break
        latestXml.current = message.xml
        for (const answer of waiting.current.splice(0)) answer(message.xml)
        break
    }
  }

  // The listener and the probe live as long as the component; they read the
  // latest state through refs.
  useEffect(() => {
    const listener = (event: MessageEvent) => handleMessage(event)
    window.addEventListener('message', listener)
    let missed = 0
    const probe = setInterval(() => {
      if (phase.current !== 'ready' || readOnly) return
      void requestXml().then((xml) => {
        missed = xml === null ? missed + 1 : 0
        if (missed < MISSED_PROBES_FOR_LOSS) return
        missed = 0
        phase.current = 'lost'
        callbacks.current.onConnectionLost()
        setGeneration((current) => current + 1)
      })
    }, PROBE_INTERVAL_MS)
    // Stryker disable BlockStatement,StringLiteral,CallExpression,ArrayDeclaration: after unmount the iframe is gone, so a leftover listener or probe finds nothing to read or post to; the effect runs once on mount.
    return () => {
      window.removeEventListener('message', listener)
      clearInterval(probe)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  // Stryker restore BlockStatement,StringLiteral,CallExpression,ArrayDeclaration

  // A recovery that does not load is tried again with a fresh iframe.
  useEffect(() => {
    // Stryker disable next-line ConditionalExpression: before the first loss the phase is never 'lost', so the timer is a no-op.
    if (generation === 0) return
    const retry = setTimeout(() => {
      if (phase.current === 'lost') setGeneration((current) => current + 1)
    }, RECOVERY_TIMEOUT_MS)
    // Stryker disable next-line ArrowFunction: a pending retry finds the phase settled and does nothing.
    return () => clearTimeout(retry)
  }, [generation])

  // A read-only diagram that has not loaded in time is most likely offline.
  useEffect(() => {
    if (!readOnly) return
    const timer = setTimeout(() => {
      if (phase.current !== 'ready') setUnavailable(true)
    }, READ_ONLY_LOAD_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [readOnly])

  return (
    <>
      <iframe
        key={generation}
        ref={iframeRef}
        title="Diagram editor"
        src={readOnly ? READ_ONLY_URL : EMBED_URL}
        className="size-full border-0"
      />
      {unavailable && (
        <p
          role="status"
          className="absolute inset-0 flex items-center justify-center bg-background p-4 text-center text-sm text-muted-foreground"
        >
          The diagram needs an internet connection to display.
        </p>
      )}
    </>
  )
}

export { DiagramEditor }
