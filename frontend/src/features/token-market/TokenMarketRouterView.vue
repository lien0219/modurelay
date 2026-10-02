<template>
  <TokenMarketShell>
    <TokenMarketRouteBoundary><RouterView /></TokenMarketRouteBoundary>
  </TokenMarketShell>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import TokenMarketRouteBoundary from './components/TokenMarketRouteBoundary.vue'
import TokenMarketShell from './components/TokenMarketShell.vue'
</script>
