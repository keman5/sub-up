<template>
  <section class="relative border-t border-[var(--border-light)] bg-background py-16 md:py-20">
    <div class="container-main relative z-10">
      <div class="mb-16 text-center">
        <h2 class="font-display mb-4 text-3xl font-bold tracking-tight break-words text-foreground md:text-4xl">
          常见问题
        </h2>
        <p class="text-lg break-words text-muted-foreground">
          围绕接入、权限、计费和运维体验的核心问题。
        </p>
      </div>

      <div class="space-y-4">
        <article
          v-for="(faq, index) in faqs"
          :key="faq.question"
          class="panel-card min-w-0 bg-background/50 landing-animate-fade-up"
          :style="{ animationDelay: `${index * 100}ms` }"
        >
          <button type="button" class="flex w-full min-w-0 items-center justify-between gap-4 p-6 text-left focus:outline-none" @click="openIndex = openIndex === index ? -1 : index">
            <span class="min-w-0 text-lg font-medium break-words text-foreground">{{ faq.question }}</span>
            <Icon
              name="chevronDown"
              size="md"
              class="size-5 shrink-0 text-muted-foreground transition-transform duration-300"
              :class="openIndex === index ? 'rotate-180' : ''"
            />
          </button>
          <div
            class="overflow-hidden transition-all duration-300 ease-in-out"
            :class="openIndex === index ? 'max-h-96 opacity-100' : 'max-h-0 opacity-0'"
          >
            <div class="px-6 pb-6 leading-relaxed break-words text-muted-foreground">
              {{ faq.answer }}
            </div>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { faqs } from './homeData'

const openIndex = ref(0)
</script>
