<template>
  <section id="integrations" class="relative overflow-hidden border-t border-[var(--border-light)] bg-background py-16 md:py-20">
    <div class="container-main relative z-10">
      <div class="grid min-w-0 items-center gap-16 lg:grid-cols-2">
        <div class="landing-animate-fade-right min-w-0">
          <h2 class="font-display mb-6 text-3xl font-bold tracking-tight break-words text-foreground md:text-4xl">
            两行代码
            <br />
            完成底层架构平替
          </h2>
          <p class="mb-8 text-lg leading-relaxed break-words text-muted-foreground">
            网关 100% 遵守开源生态的事实标准。无论你使用官方 SDK、LangChain、LlamaIndex 还是其他生态工具，只需替换 API 地址和密钥，瞬间接入。
          </p>

          <ul class="space-y-4">
            <li v-for="item in bullets" :key="item" class="flex min-w-0 items-start gap-3">
              <span class="flex size-6 shrink-0 items-center justify-center rounded-full border border-success/20 bg-success/10 text-success">
                <Icon name="check" size="xs" />
              </span>
              <span class="min-w-0 break-words text-foreground/90">{{ item }}</span>
            </li>
          </ul>
        </div>

        <div class="landing-animate-scale-in min-w-0">
          <div class="panel-card overflow-hidden bg-background/50 shadow-2xl backdrop-blur-md">
            <div class="hide-scrollbar flex flex-wrap gap-x-1 overflow-hidden border-b border-[var(--border)] bg-muted/30 px-4">
              <button
                v-for="tab in integrationTabs"
                :key="tab.id"
                class="min-w-0 border-b-2 px-3 py-3 text-sm font-medium break-words transition-colors sm:px-4 sm:whitespace-nowrap"
                :class="activeTab === tab.id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'"
                @click="activeTab = tab.id"
              >
                {{ tab.label }}
              </button>
            </div>
            <div class="w-full min-w-0 overflow-x-hidden p-4 sm:overflow-x-auto sm:p-6">
              <pre class="overflow-x-hidden font-mono text-xs leading-relaxed break-all whitespace-pre-wrap sm:text-sm"><code class="text-foreground/90">{{ activeSnippet }}</code></pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { buildIntegrationSnippets, integrationTabs, type IntegrationTab } from './homeData'
import { buildHomeSnippetUrls } from './homeApiBase'

const props = defineProps<{
  apiBaseUrl?: string
}>()

const activeTab = ref<IntegrationTab>('python')
const snippetUrls = computed(() => buildHomeSnippetUrls(props.apiBaseUrl))
const integrationSnippets = computed(() => buildIntegrationSnippets(snippetUrls.value.apiBaseUrl))
const activeSnippet = computed(() => integrationSnippets.value[activeTab.value])
const bullets = [
  '完美兼容所有围绕 OpenAI 封装的开源库',
  '支持原生 Stream 流式输出，视觉无感知延迟',
  '支持 Function Calling 等高级模型特性转发'
]
</script>
