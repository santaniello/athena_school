import { useState } from 'react'
import { Login } from '../../wailsjs/go/desktop/App'
import type { desktop } from '../../wailsjs/go/models'
import { authErrorMessage } from '../lib/authErrors'

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
    <div className="auth-screen">
      <h1>Entrar</h1>
      <form onSubmit={handleSubmit}>
        <label htmlFor="login-email">E-mail</label>
        <input
          id="login-email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />

        <label htmlFor="login-password">Senha</label>
        <input
          id="login-password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />

        {error && <p className="auth-error">{error}</p>}

        <button type="submit">Entrar</button>
      </form>

      <button type="button" onClick={onNavigateToRegister}>
        Criar conta
      </button>
      <button type="button" onClick={onNavigateToReset}>
        Esqueci minha senha
      </button>
    </div>
  )
}

export default LoginScreen
