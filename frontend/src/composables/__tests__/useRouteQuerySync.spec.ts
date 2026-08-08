import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useRouteQuerySync } from '../useRouteQuerySync'

describe('useRouteQuerySync', () => {
  const replace = vi.fn()
  const route = reactive<{ query: Record<string, string | undefined> }>({ query: {} })

  beforeEach(() => {
    replace.mockReset()
    route.query = {}
    window.history.replaceState(null, '', '/')
  })

  it('restores configured fields from the current route query', () => {
    route.query = {
      search: 'alice',
      status: 'disabled',
      group_id: '12',
      ignored: 'keep'
    }
    const state = reactive({
      search: '',
      status: 'active',
      groupId: null as number | null
    })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        { queryKey: 'search', get: () => state.search, set: (value) => { state.search = value }, defaultValue: '' },
        { queryKey: 'status', get: () => state.status, set: (value) => { state.status = value }, defaultValue: 'active' },
        { queryKey: 'group_id', get: () => state.groupId, set: (value) => { state.groupId = value }, parse: 'number' },
      ],
    })

    sync.restoreFromRoute()

    expect(state).toEqual({
      search: 'alice',
      status: 'disabled',
      groupId: 12,
    })
  })

  it('writes non-default field values to the route query and preserves unrelated params', async () => {
    route.query = { tab: 'usage', search: 'old' }
    const state = reactive({
      search: 'spark',
      status: 'active',
      groupId: 8 as number | null
    })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        { queryKey: 'search', get: () => state.search, set: (value) => { state.search = value }, defaultValue: '' },
        { queryKey: 'status', get: () => state.status, set: (value) => { state.status = value }, defaultValue: 'active' },
        { queryKey: 'group_id', get: () => state.groupId, set: (value) => { state.groupId = value }, parse: 'number' },
      ],
    })

    await sync.syncToRoute()

    expect(replace).toHaveBeenCalledWith({
      query: {
        tab: 'usage',
        search: 'spark',
        group_id: '8',
      },
    })
  })

  it('removes empty and default values from the route query', async () => {
    route.query = { search: 'alice', status: 'disabled', group_id: '8' }
    const state = reactive({
      search: '',
      status: 'active',
      groupId: null as number | null
    })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        { queryKey: 'search', get: () => state.search, set: (value) => { state.search = value }, defaultValue: '' },
        { queryKey: 'status', get: () => state.status, set: (value) => { state.status = value }, defaultValue: 'active' },
        { queryKey: 'group_id', get: () => state.groupId, set: (value) => { state.groupId = value }, parse: 'number' },
      ],
    })

    await sync.syncToRoute()

    expect(replace).toHaveBeenCalledWith({ query: {} })
  })

  it('preserves query params written by another sync instance when using history fallback', async () => {
    window.history.replaceState(null, '', '/?tab=usage')
    const searchState = reactive({ search: 'spark' })
    const logState = reactive({ level: 'error' })

    const searchSync = useRouteQuerySync({
      fields: [
        { queryKey: 'search', get: () => searchState.search, set: (value) => { searchState.search = value }, defaultValue: '' },
      ],
    })
    const logSync = useRouteQuerySync({
      fields: [
        { queryKey: 'log_level', get: () => logState.level, set: (value) => { logState.level = value }, defaultValue: '' },
      ],
    })

    await searchSync.syncToRoute()
    expect(window.location.search).toBe('?tab=usage&search=spark')

    await logSync.syncToRoute()

    expect(window.location.search).toBe('?tab=usage&search=spark&log_level=error')
  })

  it('can serialize an all-option default value with an explicit query token', async () => {
    route.query = { tab: 'accounts' }
    const state = reactive({ status: '' })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        {
          queryKey: 'status',
          get: () => state.status,
          set: (value) => { state.status = value },
          defaultValue: '',
          defaultQueryValue: 'all',
        },
      ],
    })

    await sync.syncToRoute()

    expect(replace).toHaveBeenCalledWith({
      query: {
        tab: 'accounts',
        status: 'all',
      },
    })

    route.query = { status: 'all' }
    state.status = 'active'
    sync.restoreFromRoute()

    expect(state.status).toBe('')
  })

  it('restores an explicit empty query token even when the field has a non-empty default', () => {
    route.query = { status: 'all' }
    const state = reactive({ status: 'active' })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        {
          queryKey: 'status',
          get: () => state.status,
          set: (value) => { state.status = value },
          defaultValue: 'active',
          emptyQueryValue: 'all',
        },
      ],
    })

    sync.restoreFromRoute()

    expect(state.status).toBe('')
  })

  it('can keep a non-empty default value explicit while using another token for all', async () => {
    route.query = { tab: 'accounts' }
    const state = reactive({ status: 'active' })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        {
          queryKey: 'status',
          get: () => state.status,
          set: (value) => { state.status = value },
          defaultValue: 'active',
          defaultQueryValue: 'active',
          emptyQueryValue: 'all',
        },
      ],
    })

    await sync.syncToRoute()

    expect(replace).toHaveBeenCalledWith({
      query: {
        tab: 'accounts',
        status: 'active',
      },
    })

    route.query = { status: 'active' }
    state.status = ''
    sync.restoreFromRoute()

    expect(state.status).toBe('active')
  })

  it('can serialize undefined all-option values with an explicit empty query token', async () => {
    route.query = { tab: 'usage' }
    const state = reactive({ groupId: undefined as number | undefined })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        {
          queryKey: 'group_id',
          get: () => state.groupId,
          set: (value) => { state.groupId = value },
          parse: 'number',
          defaultValue: null,
          defaultQueryValue: 'all',
          emptyQueryValue: 'all',
        },
      ],
    })

    await sync.syncToRoute()

    expect(replace).toHaveBeenCalledWith({
      query: {
        tab: 'usage',
        group_id: 'all',
      },
    })
  })

  it('restores all-option tokens to null for nullable numeric filters', () => {
    route.query = { group_id: 'all' }
    const state = reactive({ groupId: 12 as number | null | string })

    const sync = useRouteQuerySync({
      route: route as any,
      router: { replace } as any,
      fields: [
        {
          queryKey: 'group_id',
          get: () => state.groupId,
          set: (value) => { state.groupId = value },
          parse: 'number',
          defaultValue: null,
          defaultQueryValue: 'all',
          emptyQueryValue: 'all',
        },
      ],
    })

    sync.restoreFromRoute()

    expect(state.groupId).toBeNull()
  })
})
