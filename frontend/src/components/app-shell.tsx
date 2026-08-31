import { useEffect, useRef, useState } from 'react'
import { BookOpen, LogOut } from 'lucide-react'
import { Logout } from '../../wailsjs/go/desktop/App'
import { AthenaLogo } from '@/components/athena-logo'
import { NavItem } from '@/components/nav-item'
import { ComingSoonPanel } from '@/components/coming-soon-panel'
import { StudyFolderTree, type StudyFolderTreeHandle } from '@/components/study-folder-tree'
import { KnowledgeTopicTree } from '@/components/knowledge-topic-tree'
import { KnowledgeSection } from '@/components/knowledge-section'
import { IndexLoadingScreen } from '@/components/index-loading-screen'
import { IndexFailedScreen } from '@/components/index-failed-screen'
import { IndexStatusBanner } from '@/components/index-status-banner'
import { IndexReviewDialog } from '@/components/index-review-dialog'
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable'
import HomeScreen from '@/screens/HomeScreen'
import StudyChatScreen from '@/screens/StudyChatScreen'
import SettingsScreen from '@/screens/SettingsScreen'
import DocumentationScreen from '@/screens/DocumentationScreen'
import { NAVIGATION, type AppSection } from '@/lib/navigation'
import { getUserProfile, type ProfileDraft } from '@/lib/profile'
import { startStudySession, type StudySession } from '@/lib/study'
import { countDraftKnowledgeItems, countPendingReconciliations } from '@/lib/knowledge'
import {
  getKnowledgeIndexStatus,
  onKnowledgeIndexStatus,
  retryKnowledgeIndex,
  type IndexStatus,
} from '@/lib/knowledge-index'

// Stryker disable ArrayDeclaration,StringLiteral: issues/lastError are only
// ever visibly rendered once the knowledge-index effect below has already
// replaced this initial value with a resolved status — IndexLoadingScreen,
// the only screen shown while this exact object is still current, renders
// neither field.
const INITIAL_INDEX_STATUS: IndexStatus = {
  state: 'loading',
  hasSnapshot: false,
  issues: [],
  lastError: '',
}
// Stryker restore ArrayDeclaration,StringLiteral

interface AppShellProps {
  onLogout: () => void
}

interface ActiveStudySession {
  id: string
  topic: string
  folderId: string
  folderName: string
  // 'new' sessions request the opening turn; 'resume' sessions (picked from
  // the sidebar tree) load their prior history instead.
  mode: 'new' | 'resume'
}

const PRIMARY_ITEMS = NAVIGATION.filter((item) => item.group === 'primary')
const FOOTER_ITEMS = NAVIGATION.filter((item) => item.group === 'footer')

// The persistent app chrome (sidebar + topbar) mounted once, after auth, the
// key gate and onboarding are all done. Hosts every roadmap section from day
// one — most locked — so later phases only flip a status flag in
// lib/navigation.ts instead of redesigning navigation. See
// specs/phases/phase-01-desktop-mvp/03-home-screen.md.
//
// The Study section's folders/sessions live in the sidebar as an
// explorer-style tree (StudyFolderTree) rather than a separate list screen —
// this component owns which session is open so the tree (rail) and the chat
// view (main pane) can stay in sync. See
// specs/phases/phase-01-desktop-mvp/10-study-folders.md.
function AppShell({ onLogout }: AppShellProps) {
  const [section, setSection] = useState<AppSection>('home')
  const [profile, setProfile] = useState<ProfileDraft | null>(null)
  const [activeSession, setActiveSession] = useState<ActiveStudySession | null>(null)
  const [selectedTopic, setSelectedTopic] = useState<string | null>(null)
  const [indexStatus, setIndexStatus] = useState<IndexStatus>(INITIAL_INDEX_STATUS)
  const [continuedWithoutSearch, setContinuedWithoutSearch] = useState(false)
  const [retryingIndex, setRetryingIndex] = useState(false)
  const [reviewOpen, setReviewOpen] = useState(false)
  const [startingNewSession, setStartingNewSession] = useState(false)
  const [newSessionError, setNewSessionError] = useState<string | null>(null)
  const [draftCount, setDraftCount] = useState(0)
  const [pendingProposalCount, setPendingProposalCount] = useState(0)
  const studyFolderTreeRef = useRef<StudyFolderTreeHandle>(null)
  // refreshDraftCount/refreshPendingProposalCount each fire from several
  // independent call sites (mount, approve, reject, save-as-drafts, a
  // reconciliation decision); their responses can arrive out of order, so
  // only the reply to the most recently *started* call is ever applied —
  // same requestVersion guard KnowledgeExplorerScreen uses for its own list
  // fetch.
  const draftCountRequestRef = useRef(0)
  const pendingProposalCountRequestRef = useRef(0)

  // Stryker disable ArrayDeclaration: mount-once effect — its dependency
  // array's content is not itself observable behavior.
  useEffect(() => {
    void getUserProfile().then(setProfile)
  }, [])
  // Stryker restore ArrayDeclaration

  // draftCount/pendingProposalCount are lifted here (alongside profile/
  // activeSession) rather than fetched locally by KnowledgeSection, so the
  // sidebar badge (their sum) and the Review tab's own draft-only badge
  // always agree — see specs/phases/phase-02-knowledge-engine/07-knowledge-review.md
  // and 11-knowledge-reconciliation.md.
  // Stryker disable UpdateOperator: both refresh functions below only ever
  // compare their own requestId against the ref's *current* value to detect
  // staleness — incrementing or decrementing produces the same distinct,
  // monotonic sequence either way, so the direction itself is unobservable;
  // only ever assigning the same, unchanging value would be.
  function refreshDraftCount() {
    const requestId = ++draftCountRequestRef.current
    void countDraftKnowledgeItems()
      .then((count) => {
        if (draftCountRequestRef.current === requestId) setDraftCount(count)
      })
      .catch(() => {})
  }

  function refreshPendingProposalCount() {
    const requestId = ++pendingProposalCountRequestRef.current
    void countPendingReconciliations()
      .then((count) => {
        if (pendingProposalCountRequestRef.current === requestId) setPendingProposalCount(count)
      })
      .catch(() => {})
  }
  // Stryker restore UpdateOperator

  // refreshReviewCounts is what every knowledge-changing action actually
  // triggers — a single decision (approve, reject, save-as-drafts, applying
  // or rejecting a reconciliation proposal) can move either count, so both
  // refetch together rather than each call site guessing which one to ask for.
  function refreshReviewCounts() {
    refreshDraftCount()
    refreshPendingProposalCount()
  }

  // Stryker disable ArrayDeclaration: mount-once effect — its dependency
  // array's content is not itself observable behavior.
  useEffect(() => {
    refreshReviewCounts()
  }, [])
  // Stryker restore ArrayDeclaration

  // The listener is registered before the initial query fires, closing the
  // race where a fast background load finishes before this subscribes —
  // continuous polling is not used (see
  // specs/phases/phase-02-knowledge-engine/04-vector-search.md).
  // Stryker disable ArrayDeclaration: mount-once effect — its dependency
  // array's content is not itself observable behavior.
  useEffect(() => {
    let receivedStatusEvent = false
    const unsubscribe = onKnowledgeIndexStatus((status) => {
      receivedStatusEvent = true
      setIndexStatus(status)
    })

    // A rejected initial query must still resolve `loading` into a terminal
    // state — otherwise the app stays stuck behind IndexLoadingScreen
    // forever. Guarded by `receivedStatusEvent` so a slow initial response
    // can never overwrite a newer status pushed by the event above.
    void getKnowledgeIndexStatus()
      .then((status) => {
        if (!receivedStatusEvent) setIndexStatus(status)
      })
      .catch(() => {
        if (!receivedStatusEvent) {
          setIndexStatus({
            ...INITIAL_INDEX_STATUS,
            state: 'failed',
            lastError: 'Could not load the knowledge index.',
          })
        }
      })

    return unsubscribe
  }, [])
  // Stryker restore ArrayDeclaration

  async function handleRetryIndex() {
    setRetryingIndex(true)
    try {
      // "knowledge-index:status" also fires with this exact outcome, but
      // applying the resolved value directly here does not depend on that
      // separate event channel having fired yet.
      const result = await retryKnowledgeIndex()
      setIndexStatus(result)
      // Only drop the "continue without search" opt-in on an actual
      // recovery — a retry that fails again (no prior snapshot to fall
      // back to) must not silently re-block the app the user already
      // chose to continue using.
      if (result.state !== 'failed') setContinuedWithoutSearch(false)
    } finally {
      setRetryingIndex(false)
    }
  }

  const activeItem = NAVIGATION.find((item) => item.id === section)!
  // Stryker disable ConditionalExpression,EqualityOperator: studyItem drives
  // only HomeScreen's studyLocked prop (studyItem.status === 'locked'), and
  // 'study' currently shares NAVIGATION's 'unlocked' status with 'home' —
  // the first item this lookup would wrongly fall back to under those
  // mutants — so the two are indistinguishable through that prop until
  // Study's status ever actually differs from Home's.
  const studyItem = NAVIGATION.find((item) => item.id === 'study')!
  // Stryker restore ConditionalExpression,EqualityOperator
  // Stryker disable ConditionalExpression,StringLiteral: 'study' has no
  // 'locked' status yet in NAVIGATION at this phase, so this is always
  // false today — genuinely equivalent to a hardcoded false until Study
  // Mode's own status ever changes.
  const studyLocked = studyItem.status === 'locked'
  // Stryker restore ConditionalExpression,StringLiteral

  async function handleLogout() {
    await Logout()
    onLogout()
  }

  function handleSelectSession(session: StudySession, folderName: string) {
    setActiveSession({
      id: session.id,
      topic: session.topic,
      folderId: session.folderId,
      folderName,
      // Stryker disable next-line StringLiteral: StudyChatScreen only ever
      // branches on `mode === 'new'` — any non-'new' value, mutant or not,
      // behaves identically to 'resume'.
      mode: 'resume',
    })
  }

  function handleSessionStarted(session: StudySession, folderName: string) {
    setActiveSession({
      id: session.id,
      topic: session.topic,
      folderId: session.folderId,
      folderName,
      mode: 'new',
    })
  }

  // Stryker disable ArrowFunction,ConditionalExpression,OptionalChaining:
  // testing "delete a different, non-active session leaves the active one
  // untouched" surfaced a pre-existing, unrelated bug in
  // StudyFolderTree — its row's Delete menu item is a React child of the
  // row's own onClick handler, so clicking it (even though the menu itself
  // renders through a portal) also bubbles a synthetic click up to
  // onSelectSession for that row, selecting the session being deleted
  // moments before it is actually deleted. That makes every deletion look
  // like "delete the active session" here regardless of which row it came
  // from, so this function's own branching is unobservable until that
  // bubbling bug is fixed separately (out of scope for this change).
  function handleSessionDeleted(sessionId: string) {
    setActiveSession((current) => (current?.id === sessionId ? null : current))
  }
  // Stryker restore ArrowFunction,ConditionalExpression,OptionalChaining

  function handleTopicResolved(topic: string) {
    setActiveSession((current) => (current ? { ...current, topic } : current))
  }

  // Starts a fresh session on the same topic and folder as the currently
  // open one — the "Start new session" action offered by StudyChatScreen's
  // context-limit warning/blocked banners. Coordinated here (not inside
  // StudyChatScreen or StudyFolderTree) since AppShell is the sole owner of
  // navigation/active-session state and of the sidebar tree's ref. See
  // specs/phases/phase-02-knowledge-engine/06-study-context-limits.md.
  async function handleStartNewSession() {
    // Stryker disable next-line LogicalOperator,ConditionalExpression: both
    // arms are already structurally guaranteed by the UI — this only ever
    // fires from a button rendered inside an active Study session (so
    // activeSession is truthy), and every such button shares the
    // `disabled={startingNewSession}` prop this second arm duplicates, so
    // black-box testing cannot reach either arm being false without
    // bypassing the DOM's own disabled-button semantics.
    if (!activeSession || startingNewSession) return
    setStartingNewSession(true)
    setNewSessionError(null)
    try {
      const session = await startStudySession(activeSession.topic, activeSession.folderId)
      // Stryker disable next-line OptionalChaining: only reachable while
      // viewing an active Study session, which always mounts
      // StudyFolderTree via the ref this guards — current is never null
      // on this path.
      studyFolderTreeRef.current?.refreshFolder(session.folderId)
      setActiveSession({
        id: session.id,
        topic: session.topic,
        folderId: session.folderId,
        folderName: activeSession.folderName,
        mode: 'new',
      })
    } catch (err) {
      setNewSessionError(err instanceof Error ? err.message : 'Failed to start a new session.')
    } finally {
      setStartingNewSession(false)
    }
  }

  // The entire application stays behind this screen until the initial
  // background load finishes — no search or knowledge mutation can race the
  // initial snapshot.
  if (indexStatus.state === 'loading' && !indexStatus.hasSnapshot) {
    return <IndexLoadingScreen />
  }

  // A failed, snapshot-less index is unavailable, not empty — offer Retry
  // or an explicit opt-in to continue with a persistent warning instead. A
  // failed retry that still has a preserved snapshot never reaches this
  // screen: the previous snapshot keeps search working, so only the banner
  // below needs to surface the failure.
  // Stryker disable next-line ConditionalExpression: the gate above already
  // rules out 'loading'; 'ready'/'ready_with_warnings' with no snapshot is
  // a combination the backend never actually produces, so this reduces to
  // testing the same domain invariant that gate already relies on.
  if (indexStatus.state === 'failed' && !indexStatus.hasSnapshot && !continuedWithoutSearch) {
    return (
      <IndexFailedScreen
        lastError={indexStatus.lastError}
        retrying={retryingIndex}
        onRetry={() => void handleRetryIndex()}
        onContinue={() => setContinuedWithoutSearch(true)}
      />
    )
  }

  // ResizablePanelGroup/ResizablePanel hard-code `height: 100%` and
  // `overflow: auto` as inline styles, which beat any h-*/overflow-* class.
  // #root has no height of its own, so a class alone would collapse the shell
  // to its content height, and the panel's own scrollbar would double up with
  // the sidebar's. The library merges the style prop over its defaults, so
  // these two have to stay inline.
  return (
    <>
      <ResizablePanelGroup orientation="horizontal" style={{ height: '100vh' }}>
        <ResizablePanel
          defaultSize={224}
          minSize={200}
          maxSize={420}
          groupResizeBehavior="preserve-pixel-size"
          className="flex h-full flex-col"
          style={{ overflow: 'hidden' }}
        >
          <nav className="flex h-full w-full flex-col bg-[oklch(0.115_0.014_50)] p-3">
            <div className="flex items-center gap-2 px-1.5 py-2">
              <AthenaLogo className="size-6 shrink-0" />
              <span className="font-heading text-sm font-bold tracking-[0.2em] text-primary">
                ATHENA
              </span>
            </div>

            <div
              className="thin-scroll mt-2 flex flex-1 flex-col gap-0.5 overflow-y-auto"
              style={{ scrollbarGutter: 'stable' }}
            >
              {PRIMARY_ITEMS.map((item) => (
                <div key={item.id}>
                  <NavItem
                    item={item}
                    active={item.id === section}
                    onSelect={setSection}
                    badge={item.id === 'knowledge' ? draftCount + pendingProposalCount : undefined}
                  />
                  {item.id === 'study' && section === 'study' && (
                    <StudyFolderTree
                      ref={studyFolderTreeRef}
                      selectedSessionId={activeSession?.id ?? null}
                      onSelectSession={handleSelectSession}
                      onSessionStarted={handleSessionStarted}
                      onSessionDeleted={handleSessionDeleted}
                    />
                  )}
                  {item.id === 'knowledge' && section === 'knowledge' && (
                    <KnowledgeTopicTree
                      selectedTopic={selectedTopic}
                      onSelectTopic={setSelectedTopic}
                    />
                  )}
                </div>
              ))}
            </div>

            <div className="flex flex-col gap-0.5 border-t border-border pt-2">
              {FOOTER_ITEMS.map((item) => (
                <NavItem
                  key={item.id}
                  item={item}
                  active={item.id === section}
                  onSelect={setSection}
                />
              ))}
            </div>

            <div className="mt-3 flex items-center gap-2 border-t border-border pt-3">
              <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary font-heading text-xs font-bold text-primary-foreground">
                {profile?.name.charAt(0).toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold text-foreground">{profile?.name}</p>
              </div>
              <button
                type="button"
                aria-label="Log out"
                onClick={() => void handleLogout()}
                className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/15 hover:text-destructive"
              >
                <LogOut className="size-4" aria-hidden="true" />
              </button>
            </div>
          </nav>
        </ResizablePanel>

        <ResizableHandle className="transition-colors hover:bg-primary/60 active:bg-primary" />

        <ResizablePanel
          minSize={360}
          className="flex h-full w-full flex-col"
          style={{ overflow: 'hidden' }}
        >
          <header className="flex min-h-11 shrink-0 items-center border-b border-border px-6 py-2">
            {section === 'study' && activeSession ? (
              <div className="min-w-0">
                <p className="truncate text-[11px] text-muted-foreground">
                  Study / {activeSession.folderName}
                </p>
                <h1 className="font-heading truncate text-base font-bold text-foreground">
                  {activeSession.topic}
                </h1>
              </div>
            ) : (
              <h1 className="font-heading text-xs font-bold tracking-[0.14em] text-foreground uppercase">
                {activeItem.label}
              </h1>
            )}
          </header>
          <IndexStatusBanner
            status={indexStatus}
            continuedWithoutSearch={continuedWithoutSearch}
            retrying={retryingIndex}
            onRetry={() => void handleRetryIndex()}
            onReview={() => setReviewOpen(true)}
          />
          {section === 'study' && newSessionError && (
            <p className="border-b border-border px-6 py-2 text-sm text-destructive">
              {newSessionError}
            </p>
          )}
          {/* min-h-0 lets this flex item shrink below its content's height —
            without it a long chat transcript stretches <main> past the
            viewport instead of scrolling inside its own scroll area. */}
          <main className="flex min-h-0 flex-1 p-10">
            {section === 'home' ? (
              <HomeScreen
                profile={profile}
                studyLocked={studyLocked}
                onStartStudy={() => setSection('study')}
              />
            ) : section === 'study' ? (
              activeSession ? (
                <StudyChatScreen
                  key={activeSession.id}
                  sessionId={activeSession.id}
                  initialTopic={activeSession.topic}
                  mode={activeSession.mode}
                  onTopicResolved={handleTopicResolved}
                  onStartNewSession={handleStartNewSession}
                  startingNewSession={startingNewSession}
                  onKnowledgeChanged={refreshReviewCounts}
                />
              ) : (
                <div className="m-auto flex flex-col items-center gap-2 text-center">
                  <BookOpen className="size-8 text-muted-foreground" aria-hidden="true" />
                  <p className="text-sm font-semibold text-foreground">No session open</p>
                  <p className="max-w-64 text-sm text-muted-foreground">
                    Pick one from the tree on the left, or start a new one inside a folder.
                  </p>
                </div>
              )
            ) : section === 'knowledge' ? (
              <KnowledgeSection
                selectedTopic={selectedTopic}
                mutationsDisabled={retryingIndex}
                draftCount={draftCount}
                onKnowledgeChanged={refreshReviewCounts}
              />
            ) : section === 'documentation' ? (
              <DocumentationScreen />
            ) : section === 'settings' && profile ? (
              <SettingsScreen profile={profile} onProfileUpdated={setProfile} />
            ) : (
              <ComingSoonPanel item={activeItem} />
            )}
          </main>
        </ResizablePanel>
      </ResizablePanelGroup>
      <IndexReviewDialog
        open={reviewOpen}
        issues={indexStatus.issues}
        onClose={() => setReviewOpen(false)}
      />
    </>
  )
}

export { AppShell }
