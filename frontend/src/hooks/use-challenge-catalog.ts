import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { listChallengeCatalog, type Catalog, type CatalogFilter } from '@/lib/exercise'

export interface UseChallengeCatalogResult {
  catalog: Catalog | null
  loading: boolean
  error: string | null
  reload: () => void
}

// useChallengeCatalog loads the catalog for the filter, again whenever the
// filter changes (by value, not identity) and on reload(). A response that
// resolves after the filter has changed, or after unmount, is discarded: it
// no longer matches what the screen asks for. The previous catalog stays
// shown while the next one loads, so the list does not flash empty.
export function useChallengeCatalog(filter: CatalogFilter): UseChallengeCatalogResult {
  const { t } = useTranslation()
  const { query, status, difficulty, topic } = filter
  const [catalog, setCatalog] = useState<Catalog | null>(null)
  const [loading, setLoading] = useState(true)
  // Only the failure is kept: the message is written at render, in the language of the moment.
  const [failed, setFailed] = useState(false)
  // Bumping this re-runs the effect below for the same filter.
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    let stale = false
    listChallengeCatalog({ query, status, difficulty, topic })
      .then((result) => {
        if (stale) return
        setCatalog(result)
        setFailed(false)
        setLoading(false)
      })
      .catch(() => {
        if (stale) return
        setCatalog(null)
        setFailed(true)
        setLoading(false)
      })
    return () => {
      stale = true
    }
  }, [query, status, difficulty, topic, reloadToken])

  const reload = useCallback(() => {
    setLoading(true)
    setFailed(false)
    setReloadToken((token) => token + 1)
  }, [])

  return { catalog, loading, error: failed ? t('errors.loadChallenges') : null, reload }
}
