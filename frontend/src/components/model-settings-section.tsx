import { useCallback, useEffect, useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { EmbeddingChangeDialog } from '@/components/embedding-change-dialog'
import { ModelCombobox } from '@/components/model-combobox'
import {
  getModelSettings,
  listSelectableModels,
  MODEL_FEATURES,
  modelSettingsErrorMessage,
  saveModelSettings,
  type ModelChoices,
  type ModelSettings,
  type SelectableModels,
} from '@/lib/modelSettings'

const CATALOG_ERROR =
  'Could not load the OpenRouter model catalog. Check your connection and try again.'

// The Settings screen's "Models" section: which model serves each AI feature.
// A choice is saved the moment it is made and applies to the next call; the
// embedding model sits apart, behind a warning and a blocking confirmation,
// because changing it invalidates the notes' index. See
// specs/phases/phase-02-knowledge-engine/31-llm-model-param.md.
function ModelSettingsSection() {
  const [settings, setSettings] = useState<ModelSettings | null>(null)
  const [settingsFailed, setSettingsFailed] = useState(false)
  const [catalog, setCatalog] = useState<SelectableModels | null>(null)
  const [catalogFailed, setCatalogFailed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  // The embedding value awaiting confirmation ('' = back to the default),
  // or null when no confirmation is open.
  const [pendingEmbedding, setPendingEmbedding] = useState<string | null>(null)
  const [reimportReminder, setReimportReminder] = useState(false)

  const loadSettings = useCallback(() => {
    void getModelSettings().then(setSettings, () => setSettingsFailed(true))
  }, [])

  const loadCatalog = useCallback(() => {
    void listSelectableModels().then(setCatalog, () => setCatalogFailed(true))
  }, [])

  function retrySettings() {
    setSettingsFailed(false)
    loadSettings()
  }

  function retryCatalog() {
    setCatalogFailed(false)
    loadCatalog()
  }

  // Stryker disable ArrayDeclaration: mount-once effect — its dependency
  // array's content is not itself observable behavior.
  useEffect(() => {
    loadSettings()
    loadCatalog()
  }, [loadSettings, loadCatalog])
  // Stryker restore ArrayDeclaration

  async function save(next: ModelChoices): Promise<boolean> {
    setSaving(true)
    setSaveError('')
    try {
      const result = await saveModelSettings(next)
      setSettings((previous) => (previous ? { ...previous, current: next } : previous))
      if (result.embeddingModelChanged) setReimportReminder(true)
      return true
    } catch (err) {
      setSaveError(modelSettingsErrorMessage(err))
      return false
    } finally {
      setSaving(false)
    }
  }

  if (settingsFailed) {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-heading text-lg font-semibold text-foreground">Models</h2>
        <Alert variant="destructive">
          <AlertDescription>Could not load the model settings.</AlertDescription>
        </Alert>
        <Button type="button" variant="outline" className="self-start" onClick={retrySettings}>
          Retry
        </Button>
      </div>
    )
  }

  if (!settings) {
    return (
      <div className="flex flex-col gap-4">
        <h2 className="font-heading text-lg font-semibold text-foreground">Models</h2>
        <p className="text-sm text-muted-foreground">Loading...</p>
      </div>
    )
  }

  const { current, defaults } = settings
  const selectorsDisabled = saving || catalog === null

  function chooseChatModel(key: (typeof MODEL_FEATURES)[number]['key'], modelId: string) {
    void save({ ...current, [key]: modelId === defaults[key] ? '' : modelId })
  }

  function askToChangeEmbedding(modelId: string) {
    setSaveError('')
    setPendingEmbedding(modelId === defaults.embedding ? '' : modelId)
  }

  async function confirmEmbeddingChange() {
    if (pendingEmbedding === null) return
    if (await save({ ...current, embedding: pendingEmbedding })) setPendingEmbedding(null)
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 className="font-heading text-lg font-semibold text-foreground">Models</h2>
        <p className="text-sm text-muted-foreground">
          Choose which model each AI feature uses. Changes are saved immediately and apply to the
          next call.
        </p>
      </div>

      {catalogFailed && (
        <Alert variant="destructive">
          <AlertDescription>{CATALOG_ERROR}</AlertDescription>
          <Button type="button" variant="outline" size="sm" className="mt-2" onClick={retryCatalog}>
            Retry
          </Button>
        </Alert>
      )}
      {saveError && pendingEmbedding === null && (
        <Alert variant="destructive">
          <AlertDescription>{saveError}</AlertDescription>
        </Alert>
      )}

      {MODEL_FEATURES.map((feature) => (
        <div key={feature.key} className="flex flex-col gap-1.5 text-left">
          <p className="text-sm font-medium text-foreground">{feature.label}</p>
          <div className="flex items-center gap-2">
            <div className="min-w-0 flex-1">
              <ModelCombobox
                label={`${feature.label} model`}
                value={current[feature.key] || defaults[feature.key]}
                defaultValue={defaults[feature.key]}
                options={catalog?.chat ?? []}
                disabled={selectorsDisabled}
                onSelect={(modelId) => chooseChatModel(feature.key, modelId)}
              />
            </div>
            <Button
              type="button"
              variant="outline"
              aria-label={`Reset ${feature.label.toLowerCase()} model to default`}
              disabled={saving || current[feature.key] === ''}
              onClick={() => chooseChatModel(feature.key, defaults[feature.key])}
            >
              Reset to default
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">Default: {defaults[feature.key]}</p>
        </div>
      ))}

      <div className="flex flex-col gap-3 pt-2">
        <h3 className="font-heading text-base font-semibold text-muted-foreground">Advanced</h3>
        <div className="flex flex-col gap-3 rounded-lg border border-amber-500/40 bg-amber-500/10 p-4">
          <p className="flex items-start gap-2 text-sm text-amber-600 dark:text-amber-400">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            Changing the embedding model requires re-importing all of your notes.
          </p>
          <div className="flex flex-col gap-1.5 text-left">
            <p className="text-sm font-medium text-foreground">Embedding model</p>
            <div className="flex items-center gap-2">
              <div className="min-w-0 flex-1">
                <ModelCombobox
                  label="Embedding model"
                  value={current.embedding || defaults.embedding}
                  defaultValue={defaults.embedding}
                  options={catalog?.embedding ?? []}
                  disabled={selectorsDisabled}
                  onSelect={askToChangeEmbedding}
                />
              </div>
              <Button
                type="button"
                variant="outline"
                aria-label="Reset embedding model to default"
                disabled={saving || current.embedding === ''}
                onClick={() => askToChangeEmbedding(defaults.embedding)}
              >
                Reset to default
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">Default: {defaults.embedding}</p>
          </div>
        </div>
        {reimportReminder && (
          <p role="status" className="text-sm text-foreground">
            Embedding model changed. Re-import your notes to index them with the new model.
          </p>
        )}
      </div>

      <EmbeddingChangeDialog
        open={pendingEmbedding !== null}
        pending={saving}
        from={current.embedding || defaults.embedding}
        to={pendingEmbedding || defaults.embedding}
        error={saveError}
        onCancel={() => setPendingEmbedding(null)}
        onConfirm={() => void confirmEmbeddingChange()}
      />
    </div>
  )
}

export { ModelSettingsSection }
