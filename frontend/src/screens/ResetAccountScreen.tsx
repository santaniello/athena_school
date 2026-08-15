import { useState } from 'react'
import { ResetLocalAccount } from '../../wailsjs/go/desktop/App'
import { authErrorMessage } from '../lib/authErrors'
import { AuthLayout } from '@/components/auth-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface ResetAccountScreenProps {
  onDone: () => void
  onCancel: () => void
}

function ResetAccountScreen({ onDone, onCancel }: ResetAccountScreenProps) {
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError('')
    try {
      await ResetLocalAccount(email)
      onDone()
    } catch (err) {
      setError(authErrorMessage(err))
    }
  }

  return (
    <AuthLayout title="Reset local account">
      <p className="text-sm text-muted-foreground">
        This action deletes the local account and all data associated with it, so you can create a
        new account with the same email. This is not a real password recovery — since there is no
        server or email, the current password cannot be recovered.
      </p>

      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="reset-email">Email</Label>
          <Input
            id="reset-email"
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>

        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <Button type="submit" variant="destructive">
          Delete local account
        </Button>
      </form>

      <div className="flex justify-center">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </AuthLayout>
  )
}

export default ResetAccountScreen
