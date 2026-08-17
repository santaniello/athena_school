import { useEffect, useState } from 'react'
import { LogOut } from 'lucide-react'
import { Logout } from '../../wailsjs/go/desktop/App'
import { AthenaLogo } from '@/components/athena-logo'
import { NavItem } from '@/components/nav-item'
import { ComingSoonPanel } from '@/components/coming-soon-panel'
import HomeScreen from '@/screens/HomeScreen'
import StudyScreen from '@/screens/StudyScreen'
import SettingsScreen from '@/screens/SettingsScreen'
import { NAVIGATION, type AppSection } from '@/lib/navigation'
import { getUserProfile, type ProfileDraft } from '@/lib/profile'

interface AppShellProps {
  onLogout: () => void
}

const PRIMARY_ITEMS = NAVIGATION.filter((item) => item.group === 'primary')
const FOOTER_ITEMS = NAVIGATION.filter((item) => item.group === 'footer')

// The persistent app chrome (sidebar + topbar) mounted once, after auth, the
// key gate and onboarding are all done. Hosts every roadmap section from day
// one — most locked — so later phases only flip a status flag in
// lib/navigation.ts instead of redesigning navigation. See
// specs/phases/phase-01-desktop-mvp/03-home-screen.md.
function AppShell({ onLogout }: AppShellProps) {
  const [section, setSection] = useState<AppSection>('home')
  const [profile, setProfile] = useState<ProfileDraft | null>(null)

  useEffect(() => {
    void getUserProfile().then(setProfile)
  }, [])

  const activeItem = NAVIGATION.find((item) => item.id === section)!
  const studyItem = NAVIGATION.find((item) => item.id === 'study')!

  async function handleLogout() {
    await Logout()
    onLogout()
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

        <div className="mt-2 flex flex-1 flex-col gap-0.5">
          {PRIMARY_ITEMS.map((item) => (
            <NavItem key={item.id} item={item} active={item.id === section} onSelect={setSection} />
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
            <StudyScreen onEndSession={() => setSection('home')} />
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
