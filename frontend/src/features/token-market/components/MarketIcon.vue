<template>
  <svg class="market-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <template v-if="name==='search'"><circle cx="11" cy="11" r="6.5" stroke="currentColor" stroke-width="1.8"/><path d="m16 16 4.2 4.2" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></template>
    <template v-else-if="name==='bell'"><path d="M6.7 9a5.3 5.3 0 0 1 10.6 0c0 5.5 2.2 6.3 2.2 6.3H4.5S6.7 14.5 6.7 9Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="M9.5 18.1a2.7 2.7 0 0 0 5 0" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></template>
    <template v-else-if="name==='bag'"><path d="M5.4 8.2h13.2l-1 11H6.4l-1-11Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="M8.8 9V6.6a3.2 3.2 0 0 1 6.4 0V9" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></template>
    <template v-else-if="name==='cup'"><path d="M5 7.5h11v7.2A4.3 4.3 0 0 1 11.7 19H9.3A4.3 4.3 0 0 1 5 14.7V7.5Z" stroke="currentColor" stroke-width="1.6"/><path d="M16 9h1.2a2.3 2.3 0 0 1 0 4.6H16M8 4.5c.7.5.8 1.2.2 2M12 4.5c.7.5.8 1.2.2 2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></template>
    <template v-else-if="name==='game'"><path d="M7.2 9h9.6a4 4 0 0 1 3.8 5.2l-1.1 3.4a2 2 0 0 1-3.3.8l-1.7-1.6h-5l-1.7 1.6a2 2 0 0 1-3.3-.8l-1.1-3.4A4 4 0 0 1 7.2 9Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/><path d="M8 12v3M6.5 13.5h3M16.6 12.8h.01M18.1 14.2h.01" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></template>
    <template v-else-if="name==='ai'"><path d="m12 3 7.5 4.2v9.6L12 21l-7.5-4.2V7.2L12 3Z" stroke="currentColor" stroke-width="1.5"/><path d="m12 7 4 2.3v5L12 16.7l-4-2.4v-5L12 7Z" stroke="currentColor" stroke-width="1.4"/><path d="M12 3v4M19.5 7.2l-3.5 2M4.5 7.2l3.5 2M12 16.7V21" stroke="currentColor" stroke-width="1.3"/></template>
    <template v-else-if="name==='store'"><path d="M4 9.2 5.2 4h13.6L20 9.2a2.7 2.7 0 0 1-4.5 2 2.7 2.7 0 0 1-3.5 0 2.7 2.7 0 0 1-3.5 0A2.7 2.7 0 0 1 4 9.2Z" stroke="currentColor" stroke-width="1.5"/><path d="M6 11.5V20h12v-8.5M9 20v-5h6v5" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></template>
    <template v-else-if="name==='wallet'"><path d="M4 7.3A2.3 2.3 0 0 1 6.3 5h10.2A2.5 2.5 0 0 1 19 7.5V9H8.2A3.2 3.2 0 0 0 5 12.2v.6A3.2 3.2 0 0 0 8.2 16H19v1.5a2.5 2.5 0 0 1-2.5 2.5H6.3A2.3 2.3 0 0 1 4 17.7V7.3Z" stroke="currentColor" stroke-width="1.5"/><path d="M8.2 9H20v7H8.2A3.2 3.2 0 0 1 5 12.8v-.6A3.2 3.2 0 0 1 8.2 9Z" stroke="currentColor" stroke-width="1.5"/><circle cx="16.5" cy="12.5" r="1" fill="currentColor"/></template>
    <template v-else-if="name==='order'"><path d="M7 3.8h10v16.4H7z" stroke="currentColor" stroke-width="1.5"/><path d="M9.5 8h5M9.5 11.5h5M9.5 15h3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></template>
    <template v-else-if="name==='heart'"><path d="M12 20s-7-4.2-8.7-9A4.7 4.7 0 0 1 12 7.5 4.7 4.7 0 0 1 20.7 11C19 15.8 12 20 12 20Z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></template>
    <template v-else-if="name==='home'"><path d="m4 10 8-6.5 8 6.5v9.5h-6v-6h-4v6H4V10Z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></template>
    <template v-else-if="name==='message'"><path d="M4 5.5h16v11H9l-4.5 3v-3H4v-11Z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/><path d="M8 10h8M8 13h5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></template>
    <template v-else-if="name==='arrow'"><path d="M5 12h13M14 7l5 5-5 5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></template>
    <template v-else><circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.5"/></template>
  </svg>
</template>
<script setup lang="ts">
defineProps<{ name: string }>()
</script>
<style scoped>.market-icon{width:1em;height:1em;display:block}</style>
