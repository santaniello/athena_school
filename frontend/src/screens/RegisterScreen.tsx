import { useState } from 'react'
import { Login, Register } from '../../wailsjs/go/desktop/App'
import type { desktop } from '../../wailsjs/go/models'
import { authErrorMessage } from '../lib/authErrors'
import { AuthLayout } from '@/components/auth-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface RegisterScreenProps {
  onSuccess: (result: desktop.LoginResult) => void
  onNavigateToLogin: () => void
}

function RegisterScreen({ onSuccess, onNavigateToLogin }: RegisterScreenProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError("Passwords don't match.")
      return
    }

    try {
      await Register(email, password)
      const result = await Login(email, password)
      onSuccess(result)
    } catch (err) {
      setError(authErrorMessage(err))
    }
  }

  return (
    <AuthLayout title="Create account">
      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="register-email">Email</Label>
          <Input
            id="register-email"
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="register-password">Password</Label>
          <Input
            id="register-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="register-confirm-password">Confirm password</Label>
          <Input
            id="register-confirm-password"
            type="password"
            value={confirmPassword}
            onChange={(event) => setConfirmPassword(event.target.value)}
            required
          />
        </div>

        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <Button type="submit">Create account</Button>
      </form>

      <div className="flex justify-center">
        <Button type="button" variant="ghost" size="sm" onClick={onNavigateToLogin}>
          I already have an account
        </Button>
      </div>
    </AuthLayout>
  )
}

export default RegisterScreen
