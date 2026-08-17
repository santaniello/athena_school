import type { ComponentProps } from 'react'
import ReactMarkdown, { type Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { PrismLight as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import bash from 'react-syntax-highlighter/dist/esm/languages/prism/bash'
import css from 'react-syntax-highlighter/dist/esm/languages/prism/css'
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go'
import javascript from 'react-syntax-highlighter/dist/esm/languages/prism/javascript'
import json from 'react-syntax-highlighter/dist/esm/languages/prism/json'
import jsx from 'react-syntax-highlighter/dist/esm/languages/prism/jsx'
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
}

const messageBubbleVariants = cva(
  'max-w-[75%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed break-words',
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

function CodeRenderer({ className, children, ...props }: ComponentProps<'code'>) {
  const match = className ? /language-(\w+)/.exec(className) : null

  if (!match) {
    return (
      <code
        data-slot="inline-code"
        className="rounded bg-muted px-1 py-0.5 font-mono text-[0.8em] text-primary"
        {...props}
      >
        {children}
      </code>
    )
  }

  return (
    <div data-slot="code-block" className="my-2 overflow-hidden rounded-md border border-border">
      <SyntaxHighlighter
        language={match[1]}
        PreTag="div"
        style={oneDark}
        customStyle={{
          background: 'var(--color-muted)',
          margin: 0,
          padding: '0.75rem 1rem',
          fontSize: '0.8125rem',
        }}
        codeTagProps={{
          style: {
            fontFamily: 'var(--font-mono, ui-monospace, monospace)',
            color: 'var(--color-foreground)',
          },
        }}
      >
        {String(children).replace(/\n$/, '')}
      </SyntaxHighlighter>
    </div>
  )
}

const markdownComponents: Components = {
  p: (props) => <p className="mb-2 last:mb-0" {...props} />,
  h1: (props) => (
    <h2 className="mb-2 font-heading text-base font-semibold text-foreground" {...props} />
  ),
  h2: (props) => (
    <h3 className="mb-2 font-heading text-sm font-semibold text-foreground" {...props} />
  ),
  h3: (props) => (
    <h4 className="mb-1 font-heading text-sm font-semibold text-foreground" {...props} />
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
  table: (props) => <table className="mb-2 w-full border-collapse text-xs last:mb-0" {...props} />,
  th: (props) => (
    <th className="border border-border px-2 py-1 text-left font-semibold" {...props} />
  ),
  td: (props) => <td className="border border-border px-2 py-1" {...props} />,
  code: CodeRenderer,
}

function MessageBubble({ role, content }: MessageBubbleProps) {
  return (
    <div
      data-slot="message-bubble"
      data-role={role}
      className={cn(messageBubbleVariants({ role }))}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
        {content}
      </ReactMarkdown>
    </div>
  )
}

export { MessageBubble }
