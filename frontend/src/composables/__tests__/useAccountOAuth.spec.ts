import { describe, expect, it, vi, beforeEach } from 'vitest'
import { useAccountOAuth } from '../useAccountOAuth'

const { generateAuthUrl, exchangeCode, showError } = vi.hoisted(() => ({
  generateAuthUrl: vi.fn(),
  exchangeCode: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      generateAuthUrl,
      exchangeCode,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => ({
      'admin.accounts.oauth.failedToGenerateAuthUrl': '生成授权链接失败',
      'admin.accounts.oauth.failedToExchangeAuthCode': '授权码兑换失败',
      'admin.accounts.oauth.cookieAuthorizationFailed': 'Cookie 授权失败',
    })[key] || key,
  }),
}))

describe('useAccountOAuth', () => {
  beforeEach(() => {
    generateAuthUrl.mockReset()
    exchangeCode.mockReset()
    showError.mockReset()
  })

  it('uses localized fallback when generating a generic auth URL fails without backend detail', async () => {
    generateAuthUrl.mockRejectedValue({})

    const oauth = useAccountOAuth()

    await expect(oauth.generateAuthUrl('oauth')).resolves.toBe(false)
    expect(oauth.error.value).toBe('生成授权链接失败')
    expect(showError).toHaveBeenCalledWith('生成授权链接失败')
  })
})
