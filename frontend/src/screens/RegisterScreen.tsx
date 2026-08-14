import { useState } from 'react'
import { Login, Register } from '../../wailsjs/go/desktop/App'
import type { desktop } from '../../wailsjs/go/models'
import { authErrorMessage } from '../lib/authErrors'

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
      setError('As senhas não coincidem.')
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
    <div className="auth-screen">
      <h1>Criar conta</h1>
      <form onSubmit={handleSubmit}>
        <label htmlFor="register-email">E-mail</label>
        <input
          id="register-email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />

        <label htmlFor="register-password">Senha</label>
        <input
          id="register-password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />

        <label htmlFor="register-confirm-password">Confirmar senha</label>
        <input
          id="register-confirm-password"
          type="password"
          value={confirmPassword}
          onChange={(event) => setConfirmPassword(event.target.value)}
          required
        />

        {error && <p className="auth-error">{error}</p>}

        <button type="submit">Criar conta</button>
      </form>

      <button type="button" onClick={onNavigateToLogin}>
        Já tenho conta
      </button>
    </div>
  )
}

export default RegisterScreen
