import { useCallback, useEffect, useState } from 'react'
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
  const { query, status, difficulty, topic } = filter
  const [catalog, setCatalog] = useState<Catalog | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // Bumping this re-runs the effect below for the same filter.
  const [reloadToken, setReloadToken] = useState(0)

  useEffect(() => {
    let stale = false
    listChallengeCatalog({ query, status, difficulty, topic })
      .then((result) => {
        if (stale) return
        setCatalog(result)
        setError(null)
        setLoading(false)
      })
      .catch(() => {
        if (stale) return
        setCatalog(null)
        setError('Could not load the challenges.')
        setLoading(false)
      })
    return () => {
      stale = true
    }
  }, [query, status, difficulty, topic, reloadToken])

  const reload = useCallback(() => {
    setLoading(true)
    setError(null)
    setReloadToken((token) => token + 1)
  }, [])

  return { catalog, loading, error, reload }
}
