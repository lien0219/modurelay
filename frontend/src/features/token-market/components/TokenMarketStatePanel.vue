<template>
  <section class="state glass-card" :class="`state-${kind}`" role="status">
    <div class="icon">{{ icon }}</div>
    <span class="eyebrow">{{ eyebrow }}</span>
    <h2>{{ title }}</h2>
    <p>{{ description }}</p>
    <div v-if="$slots.actions" class="actions"><slot name="actions" /></div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  kind?: 'empty' | 'error' | 'forbidden'
  title: string
  description: string
}>(), { kind: 'empty' })

const icon = computed(() => props.kind === 'error' ? '!' : props.kind === 'forbidden' ? '◇' : '○')
const eyebrow = computed(() => props.kind === 'error' ? 'MARKET ERROR' : props.kind === 'forbidden' ? 'ACCESS CONTROL' : 'EMPTY STATE')
</script>

<style scoped>
.state{display:grid;place-items:center;min-height:300px;padding:42px;text-align:center}.icon{display:grid;place-items:center;width:58px;height:58px;margin-bottom:16px;border:1px solid rgba(116,145,209,.2);border-radius:18px;color:#bfd0ef;background:linear-gradient(135deg,rgba(90,73,255,.18),rgba(29,174,255,.08));font-size:24px;font-weight:900}.eyebrow{color:#6f84ad;font-size:9px;font-weight:800;letter-spacing:.16em}.state h2{margin:8px 0 10px;font-size:22px}.state p{max-width:540px;margin:0;color:#7d8fac;font-size:11px;line-height:1.8}.actions{display:flex;justify-content:center;gap:8px;margin-top:20px}.state-error .icon{color:#ffadc0;background:linear-gradient(135deg,rgba(255,79,121,.16),rgba(107,67,255,.08))}.state-forbidden .icon{color:#f3c98b;background:linear-gradient(135deg,rgba(255,173,78,.14),rgba(112,76,255,.07))}
</style>
