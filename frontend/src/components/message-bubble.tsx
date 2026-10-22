import { Children, isValidElement, memo, useEffect, useMemo, useRef, useState } from 'react'
import type { ComponentProps, ReactElement } from 'react'
import { Check, Copy } from 'lucide-react'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { CitationChip } from '@/components/citation-chip'
import { CITATION_TAG_NAME, remarkCitations, stripCitationMarkers } from '@/lib/citations'
import type { StudySource } from '@/lib/study'
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import bash from 'react-syntax-highlighter/dist/esm/languages/prism/bash'
import css from 'react-syntax-highlighter/dist/esm/languages/prism/css'
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go'
import javascript from 'react-syntax-highlighter/dist/esm/languages/prism/javascript'
import json from 'react-syntax-highlighter/dist/esm/languages/prism/json'
import jsx from 'react-syntax-highlighter/dist/esm/languages/prism/jsx'
import markdown from 'react-syntax-highlighter/dist/esm/languages/prism/markdown'
import markup from 'react-syntax-highlighter/dist/esm/languages/prism/markup'
import python from 'react-syntax-highlighter/dist/esm/languages/prism/python'
import sql from 'react-syntax-highlighter/dist/esm/languages/prism/sql'
import tsx from 'react-syntax-highlighter/dist/esm/languages/prism/tsx'
import typescript from 'react-syntax-highlighter/dist/esm/languages/prism/typescript'
import yaml from 'react-syntax-highlighter/dist/esm/languages/prism/yaml'
import { cva } from 'class-variance-authority'
import { cn } from '@/lib/utils'

// Stryker disable StringLiteral: react-syntax-highlighter's registerLanguage
// implementation ignores this first argument entirely — it forwards only the
// grammar module to refractor.register(), which derives the registered name
// from the module itself (see node_modules/react-syntax-highlighter/dist/esm/prism-light.js).
// Mutating these string literals cannot change runtime behavior.
SyntaxHighlighter.registerLanguage('bash', bash)
SyntaxHighlighter.registerLanguage('css', css)
SyntaxHighlighter.registerLanguage('go', go)
SyntaxHighlighter.registerLanguage('javascript', javascript)
SyntaxHighlighter.registerLanguage('json', json)
SyntaxHighlighter.registerLanguage('jsx', jsx)
SyntaxHighlighter.registerLanguage('markdown', markdown)
SyntaxHighlighter.registerLanguage('markup', markup)
SyntaxHighlighter.registerLanguage('python', python)
SyntaxHighlighter.registerLanguage('sql', sql)
SyntaxHighlighter.registerLanguage('tsx', tsx)
SyntaxHighlighter.registerLanguage('typescript', typescript)
SyntaxHighlighter.registerLanguage('yaml', yaml)
// Stryker restore StringLiteral

interface MessageBubbleProps {
  role: 'user' | 'assistant'
  content: string
  isStreaming?: boolean
  // The reply's full source list, so an inline [n] marker can resolve to a
  // CitationChip. Defaults to empty, which renders every marker as plain
  // text — the correct behavior with no sources to resolve against.
  sources?: StudySource[]
  // Omitted while streaming; forwarded to each CitationChip.
  onOpenCitation?: (source: StudySource) => void
}

const COPIED_RESET_DELAY_MS = 2000

// A module-level default keeps `sources` referentially stable across renders,
// which the memo() on MessageBubble relies on.
const EMPTY_SOURCES: StudySource[] = []

const messageBubbleVariants = cva(
  'max-w-[75%] rounded-2xl px-4 py-2.5 text-base leading-relaxed break-words',
  {
    variants: {
      role: {
        user: 'self-end rounded-br-sm bg-primary text-primary-foreground',
        assistant:
          'self-start rounded-bl-sm bg-card text-card-foreground ring-1 ring-foreground/10',
      },
    },
  },
)

function InlineCode(props: ComponentProps<'code'>) {
  return (
    <code
      data-slot="inline-code"
      className="rounded bg-muted px-1 py-0.5 font-mono text-[0.8em] text-primary"
      {...props}
    />
  )
}

// react-markdown v10 has no `inline` flag: a fenced block is a <pre> whose only
// child is a <code>, which is how it is told apart from inline code.
function PreRenderer({ children }: ComponentProps<'pre'>) {
  const codeElement = Children.toArray(children).find(isValidElement) as
    ReactElement<ComponentProps<'code'>> | undefined
  const { className, children: codeChildren } = codeElement?.props ?? {}
  const language = (className && /language-(\w+)/.exec(className)?.[1]) || DEFAULT_CODE_LANGUAGE

  return <CodeBlock language={language} code={String(codeChildren ?? '').replace(/\n$/, '')} />
}

const DEFAULT_CODE_LANGUAGE = 'text'

function useCopyToClipboard() {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)

  useEffect(() => () => clearTimeout(timer.current), [])

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      return
    }
    setCopied(true)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setCopied(false), COPIED_RESET_DELAY_MS)
  }

  return { copied, copy }
}

function CopyIcon({ copied }: { copied: boolean }) {
  return copied ? (
    <Check className="size-3.5" aria-hidden="true" />
  ) : (
    <Copy className="size-3.5" aria-hidden="true" />
  )
}

// How far outside the viewport a code block still gets highlighted, so it is
// ready before it scrolls into view.
const HIGHLIGHT_ROOT_MARGIN = '400px 0px'

const CODE_BLOCK_STYLE = {
  background: 'var(--color-code)',
  margin: 0,
  padding: '0.75rem 1rem',
  fontSize: '0.8125rem',
}
const CODE_TAG_STYLE = {
  fontFamily: 'var(--font-mono, ui-monospace, monospace)',
  color: 'var(--color-foreground)',
}

// Prism highlighting is the expensive part of a message: a long session holds
// hundreds of blocks, and highlighting them all while it opens freezes the
// window for seconds. A block is shown as plain monospace text, with the same
// box as the highlighted one, until it is near the viewport.
function CodeBlock({ language, code }: { language: string; code: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const { copied, copy } = useCopyToClipboard()
  const [highlight, setHighlight] = useState(typeof IntersectionObserver === 'undefined')

  useEffect(() => {
    const element = ref.current
    if (highlight || !element) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) setHighlight(true)
      },
      { rootMargin: HIGHLIGHT_ROOT_MARGIN },
    )
    observer.observe(element)
    return () => observer.disconnect()
  }, [highlight])

  return (
    <div
      ref={ref}
      data-slot="code-block-container"
      className="my-2 overflow-hidden rounded-md border border-border"
    >
      <div
        data-slot="code-block-header"
        className="flex items-center justify-between border-b border-border bg-card px-4 py-1.5 text-xs text-muted-foreground"
      >
        <span>{language}</span>
        <button
          type="button"
          onClick={() => void copy(code)}
          aria-label={copied ? 'Copied' : 'Copy code'}
          className="flex cursor-pointer items-center gap-1 hover:text-foreground"
        >
          <CopyIcon copied={copied} />
        </button>
      </div>
      <div data-slot="code-block">
        {highlight ? (
          <SyntaxHighlighter
            language={language}
            PreTag="div"
            style={oneDark}
            customStyle={CODE_BLOCK_STYLE}
            codeTagProps={{ style: CODE_TAG_STYLE }}
          >
            {code}
          </SyntaxHighlighter>
        ) : (
          <div style={{ ...CODE_BLOCK_STYLE, overflow: 'auto', lineHeight: 1.5 }}>
            <code style={{ ...CODE_TAG_STYLE, whiteSpace: 'pre' }}>{code}</code>
          </div>
        )}
      </div>
    </div>
  )
}

// The user bubble is solid primary (gold), so primary-coloured links and
// headings vanish into it; these overrides use a deep amber that stays readable.
const userMarkdownComponents: Components = {
  a: (props) => (
    <a
      className="font-semibold text-[oklch(0.3_0.08_55)] underline underline-offset-2 hover:text-[oklch(0.22_0.06_55)]"
      target="_blank"
      rel="noreferrer"
      {...props}
    />
  ),
  h1: (props) => (
    <h2 className="mb-2 font-heading text-xl font-semibold text-primary-foreground" {...props} />
  ),
  h2: (props) => (
    <h3 className="mb-2 font-heading text-lg font-semibold text-primary-foreground" {...props} />
  ),
  h3: (props) => (
    <h4 className="mb-1 font-heading text-base font-semibold text-primary-foreground" {...props} />
  ),
}

const baseMarkdownComponents: Components = {
  p: (props) => <p className="mb-2 last:mb-0" {...props} />,
  h1: (props) => (
    <h2 className="mb-2 font-heading text-xl font-semibold text-foreground" {...props} />
  ),
  h2: (props) => (
    <h3 className="mb-2 font-heading text-lg font-semibold text-foreground" {...props} />
  ),
  h3: (props) => (
    <h4 className="mb-1 font-heading text-base font-semibold text-primary" {...props} />
  ),
  ul: (props) => <ul className="mb-2 list-disc space-y-1 pl-5 last:mb-0" {...props} />,
  ol: (props) => <ol className="mb-2 list-decimal space-y-1 pl-5 last:mb-0" {...props} />,
  li: (props) => <li {...props} />,
  a: (props) => (
    <a
      className="text-primary underline underline-offset-2 hover:text-primary/80"
      target="_blank"
      rel="noreferrer"
      {...props}
    />
  ),
  strong: (props) => <strong className="font-semibold" {...props} />,
  blockquote: (props) => (
    <blockquote
      className="mb-2 border-l-2 border-primary/50 pl-3 text-muted-foreground last:mb-0"
      {...props}
    />
  ),
  hr: () => <hr className="my-3 border-border" />,
  table: (props) => <table className="mb-2 w-full border-collapse text-sm last:mb-0" {...props} />,
  th: (props) => (
    <th className="border border-border px-2 py-1 text-left font-semibold" {...props} />
  ),
  td: (props) => <td className="border border-border px-2 py-1" {...props} />,
  pre: PreRenderer,
  code: InlineCode,
}

function MessageBubbleImpl({
  role,
  content,
  isStreaming = false,
  sources = EMPTY_SOURCES,
  onOpenCitation,
}: MessageBubbleProps) {
  const { copied, copy } = useCopyToClipboard()

  const components = useMemo<Components>(
    () => ({
      ...baseMarkdownComponents,
      ...(role === 'user' ? userMarkdownComponents : {}),
      [CITATION_TAG_NAME]: (props: { citationIndex?: number }) => (
        <CitationChip
          citationIndex={props.citationIndex ?? 0}
          sources={sources}
          onOpenCitation={onOpenCitation}
        />
      ),
    }),
    [role, sources, onOpenCitation],
  )

  return (
    <div
      data-slot="message-bubble"
      data-role={role}
      className={cn(messageBubbleVariants({ role }))}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm, remarkCitations]} components={components}>
        {content}
      </ReactMarkdown>
      {role === 'assistant' && !isStreaming && (
        <button
          type="button"
          onClick={() => void copy(stripCitationMarkers(content))}
          aria-label={copied ? 'Copied' : 'Copy message'}
          className="mt-1.5 flex cursor-pointer items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
        >
          <CopyIcon copied={copied} />
        </button>
      )}
    </div>
  )
}

// Memoized: a finished message must not re-render while another one streams.
const MessageBubble = memo(MessageBubbleImpl)

export { MessageBubble }
