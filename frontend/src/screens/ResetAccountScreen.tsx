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
    <AuthLayout title="Resetar conta local">
      <p className="text-sm text-muted-foreground">
        Esta ação apaga a conta local e todos os dados associados a ela, para que você possa criar
        uma nova conta com o mesmo e-mail. Isso não é uma recuperação de senha de verdade — como não
        há servidor nem e-mail, não é possível recuperar a senha atual.
      </p>

      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="reset-email">E-mail</Label>
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
          Excluir conta local
        </Button>
      </form>

      <div className="flex justify-center">
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          Cancelar
        </Button>
      </div>
    </AuthLayout>
  )
}

export default ResetAccountScreen
