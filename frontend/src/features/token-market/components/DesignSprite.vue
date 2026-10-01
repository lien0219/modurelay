<template><span class="sprite" :style="style" aria-hidden="true" /></template>
<script setup lang="ts">
import { computed } from 'vue'
const props=defineProps<{x:number;y:number;width:number;height:number;radius?:number}>()
const SOURCE_W=1672, SOURCE_H=941
const style=computed(()=>({
  '--sx':`${SOURCE_W/props.width*100}%`,
  '--sy':`${SOURCE_H/props.height*100}%`,
  '--px':`${props.x/(SOURCE_W-props.width)*100}%`,
  '--py':`${props.y/(SOURCE_H-props.height)*100}%`,
  '--radius':`${props.radius??12}px`,
}))
</script>
<style scoped>.sprite{display:block;width:100%;height:100%;border-radius:var(--radius);background-image:url('/token-market/market-dashboard.webp');background-repeat:no-repeat;background-size:var(--sx) var(--sy);background-position:var(--px) var(--py)}</style>
