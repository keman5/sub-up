<template>
  <span class="inline-flex min-h-[1.1em] max-w-full items-baseline justify-center overflow-hidden tabular-nums" :aria-label="displayValue">
    <span
      v-for="(char, index) in displayChars"
      :key="`${index}-${char}`"
      class="relative inline-block h-[1.1em] shrink-0 overflow-hidden"
    >
      <span class="home-counter-char block min-w-[0.55em]">
        {{ char }}
      </span>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    end: number
    duration?: number
    decimals?: number
    formatter?: (value: number) => string
  }>(),
  {
    duration: 1000,
    decimals: 0,
    formatter: undefined
  }
)

const displayValue = ref(formatValue(0))
const displayChars = computed(() => Array.from(displayValue.value))

function formatValue(value: number) {
  if (props.formatter) return props.formatter(value)
  return props.decimals > 0 ? value.toFixed(props.decimals) : Math.round(value).toLocaleString()
}

function animate() {
  const start = performance.now()
  const step = (now: number) => {
    const progress = Math.min((now - start) / props.duration, 1)
    const eased = 1 - Math.pow(1 - progress, 3)
    displayValue.value = formatValue(eased * props.end)
    if (progress < 1) requestAnimationFrame(step)
  }
  requestAnimationFrame(step)
}

onMounted(() => {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    displayValue.value = formatValue(props.end)
    return
  }
  animate()
})
</script>
