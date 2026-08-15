import { AuthLayout } from '@/components/auth-layout'
import { OpenRouterKeyForm } from '@/components/openrouter-key-form'

interface KeyGateScreenProps {
  onSaved: () => void
}

function KeyGateScreen({ onSaved }: KeyGateScreenProps) {
  return (
    <AuthLayout title="Conecte sua chave OpenRouter">
      <p className="text-sm text-muted-foreground">
        Para personalizar e conduzir suas futuras sessões de estudo, o Athena precisa de uma chave
        da OpenRouter.
      </p>
      <OpenRouterKeyForm onSaved={onSaved} />
    </AuthLayout>
  )
}

export default KeyGateScreen
