import { LEAF_PATH } from '@/components/athena-logo'

// Staggered per leaf via inline animation-delay — Tailwind's delay-* utility
// sets transition-delay, not animation-delay, so it can't stagger a
// @keyframes animation.
const LEAF_DELAYS_MS = [0, 150, 300]

// Shown in the message list while waiting for the LLM's first response
// chunk (see StudyScreen's isStreaming && !streamingText gap) — three
// laurel leaves, the same shape as AthenaLogo's wreath, bouncing in
// sequence instead of a generic spinner or an external image/gif, per
// specs/phases/phase-01-desktop-mvp/06-study-mode.md.
function ThinkingIndicator() {
  return (
    <div
      role="status"
      aria-label="Athena is thinking"
      data-slot="thinking-indicator"
      className="flex max-w-[75%] items-center gap-1.5 self-start rounded-2xl rounded-bl-sm bg-card px-4 py-3 ring-1 ring-foreground/10"
    >
      {LEAF_DELAYS_MS.map((delay) => (
        <svg
          key={delay}
          viewBox="-8 -13 16 14"
          aria-hidden="true"
          data-slot="thinking-leaf"
          className="size-3 animate-leaf-bounce text-primary"
          style={{ animationDelay: `${delay}ms` }}
        >
          <path d={LEAF_PATH} fill="currentColor" />
        </svg>
      ))}
    </div>
  )
}

export { ThinkingIndicator }
