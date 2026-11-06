import { useEffect, useRef, useState } from 'react'
import { Timer } from 'lucide-react'
import { countdownTone, formatCountdown, type CountdownTone } from '@/lib/lab'
import { cn } from '@/lib/utils'

interface LabCountdownProps {
  // ISO timestamp of the attempt's deadline. The backend owns the clock.
  deadline: string
  // Injectable clock, so tests do not depend on the wall time.
  now?: () => number
  onExpire: () => void
}

function toneClasses(tone: CountdownTone): string {
  const classes: Record<CountdownTone, string> = {
    normal: 'text-foreground',
    warning: 'text-amber-600 dark:text-amber-400',
    urgent: 'font-bold text-destructive',
  }
  return classes[tone]
}

// The time left in the exam. It is a countdown with no pause: the clock never
// stops. Pure presentation: ending the exam is the caller's decision.
function LabCountdown({ deadline, now = Date.now, onExpire }: LabCountdownProps) {
  const deadlineMs = new Date(deadline).getTime()
  const [remainingMs, setRemainingMs] = useState(() => deadlineMs - now())
  const expiredRef = useRef(false)
  const onExpireRef = useRef(onExpire)

  useEffect(() => {
    onExpireRef.current = onExpire
  }, [onExpire])

  useEffect(() => {
    const tick = () => setRemainingMs(deadlineMs - now())
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [deadlineMs, now])

  useEffect(() => {
    if (remainingMs > 0 || expiredRef.current) return
    expiredRef.current = true
    onExpireRef.current()
  }, [remainingMs])

  const tone = countdownTone(remainingMs)
  return (
    <span
      role="timer"
      data-tone={tone}
      // Stryker disable next-line StringLiteral: pure layout classes of the clock.
      className={cn('inline-flex items-center gap-1.5 text-sm tabular-nums', toneClasses(tone))}
    >
      <Timer className="size-4" aria-hidden="true" />
      {formatCountdown(remainingMs)}
    </span>
  )
}

export { LabCountdown }
