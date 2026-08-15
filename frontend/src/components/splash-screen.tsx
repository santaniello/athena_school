import { AthenaLogo } from '@/components/athena-logo'
import { GreekKeyDivider } from '@/components/greek-key-divider'

// Boot animation shown while App resolves the post-auth view: the Greek key
// motif slides in from all four screen edges to frame it, then the Athena
// monogram fades in at the center and settles into a slow glowing pulse.
// Purely presentational — App swaps the view (and unmounts this) as soon as
// the session check resolves, so there's no completion callback to wire up.
function SplashScreen() {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-background">
      <GreekKeyDivider className="animate-in slide-in-from-top fill-mode-both absolute inset-x-0 top-0 duration-700 ease-out" />
      <GreekKeyDivider className="animate-in slide-in-from-bottom fill-mode-both absolute inset-x-0 bottom-0 duration-700 ease-out" />
      <GreekKeyDivider
        orientation="vertical"
        className="animate-in slide-in-from-left fill-mode-both absolute inset-y-0 left-0 delay-100 duration-700 ease-out"
      />
      <GreekKeyDivider
        orientation="vertical"
        className="animate-in slide-in-from-right fill-mode-both absolute inset-y-0 right-0 delay-100 duration-700 ease-out"
      />
      <AthenaLogo className="animate-athena-logo relative h-36 w-36" />
    </div>
  )
}

export { SplashScreen }
