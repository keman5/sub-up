<template>
  <section id="integrations" class="relative overflow-hidden border-t border-[var(--border-light)] bg-background py-16 md:py-20">
    <div class="container-main relative z-10">
      <div class="grid items-center gap-16 lg:grid-cols-2">
        <div class="landing-animate-fade-right">
          <h2 class="font-display mb-6 text-3xl font-bold tracking-tight text-foreground md:text-4xl">
            两行代码
            <br />
            完成底层架构平替
          </h2>
          <p class="mb-8 text-lg leading-relaxed text-muted-foreground">
            网关 100% 遵守开源生态的事实标准。无论你使用官方 SDK、LangChain、LlamaIndex 还是其他生态工具，只需替换 API 地址和密钥，瞬间接入。
          </p>

          <ul class="space-y-4">
            <li v-for="item in bullets" :key="item" class="flex items-center gap-3">
              <span class="flex size-6 shrink-0 items-center justify-center rounded-full border border-success/20 bg-success/10 text-success">
                <Icon name="check" size="xs" />
              </span>
              <span class="text-foreground/90">{{ item }}</span>
            </li>
          </ul>
        </div>

        <div class="landing-animate-scale-in">
          <div class="panel-card overflow-hidden bg-background/50 shadow-2xl backdrop-blur-md">
            <div class="hide-scrollbar flex overflow-x-auto border-b border-[var(--border)] bg-muted/30 px-4">
              <button
                v-for="tab in integrationTabs"
                :key="tab.id"
                class="border-b-2 px-4 py-3 text-sm font-medium whitespace-nowrap transition-colors"
                :class="activeTab === tab.id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'"
                @click="activeTab = tab.id"
              >
                {{ tab.label }}
              </button>
            </div>
            <div class="w-full overflow-x-auto p-6">
              <pre class="font-mono text-sm leading-relaxed break-all whitespace-pre-wrap"><code class="text-foreground/90">{{ activeSnippet }}</code></pre>
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
import { integrationSnippets, integrationTabs, type IntegrationTab } from './homeData'

const activeTab = ref<IntegrationTab>('python')
const activeSnippet = computed(() => integrationSnippets[activeTab.value])
const bullets = [
  '完美兼容所有围绕 OpenAI 封装的开源库',
  '支持原生 Stream 流式输出，视觉无感知延迟',
  '支持 Function Calling 等高级模型特性转发'
]
</script>
