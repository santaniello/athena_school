import { useEffect, useRef } from 'react'
import { Info, TriangleAlert, X } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface ToastItem {
  id: string
  tone: 'warning' | 'info'
  message: string
  // Omitted: the toast stays until the user dismisses it.
  durationMs?: number
}

interface ToastProps {
  tone: ToastItem['tone']
  message: string
  durationMs?: number
  onDismiss: () => void
}

// A notice that dismisses itself after durationMs, or stays until the user
// closes it when durationMs is omitted. The countdown
// pauses while the pointer is over it or focus is inside it (so it can be
// read or acted on), and resumes with the time that was left. A warning is
// announced as an alert, a plain notice as a status.
function Toast({ tone, message, durationMs, onDismiss }: ToastProps) {
  const onDismissRef = useRef(onDismiss)
  const remainingRef = useRef(durationMs ?? 0)
  const startedAtRef = useRef(0)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const hoveredRef = useRef(false)
  const focusedRef = useRef(false)

  useEffect(() => {
    onDismissRef.current = onDismiss
  }, [onDismiss])

  function start() {
    if (durationMs === undefined) return
    startedAtRef.current = Date.now()
    timerRef.current = setTimeout(() => onDismissRef.current(), remainingRef.current)
  }

  function stop() {
    if (timerRef.current === null) return
    clearTimeout(timerRef.current)
    timerRef.current = null
    remainingRef.current -= Date.now() - startedAtRef.current
  }

  function pause() {
    stop()
  }

  function resumeIfIdle() {
    if (hoveredRef.current || focusedRef.current || timerRef.current !== null) return
    start()
  }

  useEffect(() => {
    start()
    return () => {
      if (timerRef.current !== null) clearTimeout(timerRef.current)
      timerRef.current = null
    }
    // The countdown starts once per mounted toast.
  }, [])

  const isWarning = tone === 'warning'
  const Icon = isWarning ? TriangleAlert : Info

  return (
    <div
      role={isWarning ? 'alert' : 'status'}
      data-slot="toast"
      onMouseEnter={() => {
        hoveredRef.current = true
        pause()
      }}
      onMouseLeave={() => {
        hoveredRef.current = false
        resumeIfIdle()
      }}
      onFocus={() => {
        focusedRef.current = true
        pause()
      }}
      onBlur={() => {
        focusedRef.current = false
        resumeIfIdle()
      }}
      className={cn(
        'pointer-events-auto flex w-[min(28rem,100%)] items-start gap-3 rounded-lg border p-3 text-sm shadow-lg',
        isWarning
          ? 'border-orange-500/50 bg-orange-50 text-orange-950 dark:bg-orange-950 dark:text-orange-50'
          : 'bg-card text-card-foreground',
      )}
    >
      <Icon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <p className="min-w-0 flex-1">{message}</p>
      <button
        type="button"
        aria-label="Dismiss notification"
        onClick={onDismiss}
        className="shrink-0 rounded-sm opacity-70 transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        <X className="size-4" aria-hidden="true" />
      </button>
    </div>
  )
}

// Fixed top-center column over the page; toasts stack with a gap, never on
// top of each other.
function ToastViewport({
  toasts,
  onDismiss,
}: {
  toasts: ToastItem[]
  onDismiss: (id: string) => void
}) {
  return (
    <section
      aria-label="Notifications"
      className="pointer-events-none fixed inset-x-0 top-4 z-50 flex flex-col items-center gap-2 px-4"
    >
      {toasts.map((toast) => (
        <Toast
          key={toast.id}
          tone={toast.tone}
          message={toast.message}
          durationMs={toast.durationMs}
          onDismiss={() => onDismiss(toast.id)}
        />
      ))}
    </section>
  )
}

export { Toast, ToastViewport }
