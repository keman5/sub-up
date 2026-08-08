import { computed, ref } from 'vue'

type DialogKind = 'alert' | 'confirm' | 'prompt'
type DialogResolveValue = boolean | string | null

export interface AppDialogOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

export interface AppPromptOptions extends AppDialogOptions {
  defaultValue?: string
  placeholder?: string
  inputType?: 'text' | 'password'
}

export interface AppDialogRequest {
  id: number
  kind: DialogKind
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  defaultValue?: string
  placeholder?: string
  inputType?: 'text' | 'password'
  resolve: (value: DialogResolveValue) => void
}

const queue = ref<AppDialogRequest[]>([])
let dialogId = 0

const currentDialog = computed(() => queue.value[0] ?? null)

function enqueue<T extends DialogResolveValue>(
  kind: DialogKind,
  options: string | AppDialogOptions | AppPromptOptions,
): Promise<T> {
  const normalized = typeof options === 'string' ? { message: options } : options
  return new Promise<T>((resolve) => {
    queue.value.push({
      id: ++dialogId,
      kind,
      ...normalized,
      resolve: resolve as (value: DialogResolveValue) => void,
    })
  })
}

function settleCurrent(value: DialogResolveValue) {
  const request = queue.value.shift()
  request?.resolve(value)
}

export function useAppDialog() {
  return {
    currentDialog,
    alert: (options: string | AppDialogOptions) => enqueue<boolean>('alert', options),
    confirm: (options: string | AppDialogOptions) => enqueue<boolean>('confirm', options),
    askText: (options: string | AppPromptOptions) => enqueue<string | null>('prompt', options),
    settleCurrent,
  }
}
