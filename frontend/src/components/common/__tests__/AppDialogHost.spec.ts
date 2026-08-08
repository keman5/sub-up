import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

import AppDialogHost from '../AppDialogHost.vue'
import { useAppDialog } from '@/composables/useAppDialog'

describe('AppDialogHost', () => {
  it('renders app dialogs above ordinary modal forms', async () => {
    const i18n = createI18n({
      legacy: false,
      locale: 'zh',
      messages: {
        zh: {
          common: {
            cancel: '取消',
            confirm: '确认'
          }
        }
      }
    })

    const wrapper = mount(AppDialogHost, {
      global: {
        plugins: [i18n],
        stubs: {
          BaseDialog: {
            props: ['show', 'title', 'width', 'closeOnClickOutside', 'zIndex'],
            template: '<div v-if="show" data-test="base-dialog" :data-z-index="zIndex"><slot /><slot name="footer" /></div>'
          }
        }
      }
    })

    const dialog = useAppDialog()
    const pending = dialog.confirm({ title: '确认覆盖', message: '确认要覆盖旧订阅吗?' })
    await wrapper.vm.$nextTick()

    expect(wrapper.get('[data-test="base-dialog"]').attributes('data-z-index')).toBe('100')

    dialog.settleCurrent(false)
    await pending
  })
})
