<template>
  <span
    class="account-health-liquid-gauge"
    :class="`account-health-liquid-gauge--${state}`"
    aria-hidden="true"
    data-testid="account-health-liquid-gauge"
  >
    <span
      class="account-health-liquid-gauge__fill"
      :style="{ height: `${normalizedScore}%` }"
      data-testid="account-health-liquid-fill"
    >
      <span
        ref="primaryWave"
        class="account-health-liquid-gauge__wave account-health-liquid-gauge__wave--primary"
      ></span>
      <span
        ref="secondaryWave"
        class="account-health-liquid-gauge__wave account-health-liquid-gauge__wave--secondary"
      ></span>
    </span>
    <span class="account-health-liquid-gauge__highlight"></span>
  </span>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import gsap from 'gsap'
import type { AccountHealthSnapshot } from '@/types'

const props = defineProps<{
  score: number
  state: AccountHealthSnapshot['state']
}>()

const normalizedScore = computed(() => {
  const score = Number(props.score)
  if (!Number.isFinite(score)) return 0
  return Math.max(0, Math.min(100, score))
})

const primaryWave = ref<HTMLElement | null>(null)
const secondaryWave = ref<HTMLElement | null>(null)
let motionContext: ReturnType<typeof gsap.matchMedia> | null = null

onMounted(() => {
  const primary = primaryWave.value
  const secondary = secondaryWave.value
  if (!primary || !secondary) return

  motionContext = gsap.matchMedia()
  motionContext.add('(prefers-reduced-motion: no-preference)', () => {
    const waves = [primary, secondary]
    gsap.set(waves, { transformOrigin: '50% 50%' })
    const tween = gsap.to(waves, {
      xPercent: (index: number) => index === 0 ? 5 : -5,
      y: (index: number) => index === 0 ? -0.75 : 0.75,
      rotation: (index: number) => index === 0 ? 1.5 : -1.5,
      duration: 2.4,
      ease: 'sine.inOut',
      repeat: -1,
      yoyo: true,
      stagger: 0.18
    })

    return () => tween.kill()
  })
})

onUnmounted(() => {
  motionContext?.revert()
  motionContext = null
})
</script>

<style scoped>
.account-health-liquid-gauge {
  --health-liquid-color: var(--color-info);

  position: relative;
  display: inline-block;
  width: 1rem;
  height: 1.5rem;
  flex: 0 0 1rem;
  overflow: hidden;
  border: 1px solid var(--color-border-strong);
  border-radius: 0.2rem 0.2rem 0.35rem 0.35rem;
  background-color: var(--color-surface-soft);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-surface) 55%, transparent);
  vertical-align: middle;
}

.account-health-liquid-gauge::before {
  position: absolute;
  z-index: 3;
  top: 0.12rem;
  right: 0.18rem;
  left: 0.18rem;
  height: 1px;
  border-radius: 999px;
  background-color: color-mix(in srgb, var(--color-border-strong) 75%, transparent);
  content: '';
}

.account-health-liquid-gauge--healthy {
  --health-liquid-color: var(--color-success);
}

.account-health-liquid-gauge--degraded {
  --health-liquid-color: var(--color-warning);
}

.account-health-liquid-gauge--open {
  --health-liquid-color: var(--color-danger);
}

.account-health-liquid-gauge--warming,
.account-health-liquid-gauge--half_open {
  --health-liquid-color: var(--color-info);
}

.account-health-liquid-gauge__fill {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  min-height: 0;
  overflow: visible;
  background-color: color-mix(in srgb, var(--health-liquid-color) 78%, transparent);
  transition: background-color var(--motion-base) var(--ease-standard);
}

.account-health-liquid-gauge__wave {
  position: absolute;
  top: -0.17rem;
  left: -45%;
  width: 190%;
  height: 0.38rem;
  border-radius: 50%;
  will-change: transform;
}

.account-health-liquid-gauge__wave--primary {
  background-color: color-mix(in srgb, var(--health-liquid-color) 72%, var(--color-surface));
}

.account-health-liquid-gauge__wave--secondary {
  top: -0.1rem;
  left: -50%;
  background-color: color-mix(in srgb, var(--health-liquid-color) 84%, transparent);
  opacity: 0.72;
}

.account-health-liquid-gauge__highlight {
  position: absolute;
  z-index: 2;
  top: 0.24rem;
  bottom: 0.24rem;
  left: 0.16rem;
  width: 1px;
  border-radius: 999px;
  background-color: color-mix(in srgb, var(--color-surface) 72%, transparent);
  pointer-events: none;
}

@media (prefers-reduced-motion: reduce) {
  .account-health-liquid-gauge__wave {
    will-change: auto;
  }
}
</style>
