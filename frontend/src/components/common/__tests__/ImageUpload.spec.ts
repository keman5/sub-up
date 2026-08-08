import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const messages: Record<string, string> = {
          'common.fileTooLarge': `文件过大 (${params?.size} KB)，最大 ${params?.max} KB`,
          'common.selectImageFile': '请选择图片文件',
          'common.fileReadFailed': '读取文件失败',
        }
        return messages[key] ?? key
      },
    }),
  }
})

import ImageUpload from '../ImageUpload.vue'

function setInputFile(input: HTMLInputElement, file: File) {
  Object.defineProperty(input, 'files', {
    value: [file],
    configurable: true,
  })
}

describe('ImageUpload', () => {
  it('shows localized file size validation error', async () => {
    const wrapper = mount(ImageUpload, {
      props: {
        modelValue: '',
        maxSize: 1024,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const input = wrapper.get('input[type="file"]').element as HTMLInputElement
    setInputFile(input, new File(['x'.repeat(2048)], 'large.png', { type: 'image/png' }))
    await wrapper.get('input[type="file"]').trigger('change')

    expect(wrapper.text()).toContain('文件过大 (2.0 KB)，最大 1 KB')
  })
})
