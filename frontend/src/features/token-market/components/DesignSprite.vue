<template><img class="sprite" :src="asset" alt="" aria-hidden="true" /></template>
<script setup lang="ts">
import { computed } from 'vue'
const props=defineProps<{x:number;y:number;width:number;height:number;radius?:number}>()
const productArt = ['/token-market/art/model-core.png', '/token-market/art/input-token.png', '/token-market/art/output-token.png', '/token-market/art/cache-token.png', '/token-market/art/chat-token.png']
const asset = computed(() => {
  if (props.y >= 760) return '/token-market/icons/3d-coin.svg'
  if (props.height > 250) return '/token-market/art/hero-space.png'
  return productArt[Math.min(productArt.length - 1, Math.max(0, Math.floor(props.x / 230)))]
})
</script>
<style scoped>.sprite{display:block;width:100%;height:100%;border-radius:var(--radius,12px);object-fit:cover}</style>
