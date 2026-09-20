<template>
  <AppLayout :enable-onboarding="false">
    <main class="toolbox-page mx-auto max-w-7xl space-y-6">
      <template v-if="isHome">
        <header class="toolbox-hero">
          <div class="max-w-3xl">
            <p class="toolbox-eyebrow">ModuRelay / {{ t('nav.tools') }}</p>
            <h1 class="toolbox-title">{{ t('tools.title') }}</h1>
            <p class="toolbox-description">{{ t('tools.description') }}</p>
          </div>
          <label class="toolbox-search">
            <span class="sr-only">{{ t('tools.searchPlaceholder') }}</span>
            <Icon name="search" size="sm" class="toolbox-search__icon" aria-hidden="true" />
            <input v-model="search" class="input h-11 w-full pl-10" type="search" :placeholder="t('tools.searchPlaceholder')" />
          </label>
        </header>

        <div class="flex flex-wrap gap-2" role="tablist" :aria-label="t('tools.allCategories')">
          <button
            v-for="category in categories"
            :key="category"
            type="button"
            class="tool-filter"
            :class="{ 'tool-filter-active': selectedCategory === category }"
            :aria-selected="selectedCategory === category"
            role="tab"
            @click="selectedCategory = category"
          >
            {{ category === 'all' ? t('tools.allCategories') : t(`tools.categories.${category}`) }}
          </button>
        </div>

        <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3" aria-live="polite">
          <RouterLink v-for="tool in filteredTools" :key="tool.id" :to="tool.route" class="tool-card group">
            <span class="tool-card-icon" aria-hidden="true"><Icon :name="tool.icon" size="md" /></span>
            <span class="min-w-0 flex-1">
              <strong class="block text-base font-semibold text-[color:var(--color-text-primary)]">{{ t(tool.titleKey) }}</strong>
              <span class="mt-1 block text-sm leading-5 text-[color:var(--color-text-muted)]">{{ t(tool.descriptionKey) }}</span>
            </span>
            <Icon name="arrowRight" size="sm" class="shrink-0 text-[color:var(--color-text-muted)] transition-[transform,color] duration-200 group-hover:translate-x-0.5 group-hover:text-[color:var(--color-primary)]" aria-hidden="true" />
          </RouterLink>
          <div v-if="filteredTools.length === 0" class="tool-empty-state" role="status">
            <span class="tool-empty-state__icon" aria-hidden="true"><Icon name="search" size="md" /></span>
            <strong>{{ t('tools.noTools') }}</strong>
            <span>{{ t('tools.noToolsHint') }}</span>
          </div>
        </section>
      </template>

        <template v-else-if="selectedTool">
        <div class="tool-workspace-shell">
          <RouterLink to="/tools" class="tool-back-link">
            <Icon name="arrowLeft" size="sm" aria-hidden="true" />
            {{ t('tools.backToTools') }}
          </RouterLink>
          <Suspense>
            <ToolWorkspace :tool="selectedTool.id" />
            <template #fallback>
              <section class="tool-loading" aria-live="polite">{{ t('tools.loading') }}</section>
            </template>
          </Suspense>
        </div>
      </template>

      <template v-else>
        <section class="tool-invalid-route" role="alert">
          <h1>{{ t('tools.title') }}</h1>
          <p>{{ t('tools.noInput') }}</p>
          <RouterLink to="/tools" class="btn btn-primary">{{ t('tools.backToTools') }}</RouterLink>
        </section>
      </template>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { TOOL_REGISTRY, getToolDefinition, type ToolCategory } from '@/features/tools/registry'

const { t } = useI18n()
const route = useRoute()
const search = ref('')
const selectedCategory = ref<'all' | ToolCategory>('all')
const categories: readonly ('all' | ToolCategory)[] = ['all', 'security', 'encoding', 'developer', 'api', 'network', 'text']

const toolId = computed(() => typeof route.params.toolId === 'string' ? route.params.toolId : '')
const selectedTool = computed(() => getToolDefinition(toolId.value))
const ToolWorkspace = defineAsyncComponent(() => selectedTool.value?.lazyComponent() ?? import('@/features/tools/ToolWorkspace.vue'))
const isHome = computed(() => !toolId.value)
const filteredTools = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return TOOL_REGISTRY.filter((tool) => {
    if (selectedCategory.value !== 'all' && tool.category !== selectedCategory.value) return false
    if (!query) return true
    return `${tool.id} ${t(tool.titleKey)} ${t(tool.descriptionKey)}`.toLocaleLowerCase().includes(query)
  })
})
</script>

<style scoped>
.toolbox-hero { padding: 2rem; border: 1px solid var(--color-primary-border); border-radius: 1rem; background: var(--color-primary-soft); }
.toolbox-eyebrow { color: var(--color-primary); font-size: .75rem; font-weight: 700; letter-spacing: .16em; text-transform: uppercase; }
.toolbox-title { margin-top: .5rem; color: var(--color-text-primary); font-size: 2rem; font-weight: 700; letter-spacing: 0; line-height: 1.2; }
.toolbox-description { max-width: 42rem; margin-top: .75rem; color: var(--color-text-secondary); font-size: .9rem; line-height: 1.6; }
.toolbox-search { position: relative; display: block; max-width: 36rem; margin-top: 1.5rem; }
.toolbox-search__icon { position: absolute; top: 50%; left: .75rem; pointer-events: none; color: var(--color-text-muted); transform: translateY(-50%); }
.tool-filter { min-height: 40px; padding: .5rem .875rem; border: 1px solid var(--color-border); border-radius: 9999px; color: var(--color-text-secondary); transition: border-color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), color var(--motion-fast) var(--ease-standard); }
.tool-filter:hover { border-color: var(--color-primary-border); color: var(--color-text-primary); }
.tool-filter:focus-visible, .tool-back-link:focus-visible { outline: 2px solid var(--color-primary-ring); outline-offset: 2px; }
.tool-filter-active { border-color: var(--color-primary); background: var(--color-primary-soft); color: var(--color-primary-active); }
.tool-card { display: flex; min-width: 0; align-items: flex-start; gap: .875rem; padding: 1.125rem; border: 1px solid var(--color-border); border-radius: .875rem; background: var(--color-surface); box-shadow: var(--shadow-xs); transition: border-color var(--motion-base) var(--ease-standard), background-color var(--motion-base) var(--ease-standard), transform var(--motion-base) var(--ease-standard); }
.tool-card:hover { border-color: var(--color-primary-border); background: var(--color-surface-raised); transform: translateY(-1px); }
.tool-card:focus-visible { outline: 2px solid var(--color-primary-ring); outline-offset: 2px; }
.tool-card-icon { display: inline-flex; width: 2.5rem; height: 2.5rem; flex: 0 0 auto; align-items: center; justify-content: center; border-radius: .75rem; background: var(--color-primary-soft); color: var(--color-primary); }
.tool-empty-state, .tool-invalid-route { display: grid; justify-items: center; gap: .5rem; min-height: 180px; padding: 2.5rem 2rem; border: 1px solid var(--color-border); border-radius: 1rem; background: var(--color-surface); color: var(--color-text-muted); text-align: center; }
.tool-empty-state strong { color: var(--color-text-primary); font-size: 1rem; font-weight: 650; }
.tool-empty-state__icon { display: grid; width: 2.75rem; height: 2.75rem; place-items: center; margin-bottom: .25rem; border-radius: .875rem; background: var(--color-primary-soft); color: var(--color-primary); }
.tool-loading { min-height: 260px; display: grid; place-items: center; padding: 2rem; border: 1px solid var(--color-border); border-radius: .875rem; background: var(--color-surface); color: var(--color-text-muted); }
.tool-workspace-shell { display: grid; gap: 1rem; }
.tool-back-link { display: inline-flex; width: fit-content; min-height: 40px; align-items: center; gap: .5rem; color: var(--color-primary); font-size: .875rem; font-weight: 600; }
.tool-invalid-route { display: grid; justify-items: center; gap: .75rem; }
.tool-invalid-route h1 { color: var(--color-text-primary); font-size: 1.5rem; font-weight: 700; }
.tool-invalid-route p { color: var(--color-text-secondary); }
@media (max-width: 640px) { .toolbox-hero { padding: 1.25rem; } }
@media (max-width: 640px) { .toolbox-title { font-size: 1.75rem; } }
@media (prefers-reduced-motion: reduce) { .tool-card, .tool-filter { transition: none; } }
</style>
