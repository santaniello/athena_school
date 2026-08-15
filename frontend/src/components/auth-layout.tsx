import type { ReactNode } from 'react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { GreekKeyDivider } from '@/components/greek-key-divider'
import { AthenaLogo } from '@/components/athena-logo'

interface AuthLayoutProps {
  title: string
  children: ReactNode
}

// Shared shell for the auth screens (login, register, reset): a centered
// card framed by a Greek key motif, with the screen title as a real <h1>
// so it stays an accessible heading regardless of the card's own markup.
function AuthLayout({ title, children }: AuthLayoutProps) {
  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <Card className="w-full max-w-sm">
        <CardHeader className="items-center gap-3 text-center">
          <GreekKeyDivider />
          <div className="flex items-center justify-center gap-2">
            <AthenaLogo className="h-9 w-9 shrink-0" />
            <h1 className="font-heading text-2xl font-bold tracking-[0.15em] text-primary uppercase">
              {title}
            </h1>
          </div>
          <GreekKeyDivider className="scale-y-[-1]" />
        </CardHeader>
        <CardContent className="flex flex-col gap-4">{children}</CardContent>
      </Card>
    </div>
  )
}

export { AuthLayout }
