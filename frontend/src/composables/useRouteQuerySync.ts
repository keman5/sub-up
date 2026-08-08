import { getCurrentInstance } from 'vue'
import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

type QueryPrimitive = string | number | boolean | null | undefined
type QueryParseMode = 'string' | 'number' | 'boolean'

export interface RouteQuerySyncField<TValue extends QueryPrimitive = QueryPrimitive> {
  queryKey: string
  get: () => TValue
  set: (value: any) => void
  defaultValue?: TValue
  defaultQueryValue?: string
  emptyQueryValue?: string
  parse?: QueryParseMode
}

interface UseRouteQuerySyncOptions {
  route?: RouteLocationNormalizedLoaded
  router?: Router
  fields: RouteQuerySyncField[]
}

const windowQuery = (): Record<string, string> => {
  if (typeof window === 'undefined') return {}
  const params = new URLSearchParams(window.location.search)
  return Object.fromEntries(params.entries())
}

const firstQueryValue = (value: unknown): string | undefined => {
  if (Array.isArray(value)) {
    return value.find((item): item is string => typeof item === 'string' && item.length > 0)
  }
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

const parseQueryValue = (raw: string, mode: QueryParseMode = 'string') => {
  if (mode === 'number') {
    const parsed = Number(raw)
    return Number.isFinite(parsed) ? parsed : null
  }
  if (mode === 'boolean') {
    if (raw === 'true' || raw === '1') return true
    if (raw === 'false' || raw === '0') return false
    return null
  }
  return raw
}

const isEmptyQueryValue = (value: QueryPrimitive): boolean => {
  return value == null || value === ''
}

const queryStringValue = (value: QueryPrimitive): string | undefined => {
  if (isEmptyQueryValue(value)) return undefined
  return String(value)
}

export function useRouteQuerySync(options: UseRouteQuerySyncOptions) {
  const instanceProxy = getCurrentInstance()?.proxy
  const instanceRoute = instanceProxy?.$route as RouteLocationNormalizedLoaded | undefined
  const instanceRouter = instanceProxy?.$router as Router | undefined
  const route = options.route ?? instanceRoute ?? ({ query: windowQuery() } as RouteLocationNormalizedLoaded)
  const router = options.router ?? instanceRouter

  const restoreFromRoute = () => {
    for (const field of options.fields) {
      const raw = firstQueryValue(route.query[field.queryKey])
      if (raw == null) continue
      if (field.defaultQueryValue !== undefined && raw === field.defaultQueryValue && field.defaultValue !== undefined) {
        field.set(field.defaultValue)
        continue
      }
      if (field.emptyQueryValue !== undefined && raw === field.emptyQueryValue) {
        field.set('')
        continue
      }
      const parsed = parseQueryValue(raw, field.parse)
      if (parsed != null) {
        field.set(parsed)
      }
    }
  }

  const syncToRoute = async () => {
    const nextQuery: Record<string, string> = {}
    const sourceQuery = router?.replace ? route.query : windowQuery()
    for (const [key, value] of Object.entries(sourceQuery)) {
      const raw = firstQueryValue(value)
      if (raw != null) {
        nextQuery[key] = raw
      }
    }

    for (const field of options.fields) {
      const value = field.get()
      const defaultValue = field.defaultValue
      if (defaultValue !== undefined && value === defaultValue) {
        if (field.defaultQueryValue !== undefined) {
          nextQuery[field.queryKey] = field.defaultQueryValue
        } else {
          delete nextQuery[field.queryKey]
        }
        continue
      }
      if (isEmptyQueryValue(value)) {
        if (field.emptyQueryValue !== undefined) {
          nextQuery[field.queryKey] = field.emptyQueryValue
        } else {
          delete nextQuery[field.queryKey]
        }
        continue
      }
      const serialized = queryStringValue(value)
      if (serialized == null) {
        delete nextQuery[field.queryKey]
        continue
      }
      nextQuery[field.queryKey] = serialized
    }

    if (router?.replace) {
      await router.replace({ query: nextQuery })
      return
    }

    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href)
      url.search = ''
      for (const [key, value] of Object.entries(nextQuery)) {
        url.searchParams.set(key, value)
      }
      window.history.replaceState(window.history.state, '', url)
    }
  }

  return {
    restoreFromRoute,
    syncToRoute,
  }
}
