import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import SearchSuggestInput from '../SearchSuggestInput.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('SearchSuggestInput', () => {
  it('emits model updates and debounced search text while typing', async () => {
    vi.useFakeTimers()

    const wrapper = mount(SearchSuggestInput, {
      props: {
        modelValue: '',
        suggestions: [],
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const input = wrapper.get('input')
    await input.setValue('alice@example.com')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['alice@example.com'])

    vi.advanceTimersByTime(300)
    expect(wrapper.emitted('search')?.at(-1)).toEqual(['alice@example.com'])

    vi.useRealTimers()
  })

  it('refills visible text and emits select when choosing a suggestion', async () => {
    const wrapper = mount(SearchSuggestInput, {
      props: {
        modelValue: 'ali',
        suggestions: [
          {
            id: 'u1',
            primaryText: 'alice@example.com',
            secondaryText: 'alice / VIP',
            value: { email: 'alice@example.com' },
          },
        ],
        open: true,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const option = wrapper.get('[data-test="search-suggest-option"]')
    await option.trigger('mousedown')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['alice@example.com'])
    expect(wrapper.emitted('select')?.at(-1)?.[0]).toMatchObject({
      id: 'u1',
      primaryText: 'alice@example.com',
    })
  })

  it('shows clear button and clears visible text', async () => {
    const wrapper = mount(SearchSuggestInput, {
      props: {
        modelValue: 'bob@example.com',
        suggestions: [],
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const clear = wrapper.get('[data-test="search-suggest-clear"]')
    await clear.trigger('click')

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([''])
    expect(wrapper.emitted('clear')).toBeTruthy()
    expect(wrapper.emitted('search')?.at(-1)).toEqual([''])
  })
})
