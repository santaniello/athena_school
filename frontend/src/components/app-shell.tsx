import { useEffect, useRef, useState } from 'react'
import { BookOpen, LogOut } from 'lucide-react'
import { Logout } from '../../wailsjs/go/desktop/App'
import { AthenaLogo } from '@/components/athena-logo'
import { NavItem } from '@/components/nav-item'
import { ComingSoonPanel } from '@/components/coming-soon-panel'
import { StudyFolderTree, type StudyFolderTreeHandle } from '@/components/study-folder-tree'
import HomeScreen from '@/screens/HomeScreen'
import StudyChatScreen from '@/screens/StudyChatScreen'
import SettingsScreen from '@/screens/SettingsScreen'
import { NAVIGATION, type AppSection } from '@/lib/navigation'
import { getUserProfile, type ProfileDraft } from '@/lib/profile'
import type { StudySession } from '@/lib/study'

interface AppShellProps {
  onLogout: () => void
}

interface ActiveStudySession {
  id: string
  folderId: string
  topic: string
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
  const studyTreeRef = useRef<StudyFolderTreeHandle>(null)

  useEffect(() => {
    void getUserProfile().then(setProfile)
  }, [])

  const activeItem = NAVIGATION.find((item) => item.id === section)!
  const studyItem = NAVIGATION.find((item) => item.id === 'study')!

  async function handleLogout() {
    await Logout()
    onLogout()
  }

  function handleSelectSession(session: StudySession) {
    setActiveSession({
      id: session.id,
      folderId: session.folderId,
      topic: session.topic,
      mode: 'resume',
    })
  }

  function handleSessionStarted(session: StudySession) {
    setActiveSession({
      id: session.id,
      folderId: session.folderId,
      topic: session.topic,
      mode: 'new',
    })
  }

  function handleSessionEnded(_sessionId: string, folderId: string) {
    studyTreeRef.current?.refreshFolder(folderId)
    setActiveSession(null)
  }

  function handleSessionDeleted(sessionId: string) {
    setActiveSession((current) => (current?.id === sessionId ? null : current))
  }

  return (
    <div className="flex h-screen">
      <nav className="flex w-56 flex-col border-r border-border bg-card p-3">
        <div className="flex items-center gap-2 px-1.5 py-2">
          <AthenaLogo className="size-6 shrink-0" />
          <span className="font-heading text-sm font-bold tracking-[0.2em] text-foreground">
            ATHENA
          </span>
        </div>

        <div
          className="sidebar-scroll mt-2 flex flex-1 flex-col gap-0.5 overflow-y-auto"
          style={{ scrollbarGutter: 'stable' }}
        >
          {PRIMARY_ITEMS.map((item) => (
            <div key={item.id}>
              <NavItem item={item} active={item.id === section} onSelect={setSection} />
              {item.id === 'study' && section === 'study' && (
                <StudyFolderTree
                  ref={studyTreeRef}
                  selectedSessionId={activeSession?.id ?? null}
                  onSelectSession={handleSelectSession}
                  onSessionStarted={handleSessionStarted}
                  onSessionDeleted={handleSessionDeleted}
                />
              )}
            </div>
          ))}
        </div>

        <div className="flex flex-col gap-0.5 border-t border-border pt-2">
          {FOOTER_ITEMS.map((item) => (
            <NavItem key={item.id} item={item} active={item.id === section} onSelect={setSection} />
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

      <div className="flex flex-1 flex-col">
        <header className="flex h-11 shrink-0 items-center border-b border-border px-6">
          <h1 className="font-heading text-xs font-bold tracking-[0.14em] text-foreground uppercase">
            {activeItem.label}
          </h1>
        </header>
        <main className="flex flex-1 p-10">
          {section === 'home' ? (
            <HomeScreen
              profile={profile}
              studyLocked={studyItem.status === 'locked'}
              onStartStudy={() => setSection('study')}
            />
          ) : section === 'study' ? (
            activeSession ? (
              <StudyChatScreen
                key={activeSession.id}
                sessionId={activeSession.id}
                folderId={activeSession.folderId}
                initialTopic={activeSession.topic}
                mode={activeSession.mode}
                onBack={() => setActiveSession(null)}
                onSessionEnded={handleSessionEnded}
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
          ) : section === 'settings' && profile ? (
            <SettingsScreen profile={profile} onProfileUpdated={setProfile} />
          ) : (
            <ComingSoonPanel item={activeItem} />
          )}
        </main>
      </div>
    </div>
  )
}

export { AppShell }
