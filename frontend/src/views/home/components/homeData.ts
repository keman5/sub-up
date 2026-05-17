export const externalAppUrls = {
  login: 'https://ai.upit.top/login',
  register: 'https://ai.upit.top/register',
  console: '/admin/dashboard'
} as const

export type HeroCodeTab = 'mac' | 'windows' | 'python'

export const heroTabs: Array<{ id: HeroCodeTab; label: string }> = [
  { id: 'mac', label: 'macOS/Linux' },
  { id: 'windows', label: 'Windows' },
  { id: 'python', label: 'integration.py' }
]

export const heroSnippets: Record<HeroCodeTab, string> = {
  mac: `# macOS / Linux 环境设置命令

# 临时设置 (当前终端有效)
export OPENAI_API_BASE="https://api.upit.top/51Token/v1"
export OPENAI_API_KEY="sk-gw-xxxxxxxx"

# 永久设置
echo 'export OPENAI_API_BASE="https://api.upit.top/51Token/v1"' >> ~/.zshrc
echo 'export OPENAI_API_KEY="sk-gw-xxxxxxxx"' >> ~/.zshrc
source ~/.zshrc`,
  windows: `# Windows 环境设置命令 (cmd / powershell)

# 临时设置 (当前控制台有效)
set OPENAI_API_BASE=https://api.upit.top/51Token/v1
set OPENAI_API_KEY=sk-gw-xxxxxxxx

# 永久设置 (修改系统变量)
setx OPENAI_API_BASE "https://api.upit.top/51Token/v1"
setx OPENAI_API_KEY "sk-gw-xxxxxxxx"`,
  python: `import openai

# 只需要修改两行代码，无缝切换至 Gateway
openai.api_base = "https://api.upit.top/51Token/v1"
openai.api_key = "sk-gw-xxxxxxxx"

# 像往常一样发请求，内部自动映射加速
response = openai.ChatCompletion.create(
  model="codex-pro",
  messages=[{"role": "user", "content": "写一个快排算法"}]
)

print(response.choices[0].message.content)`
}

export type IntegrationTab = 'python' | 'nodejs' | 'curl' | 'langchain'

export const integrationTabs: Array<{ id: IntegrationTab; label: string }> = [
  { id: 'python', label: 'Python' },
  { id: 'nodejs', label: 'Node.js' },
  { id: 'curl', label: 'cURL' },
  { id: 'langchain', label: 'LangChain' }
]

export const integrationSnippets: Record<IntegrationTab, string> = {
  python: `import openai

openai.api_base = "https://api.upit.top/51Token/v1"
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
  basePath: "https://api.upit.top/51Token/v1",
});

const openai = new OpenAIApi(configuration);

const completion = await openai.createChatCompletion({
  model: "codex-pro",
  messages: [{ role: "user", content: "Optimize this algorithm..." }],
});

console.log(completion.data.choices[0].message);`,
  curl: `curl https://api.upit.top/51Token/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-gw-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "codex-pro",
    "messages": [{"role": "user", "content": "Hello World!"}]
  }'`,
  langchain: `from langchain.chat_models import ChatOpenAI
from langchain.schema import HumanMessage

chat = ChatOpenAI(
    openai_api_base="https://api.upit.top/51Token/v1",
    openai_api_key="sk-gw-xxxxxxxxxxxxxxxx",
    model_name="codex-pro"
)

response = chat([HumanMessage(content="Explain quantum computing.")])
print(response.content)`
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
    price: '80',
    frequency: '/ 个',
    description: '适用于个人开发者测试与小规模内部系统接入。',
    features: [
      '$500 固定额度',
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
    price: '120',
    frequency: '/ 个',
    description: '独立团队或中小型企业，平摊高昂的 Plus 账号费用。',
    features: [
      '$850 固定额度',
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
    price: '400',
    frequency: '/ 个',
    description: '为高端业务场景定制，提供无缝的 Pro 层级极致体验。',
    features: [
      '$2500 固定额度',
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
  { name: '1日小包', price: '5', tokens: '$33 额度', equivalent: '适合轻量测试与临时验证' },
  { name: '1日中包', price: '15', tokens: '$100 额度', equivalent: '适合一天内集中开发调试' },
  { name: '1日大包', price: '30', tokens: '$200 额度', equivalent: '适合短期演示与高频测试' },
  { name: '1周小包', price: '35', tokens: '$233 额度', equivalent: '适合一周内低频稳定使用' },
  { name: '1周中包', price: '84', tokens: '$560 额度', equivalent: '适合小团队阶段性开发' },
  { name: '1周大包', price: '168', tokens: '$1120 额度', equivalent: '适合高频开发与项目冲刺' }
] as const

export const faqs = [
  {
    question: '我们和官方 API 有什么区别？',
    answer: '业务侧仍按兼容协议调用，只是请求先进入你的统一网关，再由网关根据渠道、额度、负载和权限转发到真实上游。'
  },
  {
    question: '迁移现有项目需要多久？',
    answer: '通常只需要替换 API Base 与 API Key。模型名、流式响应、函数调用等能力会按照系统配置继续转发。'
  },
  {
    question: '能否限制团队成员的用量？',
    answer: '可以。你可以通过用户、令牌、分组、模型权限、额度和过期时间进行细粒度控制，并在日志中审计调用情况。'
  },
  {
    question: '后台页面也会跟随新主题吗？',
    answer: '会。此次迁移将黑白主题写入全局语义变量，导航、侧边栏、表格、弹窗和设置页都会使用同一套颜色基础。'
  }
] as const
