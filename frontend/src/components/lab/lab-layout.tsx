import { useEffect, useRef, useState, type ReactNode } from 'react'
import { PanelLeft, PanelRight } from 'lucide-react'
import type { PanelImperativeHandle } from 'react-resizable-panels'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable'
import { DIFFICULTY_LABELS, type ChallengeDifficulty } from '@/lib/exercise'
import type { LabCriterion, LabHint, LabStatement } from '@/lib/lab'
import { LabCountdown } from './lab-countdown'
import { LabLeaveDialog } from './lab-leave-dialog'
import { LabRubricPanel } from './lab-rubric-panel'
import { LabStatementPanel } from './lab-statement-panel'
import { LabTutorPanel } from './lab-tutor-panel'

// Below this window width the side panels start collapsed, so the board keeps
// the screen. There is no mobile layout.
const MIN_WIDTH_FOR_SIDE_PANELS = 1100

interface LabLayoutProps {
  title: string
  topic: string
  difficulty: string
  statement: LabStatement
  criteria: LabCriterion[]
  // ISO timestamp of the attempt's deadline, owned by the backend.
  deadline: string
  now?: () => number
  hints: LabHint[]
  hintsLeft: number
  hintAsking: boolean
  hintFailed: boolean
  // True once the board has content.
  canFinish: boolean
  // The board: the diagram editor, supplied by the caller.
  board: ReactNode
  // Set once the diagram was submitted: the stage of the exam that follows (the
  // evaluation, the tutor's questions). It takes the place of the tutor's
  // hints, and the clock and Finish are gone: the drawing is over.
  stage?: ReactNode
  onExpire: () => void
  onAskHint: () => void
  onFinish: () => void
  onLeave: () => void
}

// Stryker disable next-line StringLiteral: hover styling, no layout engine in jsdom, checked by eye in the dev preview.
const HANDLE_CLASSES = 'transition-colors hover:bg-primary/60 active:bg-primary'

// The exam screen of the Architecture Lab: statement on the left, the board in
// the center, the tutor and the rubric on the right, and a top bar with the
// countdown and the actions. It fetches nothing and starts, submits or
// discards nothing: everything comes in as props and every action is a
// handler. See specs/phases/phase-03-architecture-lab/08-lab-layout-sketch.md.
function LabLayout({
  title,
  topic,
  difficulty,
  statement,
  criteria,
  deadline,
  now,
  hints,
  hintsLeft,
  hintAsking,
  hintFailed,
  canFinish,
  board,
  stage,
  onExpire,
  onAskHint,
  onFinish,
  onLeave,
}: LabLayoutProps) {
  const statementRef = useRef<PanelImperativeHandle>(null)
  const sidebarRef = useRef<PanelImperativeHandle>(null)
  const [wideWindow] = useState(() => window.innerWidth >= MIN_WIDTH_FOR_SIDE_PANELS)
  const [statementOpen, setStatementOpen] = useState(wideWindow)
  const [sidebarOpen, setSidebarOpen] = useState(wideWindow)
  const [leaving, setLeaving] = useState(false)

  // Stryker disable BlockStatement,ConditionalExpression,OptionalChaining,ArrayDeclaration,CallExpression: the
  // panel library cannot compute a real collapsed/expanded size in jsdom
  // (setup.ts zeroes every panel's rect), so the imperative collapse/expand
  // calls are verified by hand in a real window; the open/closed state they
  // accompany is what the tests assert, through the toggle labels.
  useEffect(() => {
    if (wideWindow) return
    statementRef.current?.collapse()
    sidebarRef.current?.collapse()
  }, [wideWindow])

  function toggle(panel: PanelImperativeHandle | null, open: boolean) {
    if (open) panel?.collapse()
    else panel?.expand()
  }

  function handleToggleStatement() {
    toggle(statementRef.current, statementOpen)
    setStatementOpen(!statementOpen)
  }

  function handleToggleSidebar() {
    toggle(sidebarRef.current, sidebarOpen)
    setSidebarOpen(!sidebarOpen)
  }
  // Stryker restore BlockStatement,ConditionalExpression,OptionalChaining,ArrayDeclaration,CallExpression

  const difficultyLabel = DIFFICULTY_LABELS[difficulty as ChallengeDifficulty] ?? difficulty

  return (
    <div className="flex h-full min-h-0 w-full flex-col gap-3">
      <header className="flex flex-col gap-2">
        <nav aria-label="Breadcrumb" className="text-xs text-muted-foreground">
          <ol className="flex items-center gap-1.5">
            <li>Architecture Lab</li>
            <li aria-hidden="true">›</li>
            <li>{topic}</li>
            <li aria-hidden="true">›</li>
            <li>{title}</li>
          </ol>
        </nav>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={statementOpen ? 'Hide statement' : 'Show statement'}
              onClick={handleToggleStatement}
            >
              <PanelLeft className="size-4" aria-hidden="true" />
            </Button>
            <h1 className="truncate text-lg font-semibold text-foreground">{title}</h1>
            <Badge variant="muted">{difficultyLabel}</Badge>
            <Badge variant="muted">{topic}</Badge>
          </div>
          <div className="flex items-center gap-3">
            {!stage && <LabCountdown deadline={deadline} now={now} onExpire={onExpire} />}
            <Button type="button" variant="outline" size="sm" onClick={() => setLeaving(true)}>
              Leave
            </Button>
            {!stage && (
              <Button type="button" size="sm" disabled={!canFinish} onClick={onFinish}>
                Finish and evaluate
              </Button>
            )}
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={sidebarOpen ? 'Hide tutor and rubric' : 'Show tutor and rubric'}
              onClick={handleToggleSidebar}
            >
              <PanelRight className="size-4" aria-hidden="true" />
            </Button>
          </div>
        </div>
      </header>

      {/* The panels are collapsible, never unmounted, so the tutor's hints
          survive being hidden. The board never collapses. */}
      <ResizablePanelGroup
        orientation="horizontal"
        className="min-h-0 flex-1 rounded-lg border border-border"
        // Stryker disable next-line ObjectLiteral,StringLiteral: layout style, no layout engine in jsdom, checked by eye in the dev preview.
        style={{ height: 'auto' }}
      >
        <ResizablePanel
          panelRef={statementRef}
          collapsible
          collapsedSize={0}
          defaultSize={320}
          minSize={240}
          maxSize={480}
          groupResizeBehavior="preserve-pixel-size"
          // Stryker disable next-line all: drag-resize cannot run in jsdom (see above).
          onResize={(size) => setStatementOpen(size.inPixels > 0)}
          // Stryker disable next-line ObjectLiteral,StringLiteral: layout style, no layout engine in jsdom, checked by eye in the dev preview.
          style={{ overflow: 'hidden' }}
        >
          <LabStatementPanel statement={statement} />
        </ResizablePanel>
        <ResizableHandle className={HANDLE_CLASSES} />
        <ResizablePanel
          minSize={400}
          // Stryker disable next-line ObjectLiteral,StringLiteral: layout style, no layout engine in jsdom, checked by eye in the dev preview.
          style={{ overflow: 'hidden' }}
        >
          <section aria-label="Board" className="relative h-full bg-muted/30">
            {board}
          </section>
        </ResizablePanel>
        <ResizableHandle className={HANDLE_CLASSES} />
        <ResizablePanel
          panelRef={sidebarRef}
          collapsible
          collapsedSize={0}
          defaultSize={340}
          minSize={280}
          maxSize={520}
          groupResizeBehavior="preserve-pixel-size"
          // Stryker disable next-line all: drag-resize cannot run in jsdom (see above).
          onResize={(size) => setSidebarOpen(size.inPixels > 0)}
          // Stryker disable next-line ObjectLiteral,StringLiteral: layout style, no layout engine in jsdom, checked by eye in the dev preview.
          style={{ overflow: 'hidden' }}
        >
          <div className="flex h-full flex-col divide-y divide-border overflow-y-auto">
            {stage ?? (
              <LabTutorPanel
                hints={hints}
                hintsLeft={hintsLeft}
                asking={hintAsking}
                failed={hintFailed}
                onAskHint={onAskHint}
              />
            )}
            <LabRubricPanel criteria={criteria} />
          </div>
        </ResizablePanel>
      </ResizablePanelGroup>

      <LabLeaveDialog
        open={leaving}
        onCancel={() => setLeaving(false)}
        onConfirm={() => {
          setLeaving(false)
          onLeave()
        }}
      />
    </div>
  )
}

export { LabLayout }
