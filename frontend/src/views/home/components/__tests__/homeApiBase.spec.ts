import { describe, expect, it } from 'vitest'
import { buildHeroSnippetBlocks } from '../homeData'
import { buildClaudeBaseUrl, buildHomeSnippetUrls } from '../homeApiBase'

describe('homeApiBase', () => {
  it('keeps the 51Token path for Claude Code and omits v1', () => {
    expect(buildClaudeBaseUrl('https://api.upit.top/51Token/v1')).toBe('https://api.upit.top/51Token')
  })

  it('adds 51Token path when Claude Code base is derived from a bare domain', () => {
    expect(buildClaudeBaseUrl('https://api.upit.top')).toBe('https://api.upit.top/51Token')
  })

  it('uses configured API base before falling back to the frontend origin', () => {
    expect(buildHomeSnippetUrls('https://a2t.upit.top/51Token/v1', 'https://test.upit.top')).toEqual({
      apiBaseUrl: 'https://a2t.upit.top/51Token/v1',
      claudeBaseUrl: 'https://a2t.upit.top/51Token'
    })
  })

  it('renders full Claude Code home config with 51Token base path', () => {
    const blocks = buildHeroSnippetBlocks({
      apiBaseUrl: 'https://api.upit.top/51Token/v1',
      claudeBaseUrl: 'https://api.upit.top/51Token'
    })
    const claudeConfig = blocks.mac.find((block) => block.id === 'claude-config')

    expect(claudeConfig?.description).toContain('不带 /v1')
    expect(claudeConfig?.code).toContain('"ANTHROPIC_BASE_URL": "https://api.upit.top/51Token"')
    expect(claudeConfig?.code).not.toContain('https://api.upit.top/51Token/v1')
    expect(claudeConfig?.code).toContain('"ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-5.5"')
    expect(claudeConfig?.code).toContain('"ANTHROPIC_DEFAULT_OPUS_MODEL": "gpt-5.5"')
    expect(claudeConfig?.code).toContain('"ANTHROPIC_REASONING_MODEL": "gpt-5.5"')
    expect(claudeConfig?.code).toContain('"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"')
    expect(claudeConfig?.code).toContain('"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"')
  })
})
