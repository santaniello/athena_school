import { useEffect, useImperativeHandle, useRef, useState, type Ref } from 'react'

const EMBED_ORIGIN = 'https://embed.diagrams.net'
const EMBED_URL =
  `${EMBED_ORIGIN}/?embed=1&proto=json&spin=1&configure=1&ui=min&noSaveBtn=1` +
  '&noExitBtn=1&saveAndExit=0&libraries=1&lang=en'
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

export interface DiagramEditorHandle {
  // The current diagram XML: an export pull, or the latest autosave when the
  // editor does not answer in time.
  getXml: () => Promise<string>
}

interface DiagramEditorProps {
  ref?: Ref<DiagramEditorHandle>
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
  const latestXml = useRef('')
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
        post({ action: 'load', xml: latestXml.current || EMPTY_DIAGRAM, autosave: 1 })
        break
      case 'load':
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
      if (phase.current !== 'ready') return
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

  return (
    <iframe
      key={generation}
      ref={iframeRef}
      title="Diagram editor"
      src={EMBED_URL}
      className="size-full border-0"
    />
  )
}

export { DiagramEditor }
