import { useState } from 'react'
import { ResetLocalAccount } from '../../wailsjs/go/desktop/App'
import { authErrorMessage } from '../lib/authErrors'

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
    <div className="auth-screen">
      <h1>Resetar conta local</h1>
      <p>
        Esta ação apaga a conta local e todos os dados associados a ela, para que você possa criar
        uma nova conta com o mesmo e-mail. Isso não é uma recuperação de senha de verdade — como não
        há servidor nem e-mail, não é possível recuperar a senha atual.
      </p>

      <form onSubmit={handleSubmit}>
        <label htmlFor="reset-email">E-mail</label>
        <input
          id="reset-email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />

        {error && <p className="auth-error">{error}</p>}

        <button type="submit">Excluir conta local</button>
      </form>

      <button type="button" onClick={onCancel}>
        Cancelar
      </button>
    </div>
  )
}

export default ResetAccountScreen
