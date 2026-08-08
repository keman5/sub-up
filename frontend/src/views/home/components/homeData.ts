function resolveAppUrl(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const configuredOrigin = import.meta.env.VITE_HOME_APP_ORIGIN?.trim().replace(/\/+$/, '')

  if (!configuredOrigin) {
    return normalizedPath
  }

  try {
    return new URL(normalizedPath, `${configuredOrigin}/`).toString()
  } catch {
    return normalizedPath
  }
}

export const externalAppUrls = {
  login: resolveAppUrl('/login'),
  register: resolveAppUrl('/register'),
  console: resolveAppUrl('/dashboard')
} as const

export const homeSupportEntry = {
  buttonLabel: '联系客服',
  panelTitle: '加入 QQ 群获取支持',
  helperText: '扫码进群，或复制群号后在 QQ 内搜索加入。',
  groupNumber: '550744305',
  qrImagePath: '/qq-support-qr.jpeg'
} as const

export type HeroCodeTab = 'mac' | 'windows' | 'python'

export const heroTabs: Array<{ id: HeroCodeTab; label: string }> = [
  { id: 'mac', label: 'macOS / Linux' },
  { id: 'windows', label: 'Windows' },
  { id: 'python', label: 'integration.py' }
]

export type HeroSnippetBlock = {
  id: string
  title: string
  description: string
  language: string
  code: string
}

type HomeSnippetUrls = {
  apiBaseUrl: string
  claudeBaseUrl: string
}

const codexAuthJson = `{
  "OPENAI_API_KEY": "sk-key"
}`

function buildCodexConfigToml(apiBaseUrl: string): string {
  return `model = "gpt-5.5"
model_provider = "51token"
review_model = "gpt-5.4"
web_search = "live"

[model_providers.51token]
name = "51token"
approval_policy = "on-request"
base_url = "${apiBaseUrl}"
sandbox_mode = "workspace-write" # 或 "danger-full-access"
wire_api = "responses"`
}

function buildClaudeConfigJson(claudeBaseUrl: string): string {
  return `{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "api key",
    "ANTHROPIC_BASE_URL": "${claudeBaseUrl}",
    "ANTHROPIC_MODEL": "gpt-5.5",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "gpt-5.5",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-5.5",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "gpt-5.5",
    "ANTHROPIC_REASONING_MODEL": "gpt-5.5"
  }
}`
}

export function buildHeroSnippetBlocks(urls: HomeSnippetUrls): Record<HeroCodeTab, HeroSnippetBlock[]> {
  const codexConfigToml = buildCodexConfigToml(urls.apiBaseUrl)
  const claudeConfigJson = buildClaudeConfigJson(urls.claudeBaseUrl)

  return {
    mac: [
    {
      id: 'codex-auth',
      title: 'Codex 配置',
      description: '1. 使用 vi ~/.codex/auth.json 全部覆盖',
      language: 'json',
      code: codexAuthJson
    },
    {
      id: 'codex-config',
      title: '',
      description: '2. 使用 vi ~/.codex/config.toml 加入以下内容',
      language: 'toml',
      code: codexConfigToml
    },
    {
      id: 'claude-config',
      title: 'Claude 配置',
      description: '将环境变量配置为以下内容。ANTHROPIC_BASE_URL 包含 /51Token，这里不带 /v1',
      language: 'json',
      code: claudeConfigJson
    }
    ],
    windows: [
    {
      id: 'codex-auth',
      title: 'Codex 配置',
      description: '1. 使用 notepad %USERPROFILE%\\.codex\\auth.json 全部覆盖',
      language: 'json',
      code: codexAuthJson
    },
    {
      id: 'codex-config',
      title: '',
      description: '2. 使用 notepad %USERPROFILE%\\.codex\\config.toml 加入以下内容',
      language: 'toml',
      code: codexConfigToml
    },
    {
      id: 'claude-config',
      title: 'Claude 配置',
      description: '将环境变量配置为以下内容。ANTHROPIC_BASE_URL 包含 /51Token，这里不带 /v1',
      language: 'json',
      code: claudeConfigJson
    }
    ],
    python: [
    {
      id: 'python-sdk',
      title: 'OpenAI SDK 接入示例',
      description: '使用兼容 OpenAI Responses API 的 Python SDK 调用示例',
      language: 'python',
      code: `from openai import OpenAI

client = OpenAI(
    api_key="sk-key",
    base_url="${urls.apiBaseUrl}",
)

response = client.responses.create(
    model="gpt-5.5",
    input="写一个快排算法",
)

print(response.output_text)`
    }
    ]
  }
}

export type IntegrationTab = 'python' | 'nodejs' | 'curl' | 'langchain'

export const integrationTabs: Array<{ id: IntegrationTab; label: string }> = [
  { id: 'python', label: 'Python' },
  { id: 'nodejs', label: 'Node.js' },
  { id: 'curl', label: 'cURL' },
  { id: 'langchain', label: 'LangChain' }
]

export function buildIntegrationSnippets(apiBaseUrl: string): Record<IntegrationTab, string> {
  return {
    python: `import openai

openai.api_base = "${apiBaseUrl}"
openai.api_key = "sk-gw-xxxxxxxxxxxxxxxx"

response = openai.ChatCompletion.create(
    model="codex-pro",
    messages=[
        {"role": "user", "content": "How to optimize React performance?"}
    ],
    stream=True
)

for chunk in response:
    print(chunk.choices[0].delta.content or "", end="")`,
    nodejs: `import { Configuration, OpenAIApi } from "openai";

const configuration = new Configuration({
  apiKey: "sk-gw-xxxxxxxxxxxxxxxx",
  basePath: "${apiBaseUrl}",
});

const openai = new OpenAIApi(configuration);

const completion = await openai.createChatCompletion({
  model: "codex-pro",
  messages: [{ role: "user", content: "Optimize this algorithm..." }],
});

console.log(completion.data.choices[0].message);`,
    curl: `curl ${apiBaseUrl}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-gw-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "codex-pro",
    "messages": [{"role": "user", "content": "Hello World!"}]
  }'`,
    langchain: `from langchain.chat_models import ChatOpenAI
from langchain.schema import HumanMessage

chat = ChatOpenAI(
    openai_api_base="${apiBaseUrl}",
    openai_api_key="sk-gw-xxxxxxxxxxxxxxxx",
    model_name="codex-pro"
)

response = chat([HumanMessage(content="Explain quantum computing.")])
print(response.content)`
  }
}

export const featureCards = [
  {
    title: 'OpenAI 官方账号与额度',
    description: '接入真实 OpenAI 官方账号资源与 Codex Pro 额度，不是套壳模型，也不是自建模型替代品。',
    icon: 'badge'
  },
  {
    title: '官方能力稳定透传',
    description: '保留 ChatGPT 与 Codex 的原生能力边界，流式输出、工具调用和上下文表现更接近官方体验。',
    icon: 'shield'
  },
  {
    title: '解决购买渠道问题',
    description: '面向国内团队降低海外账号、订阅、支付与额度维护门槛，把复杂采购流程收敛为一个可用接口。',
    icon: 'creditCard'
  },
  {
    title: '专业稳定 Codex 通道',
    description: '当前聚焦 ChatGPT 与 Codex 场景，不盲目堆渠道，优先保障核心模型的稳定性与可用体验。',
    icon: 'server'
  },
  {
    title: '完全兼容 OpenAI 协议',
    description: '业务代码只需修改 API Base 与 API Key，即可无缝迁移，兼容主流 OpenAI SDK 与开发工具链。',
    icon: 'terminal'
  },
  {
    title: '团队额度统一管理',
    description: '不同成员、项目或客户可使用独立 Key，额度、过期时间、权限范围和使用记录都能分开管理。',
    icon: 'users'
  },
  {
    title: '高可用账号池调度',
    description: '系统持续监控账号状态、频率限制与失败请求，自动切换可用资源，减少单账号不可用带来的中断。',
    icon: 'sync'
  },
  {
    title: '国内网络友好',
    description: '面向本地开发、服务器部署和 CI/CD 流水线提供一致入口，减少网络环境差异带来的连接失败与排查成本。',
    icon: 'globe'
  }
] as const

export const pricingPlans = [
  {
    name: '基础开发者',
    price: '75',
    frequency: '/ 个',
    description: '适用于个人开发者测试与小规模内部系统接入。',
    features: [
      '$300 固定额度',
      '不限使用时间，用完即止',
      '共享 Codex 基础速率',
      '基础并发限流 (3次/秒)',
      '标准请求响应速度',
      '过去 7 天调用日志',
      '免费社区技术支持'
    ],
    highlighted: false
  },
  {
    name: 'Plus 资源合租',
    price: '128',
    frequency: '/ 个',
    description: '独立团队或中小型企业，平摊高昂的 Plus 账号费用。',
    features: [
      '$600 固定额度',
      '不限使用时间，用完即止',
      '独享或少量共享的 Plus 级速率',
      '放宽并发限制 (20次/秒)',
      '专属加速通道与高可用路由',
      '近 30 天详细调用明细分析',
      '自定义子账号限额与分组管理',
      '7x24 小时随时支持'
    ],
    highlighted: true
  },
  {
    name: 'Pro 资源合租',
    price: '300',
    frequency: '/ 月',
    description: '为高端业务场景定制，提供无缝的 Pro 层级极致体验。',
    features: [
      '$2000 固定额度',
      '按官方 5 小时限额与周限额同步使用',
      '独享或高级优化的 Pro 级速率',
      '极致并发与极低延迟节点',
      '专用 API 域名与独立网关',
      '无限制的全局数据分析与导出',
      '企业级子账号及权限分配',
      '7x24 小时专属工单响应'
    ],
    highlighted: false
  }
] as const

export const shortTermPlans = [
  { name: '1日小包', price: '5', tokens: '$20 额度', equivalent: '适合轻量测试与临时验证' },
  { name: '1日中包', price: '15', tokens: '$60 额度', equivalent: '适合一天内集中开发调试' },
  { name: '1日大包', price: '40', tokens: '$160 额度', equivalent: '适合短期演示与高频测试' },
  { name: '1周小包', price: '35', tokens: '$140 额度', equivalent: '适合一周内低频稳定使用' },
  { name: '1周中包', price: '84', tokens: '$336 额度', equivalent: '适合小团队阶段性开发' },
  { name: '1周大包', price: '168', tokens: '$672 额度', equivalent: '适合高频开发与项目冲刺' }
] as const

export const faqs = [
  {
    question: '51token 和官方 API 有什么区别？',
    answer: '调用方式保持 OpenAI 协议兼容，你只需要替换 API Base 与 API Key。我们负责聚合可用资源、维护线路和处理额度分发，让国内开发者更容易稳定接入 Codex/ChatGPT 能力。'
  },
  {
    question: '迁移现有项目需要多久？',
    answer: '大多数项目只需要几分钟。OpenAI SDK、LangChain、Dify、自写 curl 调用等场景通常只改两行配置，模型名、流式响应和工具调用会按兼容协议继续工作。'
  },
  {
    question: '适合哪些使用场景？',
    answer: '适合个人开发者、AI 工具团队、内部研发、自动化脚本、CI/CD、插件开发和短期项目交付。尤其适合想使用 Codex Pro 能力，但不想反复处理海外账号、订阅和网络问题的团队。'
  },
  {
    question: '速度和稳定性怎么样？',
    answer: '我们会持续维护可用通道和线路状态，优先保障首 token 响应、流式输出和高频开发场景的稳定体验。不同地区网络会有差异，建议先购买短期包实测。'
  },
  {
    question: '额度用完或临时测试怎么办？',
    answer: '可以选择按需购买固定额度包，也可以使用日包、周包等短期方案。短期包适合临时验证、项目演示和集中开发，避免一开始就绑定长期成本。'
  },
  {
    question: '遇到接入问题如何联系？',
    answer: '可以加入客服 QQ 群 550744305。建议带上你的使用场景、请求方式、报错信息和大概时间，方便更快定位配置或网络问题。'
  }
] as const
