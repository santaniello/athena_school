import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MessageBubble } from './message-bubble'
import type { StudySource } from '@/lib/study'

// testSource fills in the fields a given case doesn't care about with
// harmless defaults, so each one only spells out what it's asserting on.
function testSource(overrides: Partial<StudySource>): StudySource {
  return {
    chunkId: 'chunk-1',
    itemId: 'item-1',
    sourceType: 'imported_doc',
    filePath: 'notes/a.md',
    heading: 'Channels',
    concept: '',
    score: 0.9,
    excerpt: 'Channels are typed conduits for communicating between goroutines.',
    ...overrides,
  }
}

describe('MessageBubble', () => {
  it('shows a copy button for a settled assistant message', () => {
    // Given a settled (non-streaming) assistant message
    render(<MessageBubble role="assistant" content="Hi there" />)

    // Then a copy button is offered, same as ChatGPT-style chat UIs
    expect(screen.getByRole('button', { name: 'Copy message' })).toBeInTheDocument()
  })

  it('does not show a copy button for a user message', () => {
    // Given a user message
    render(<MessageBubble role="user" content="What is CAP theorem?" />)

    // Then no copy button is offered
    expect(screen.queryByRole('button', { name: 'Copy message' })).not.toBeInTheDocument()
  })

  it('does not show a copy button while the message is still streaming in', () => {
    // Given an assistant message that is still being streamed
    render(<MessageBubble role="assistant" content="partial rep" isStreaming />)

    // Then no copy button is offered yet
    expect(screen.queryByRole('button', { name: 'Copy message' })).not.toBeInTheDocument()
  })

  it('copies the raw message content to the clipboard and confirms it', async () => {
    // Given a settled assistant message with Markdown content
    const user = userEvent.setup()
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    render(<MessageBubble role="assistant" content="**bold** answer" />)

    // When clicking the copy button
    await user.click(screen.getByRole('button', { name: 'Copy message' }))

    // Then the raw (unrendered) content is copied and the button confirms it
    expect(writeText).toHaveBeenCalledWith('**bold** answer')
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument()
  })

  it('reverts the confirmation back to the copy button after a moment', async () => {
    // Given a settled assistant message that was just copied
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const user = userEvent.setup({ delay: null, advanceTimers: vi.advanceTimersByTime })
    vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
    render(<MessageBubble role="assistant" content="answer" />)
    await user.click(screen.getByRole('button', { name: 'Copy message' }))
    await screen.findByRole('button', { name: 'Copied' })

    // When enough time passes
    vi.advanceTimersByTime(2000)

    // Then the button reverts to its normal copy state
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Copy message' })).toBeInTheDocument(),
    )
    vi.useRealTimers()
  })

  it('renders plain-text content for a user message', () => {
    // Given a plain-text user message
    render(<MessageBubble role="user" content="What is CAP theorem?" />)

    // Then the text is rendered
    expect(screen.getByText('What is CAP theorem?')).toBeInTheDocument()
  })

  it('styles a user message as a right-aligned gold bubble', () => {
    // Given a user message
    render(<MessageBubble role="user" content="Hello" />)

    // Then it carries the user-role bubble styling, including the shared
    // sizing/padding classes common to both roles
    const bubble = screen.getByText('Hello').closest('[data-slot="message-bubble"]')
    expect(bubble).toHaveAttribute('data-role', 'user')
    expect(bubble).toHaveClass('self-end')
    expect(bubble).toHaveClass('bg-primary')
    expect(bubble).toHaveClass('max-w-[75%]')
    expect(bubble).not.toHaveClass('self-start')
    expect(bubble).not.toHaveClass('bg-card')
  })

  it('styles an assistant message as a left-aligned neutral bubble', () => {
    // Given an assistant message
    render(<MessageBubble role="assistant" content="Hi there" />)

    // Then it carries the assistant-role bubble styling
    const bubble = screen.getByText('Hi there').closest('[data-slot="message-bubble"]')
    expect(bubble).toHaveAttribute('data-role', 'assistant')
    expect(bubble).toHaveClass('self-start')
    expect(bubble).toHaveClass('bg-card')
    expect(bubble).not.toHaveClass('self-end')
    expect(bubble).not.toHaveClass('bg-primary')
  })

  it('renders markdown headings, lists, bold text and links as real elements', () => {
    // Given content with common Markdown constructs
    const content = [
      '# Title',
      '',
      '- first item',
      '- second item',
      '',
      '**bold text** and a [link](https://example.com)',
    ].join('\n')
    render(<MessageBubble role="assistant" content={content} />)

    // Then Markdown was parsed into real elements, not left as raw syntax
    expect(screen.getByRole('heading', { name: 'Title' })).toBeInTheDocument()
    expect(screen.getByRole('list')).toBeInTheDocument()
    expect(screen.getByText('first item')).toBeInTheDocument()
    expect(screen.getByText('second item')).toBeInTheDocument()
    expect(screen.getByText('bold text')).toHaveProperty('tagName', 'STRONG')
    const link = screen.getByRole('link', { name: 'link' })
    expect(link).toHaveAttribute('href', 'https://example.com')
    expect(screen.queryByText(/^#/)).not.toBeInTheDocument()
    expect(screen.queryByText(/\*\*/)).not.toBeInTheDocument()
  })

  it('renders a fenced code block through the syntax highlighter, not as raw text', () => {
    // Given a fenced code block with a known language
    const content = ['```typescript', 'const x = 1', '```'].join('\n')
    const { container } = render(<MessageBubble role="assistant" content={content} />)

    // Then it renders via the code-block wrapper with the code text present
    const codeBlock = container.querySelector('[data-slot="code-block"]')
    expect(codeBlock).not.toBeNull()
    expect(codeBlock?.textContent).toContain('const x = 1')
    // And the raw fence markers are not shown as literal text
    expect(screen.queryByText('```typescript')).not.toBeInTheDocument()
    // And the "typescript" grammar actually tokenized the code (proves the
    // full language name was captured and a registered grammar was used,
    // not just that the raw text happens to be present)
    expect(codeBlock?.querySelectorAll('.token').length).toBeGreaterThan(0)
  })

  it('themes the code block chrome and inner code element with the Athena design tokens', () => {
    // Given a fenced code block
    const content = ['```typescript', 'const x = 1', '```'].join('\n')
    const { container } = render(<MessageBubble role="assistant" content={content} />)

    // Then the highlighter's wrapper and code element carry the exact
    // Athena CSS-variable overrides, not the syntax highlighter's own
    // built-in chrome
    const wrapper = container.querySelector('[data-slot="code-block"] > div') as HTMLElement
    expect(wrapper.style.background).toBe('var(--color-muted)')
    expect(wrapper.style.margin).toBe('0px')
    expect(wrapper.style.padding).toBe('0.75rem 1rem')
    expect(wrapper.style.fontSize).toBe('0.8125rem')
    const codeTag = wrapper.querySelector('code') as HTMLElement
    expect(codeTag.style.fontFamily).toBe('var(--font-mono, ui-monospace, monospace)')
    expect(codeTag.style.color).toBe('var(--color-foreground)')
  })

  it('strips exactly the trailing newline of a fenced code block, keeping internal line breaks', () => {
    // Given a multi-line fenced code block (Markdown appends a trailing
    // newline to the code text before the closing fence)
    const content = ['```typescript', 'const a = 1', 'const b = 2', '```'].join('\n')
    const { container } = render(<MessageBubble role="assistant" content={content} />)

    // Then only the trailing newline was removed — the internal line break
    // between the two statements is preserved
    const codeTag = container.querySelector('[data-slot="code-block"] code')
    expect(codeTag?.textContent).toBe('const a = 1\nconst b = 2')
  })

  it.each([
    ['bash', 'echo "hi"'],
    ['css', '.a { color: red; }'],
    ['go', 'package main'],
    ['javascript', 'const x = 1'],
    ['json', '{"a": 1}'],
    ['jsx', 'const x = <div />'],
    ['markup', '<div>hi</div>'],
    ['python', 'x = 1'],
    ['sql', 'SELECT * FROM t'],
    ['tsx', 'const x: number = 1'],
    ['yaml', 'key: value'],
  ])('registers the "%s" language grammar for real syntax highlighting', (language, snippet) => {
    // Given a fenced code block for each language the bubble registers
    const content = ['```' + language, snippet, '```'].join('\n')
    const { container } = render(<MessageBubble role="assistant" content={content} />)

    // Then Prism actually tokenized it (an unregistered language renders as
    // one plain, un-tokenized span instead)
    const codeBlock = container.querySelector('[data-slot="code-block"]')
    expect(codeBlock?.querySelectorAll('.token').length).toBeGreaterThan(0)
  })

  it('renders inline code as a plain code element, not through the syntax highlighter', () => {
    // Given inline code (no language fence)
    const { container } = render(
      <MessageBubble role="assistant" content="Use the `useState` hook." />,
    )

    // Then it renders as inline code, with no code-block wrapper anywhere
    const inlineCode = screen.getByText('useState')
    expect(inlineCode.tagName).toBe('CODE')
    expect(inlineCode).toHaveAttribute('data-slot', 'inline-code')
    expect(container.querySelector('[data-slot="code-block"]')).toBeNull()
  })

  it('renders nested headings, ordered lists, blockquotes, horizontal rules and GFM tables', () => {
    // Given content covering every remaining Markdown element the bubble supports
    const content = [
      '## Section',
      '### Subsection',
      '',
      '1. step one',
      '2. step two',
      '',
      '> a quoted aside',
      '',
      '---',
      '',
      '| Col A | Col B |',
      '| --- | --- |',
      '| a1 | b1 |',
    ].join('\n')
    render(<MessageBubble role="assistant" content={content} />)

    // Then each element was parsed into its real, styled counterpart
    expect(screen.getByRole('heading', { name: 'Section' })).toHaveProperty('tagName', 'H3')
    expect(screen.getByRole('heading', { name: 'Subsection' })).toHaveProperty('tagName', 'H4')
    expect(screen.getByRole('list').tagName).toBe('OL')
    expect(screen.getByText('step one')).toBeInTheDocument()
    expect(screen.getByText('a quoted aside').closest('blockquote')).toBeInTheDocument()
    expect(document.querySelector('hr')).not.toBeNull()
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Col A' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'a1' })).toBeInTheDocument()
  })

  describe('citations', () => {
    it('renders a citation chip for a valid index', () => {
      // Given a reply citing its one source
      const sources = [testSource({})]
      render(<MessageBubble role="assistant" content="Channels are typed [1]." sources={sources} />)

      // Then a chip renders for that index
      expect(screen.getByRole('button', { name: '1' })).toHaveAttribute(
        'data-slot',
        'citation-chip',
      )
    })

    it('renders plain text for an out-of-range index', () => {
      // Given a reply citing an index with no matching source
      render(<MessageBubble role="assistant" content="Channels are typed [1]." sources={[]} />)

      // Then no chip is rendered, and the marker stays as plain text
      expect(screen.queryByRole('button', { name: '1' })).not.toBeInTheDocument()
      expect(screen.getByText(/\[1\]/)).toBeInTheDocument()
    })

    it('renders plain text when no sources prop is given at all', () => {
      // Given a reply with a citation marker and no sources prop
      render(<MessageBubble role="assistant" content="Channels are typed [1]." />)

      // Then the marker stays as plain text
      expect(screen.queryByRole('button', { name: '1' })).not.toBeInTheDocument()
      expect(screen.getByText(/\[1\]/)).toBeInTheDocument()
    })

    it('shows the source title, heading, and excerpt start on hover', async () => {
      // Given a reply citing a source with a known title/heading/excerpt
      const user = userEvent.setup()
      const sources = [
        testSource({
          filePath: 'notes/channels.md',
          heading: 'Channels',
          excerpt: 'Channels are typed conduits.',
        }),
      ]
      render(<MessageBubble role="assistant" content="Channels are typed [1]." sources={sources} />)

      // When hovering the chip
      await user.hover(screen.getByRole('button', { name: '1' }))

      // Then the tooltip shows the title, heading, and excerpt start
      expect(await screen.findByText('notes/channels.md')).toBeInTheDocument()
      expect(screen.getByText('Channels')).toBeInTheDocument()
      expect(screen.getByText(/Channels are typed conduits\./)).toBeInTheDocument()
    })

    it('calls onOpenCitation with the resolved source when a chip is clicked', async () => {
      // Given a reply citing its one source
      const user = userEvent.setup()
      const source = testSource({})
      const onOpenCitation = vi.fn()
      render(
        <MessageBubble
          role="assistant"
          content="Channels are typed [1]."
          sources={[source]}
          onOpenCitation={onOpenCitation}
        />,
      )

      // When clicking the chip
      await user.click(screen.getByRole('button', { name: '1' }))

      // Then onOpenCitation is called with that exact source
      expect(onOpenCitation).toHaveBeenCalledWith(source)
    })

    it('re-resolves a chip against a new sources list on rerender', async () => {
      // Given a bubble citing its one source
      const user = userEvent.setup()
      const first = testSource({ filePath: 'notes/first.md', heading: 'First' })
      const { rerender } = render(
        <MessageBubble role="assistant" content="Channels are typed [1]." sources={[first]} />,
      )
      await user.hover(screen.getByRole('button', { name: '1' }))
      expect(await screen.findByText('notes/first.md')).toBeInTheDocument()

      // When rerendered with a different sources list (e.g. the same
      // streaming bubble instance, a later study:sources event)
      const second = testSource({ filePath: 'notes/second.md', heading: 'Second' })
      rerender(
        <MessageBubble role="assistant" content="Channels are typed [1]." sources={[second]} />,
      )
      await user.unhover(screen.getByRole('button', { name: '1' }))
      await user.hover(screen.getByRole('button', { name: '1' }))

      // Then the chip resolves against the new list, not a memoized stale one
      expect(await screen.findByText('notes/second.md')).toBeInTheDocument()
      expect(screen.queryByText('notes/first.md')).not.toBeInTheDocument()
    })

    it('resolves citation chips in a streaming bubble', () => {
      // Given a streaming assistant bubble that already has its sources
      const sources = [testSource({})]
      render(
        <MessageBubble
          role="assistant"
          content="Channels are typed [1]"
          isStreaming
          sources={sources}
        />,
      )

      // Then the chip still resolves before the stream settles
      expect(screen.getByRole('button', { name: '1' })).toHaveAttribute(
        'data-slot',
        'citation-chip',
      )
    })

    it('strips citation markers when copying', async () => {
      // Given a settled assistant message citing a source
      const user = userEvent.setup()
      const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined)
      const sources = [testSource({})]
      render(<MessageBubble role="assistant" content="Channels are typed [1]." sources={sources} />)

      // When clicking the copy button
      await user.click(screen.getByRole('button', { name: 'Copy message' }))

      // Then the copied text has no citation markers
      expect(writeText).toHaveBeenCalledWith('Channels are typed .')
    })

    it('leaves a marker inside a code span unconverted', () => {
      // Given a citation-shaped marker inside an inline code span
      const sources = [testSource({})]
      render(<MessageBubble role="assistant" content="Run `foo[1]bar`." sources={sources} />)

      // Then it stays plain text inside the code element, not a chip
      expect(screen.queryByRole('button', { name: '1' })).not.toBeInTheDocument()
      expect(screen.getByText('foo[1]bar')).toHaveProperty('tagName', 'CODE')
    })
  })
})
