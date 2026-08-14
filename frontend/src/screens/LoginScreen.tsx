import { useState } from 'react'
import { Login } from '../../wailsjs/go/desktop/App'
import type { desktop } from '../../wailsjs/go/models'
import { authErrorMessage } from '../lib/authErrors'
import { AuthLayout } from '@/components/auth-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface LoginScreenProps {
  onSuccess: (result: desktop.LoginResult) => void
  onNavigateToRegister: () => void
  onNavigateToReset: () => void
}

function LoginScreen({ onSuccess, onNavigateToRegister, onNavigateToReset }: LoginScreenProps) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError('')
    try {
      const result = await Login(email, password)
      onSuccess(result)
    } catch (err) {
      setError(authErrorMessage(err))
    }
  }

  return (
    <AuthLayout title="Entrar">
      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="login-email">E-mail</Label>
          <Input
            id="login-email"
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="login-password">Senha</Label>
          <Input
            id="login-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </div>

        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <Button type="submit">Entrar</Button>
      </form>

      <div className="flex justify-center gap-2">
        <Button type="button" variant="ghost" size="sm" onClick={onNavigateToRegister}>
          Criar conta
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onNavigateToReset}>
          Esqueci minha senha
        </Button>
      </div>
    </AuthLayout>
  )
}

export default LoginScreen
