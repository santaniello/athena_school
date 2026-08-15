import { AuthLayout } from '@/components/auth-layout'
import { OpenRouterKeyForm } from '@/components/openrouter-key-form'

interface KeyGateScreenProps {
  onSaved: () => void
}

function KeyGateScreen({ onSaved }: KeyGateScreenProps) {
  return (
    <AuthLayout title="Connect your OpenRouter key">
      <p className="text-sm text-muted-foreground">
        To personalize and run your future study sessions, Athena needs an OpenRouter key.
      </p>
      <OpenRouterKeyForm onSaved={onSaved} />
    </AuthLayout>
  )
}

export default KeyGateScreen
