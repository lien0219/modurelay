<template>
  <AppLayout>
    <main class="plan-store-page">
      <div class="plan-store-shell">
        <section class="plan-store-hero">
          <div class="plan-store-hero__copy">
            <p class="plan-store-eyebrow">MODURELAY / PLAN STORE</p>
            <h1 class="plan-store-title">
              {{ t('planCatalog.storeHeroTitleLead') }}
              <span>{{ t('planCatalog.storeHeroTitleAccent') }}</span>
            </h1>
            <p class="plan-store-description">{{ t('planCatalog.storeHeroDescription') }}</p>

            <div class="plan-store-highlights" aria-label="plan store highlights">
              <div class="plan-store-highlight">
                <span class="plan-store-highlight__icon"><Icon name="grid" size="md" aria-hidden="true" /></span>
                <span><strong>{{ t('planCatalog.highlightPlans') }}</strong><small>{{ t('planCatalog.highlightPlansDesc') }}</small></span>
              </div>
              <div class="plan-store-highlight">
                <span class="plan-store-highlight__icon plan-store-highlight__icon--accent"><Icon name="clock" size="md" aria-hidden="true" /></span>
                <span><strong>{{ t('planCatalog.highlightPeriods') }}</strong><small>{{ t('planCatalog.highlightPeriodsDesc') }}</small></span>
              </div>
              <div class="plan-store-highlight">
                <span class="plan-store-highlight__icon"><Icon name="chart" size="md" aria-hidden="true" /></span>
                <span><strong>{{ t('planCatalog.highlightQuota') }}</strong><small>{{ t('planCatalog.highlightQuotaDesc') }}</small></span>
              </div>
              <div class="plan-store-highlight">
                <span class="plan-store-highlight__icon plan-store-highlight__icon--accent"><Icon name="externalLink" size="md" aria-hidden="true" /></span>
                <span><strong>{{ t('planCatalog.highlightPayment') }}</strong><small>{{ t('planCatalog.highlightPaymentDesc') }}</small></span>
              </div>
            </div>
          </div>

          <div class="plan-store-hero__visual" aria-hidden="true">
            <div class="plan-store-visual-orbit plan-store-visual-orbit--one"></div>
            <div class="plan-store-visual-orbit plan-store-visual-orbit--two"></div>
            <div class="plan-store-stack">
              <span class="plan-store-stack__layer plan-store-stack__layer--back"></span>
              <span class="plan-store-stack__layer plan-store-stack__layer--mid"></span>
              <span class="plan-store-stack__layer plan-store-stack__layer--front">
                <span class="plan-store-stack__mark">AI</span>
              </span>
            </div>
            <span
              v-for="(provider, index) in heroProviders"
              :key="provider"
              class="plan-store-provider"
              :class="'plan-store-provider--' + (index + 1)"
            >
              <span class="plan-store-provider__dot"></span>
              {{ provider }}
            </span>
            <div class="plan-store-visual-caption">
              <strong>{{ t('planCatalog.heroVisualTitle') }}</strong>
              <span>{{ t('planCatalog.heroVisualSubtitle') }}</span>
            </div>
          </div>
        </section>

        <template v-if="loading">
          <section class="plan-store-toolbar plan-store-toolbar--loading" aria-hidden="true">
            <span v-for="i in 4" :key="i" class="plan-store-skeleton-chip"></span>
          </section>
          <div class="plan-store-grid">
            <article v-for="i in 3" :key="i" class="plan-card plan-card--skeleton" aria-hidden="true">
              <span class="plan-card-skeleton plan-card-skeleton--short"></span>
              <span class="plan-card-skeleton plan-card-skeleton--title"></span>
              <span class="plan-card-skeleton plan-card-skeleton--price"></span>
              <span class="plan-card-skeleton plan-card-skeleton--block"></span>
              <span class="plan-card-skeleton plan-card-skeleton--button"></span>
            </article>
          </div>
        </template>

        <section v-else-if="error" class="plan-store-state" role="alert">
          <span class="plan-store-state__icon plan-store-state__icon--error"><Icon name="exclamationCircle" size="lg" aria-hidden="true" /></span>
          <strong>{{ t('planCatalog.loadErrorTitle') }}</strong>
          <p>{{ error }}</p>
          <button type="button" class="plan-store-state__button" @click="loadPlans">
            <Icon name="refresh" size="sm" aria-hidden="true" />
            {{ t('common.retry') }}
          </button>
        </section>

        <section v-else-if="!items.length" class="plan-store-state" role="status">
          <span class="plan-store-state__icon"><Icon name="cube" size="lg" aria-hidden="true" /></span>
          <strong>{{ t('planCatalog.emptyTitle') }}</strong>
          <p>{{ t('planCatalog.empty') }}</p>
        </section>

        <template v-else>
          <section class="plan-store-toolbar">
            <div class="plan-store-filters" role="tablist" :aria-label="t('planCatalog.filterLabel')">
              <button
                v-for="filter in availableFilters"
                :key="filter"
                type="button"
                class="plan-store-filter"
                :class="{ 'is-active': activeFilter === filter }"
                role="tab"
                :aria-selected="activeFilter === filter"
                @click="activeFilter = filter"
              >
                {{ filterLabel(filter) }}
                <span>{{ filterCount(filter) }}</span>
              </button>
            </div>

            <div class="plan-store-summary">
              <span class="plan-store-summary__icon"><Icon name="globe" size="sm" aria-hidden="true" /></span>
              <span>{{ currencySummary }}</span>
              <span class="plan-store-summary__divider" aria-hidden="true"></span>
              <span>{{ t('planCatalog.visiblePlans', { count: filteredItems.length }) }}</span>
            </div>
          </section>

          <div class="plan-store-grid" :class="{ 'plan-store-grid--compact': filteredItems.length <= 2 }">
            <article
              v-for="item in filteredItems"
              :key="item.id"
              class="plan-card"
              :class="[
                'plan-card--' + item.accent,
                { 'plan-card--featured': item.is_featured },
              ]"
            >
              <div class="plan-card__accent" aria-hidden="true"></div>

              <div class="plan-card__header">
                <div class="plan-card__identity">
                  <div class="plan-card__badges">
                    <span v-if="item.is_featured" class="plan-badge plan-badge--featured">
                      <Icon name="sparkles" size="xs" aria-hidden="true" />
                      {{ t('planCatalog.featured') }}
                    </span>
                    <span v-if="item.badge" class="plan-badge">{{ item.badge }}</span>
                  </div>
                  <h2 :title="item.name">{{ item.name }}</h2>
                  <p v-if="item.subtitle">{{ item.subtitle }}</p>
                </div>

                <div class="plan-card__price">
                  <div class="plan-card__price-main">
                    <span class="plan-card__currency">{{ currency(item.currency) }}</span>
                    <strong>{{ item.price }}</strong>
                    <span>{{ item.currency }}</span>
                  </div>
                  <div class="plan-card__price-meta">
                    <span class="plan-period">{{ period(item.billing_period) }}</span>
                    <template v-if="item.original_price">
                      <span class="plan-original">{{ currency(item.currency) }}{{ item.original_price }}</span>
                      <span v-if="discountText(item)" class="plan-discount">{{ discountText(item) }}</span>
                    </template>
                  </div>
                </div>
              </div>

              <div v-if="item.description" class="plan-card__description">
                {{ item.description }}
              </div>

              <div class="plan-card__facts">
                <div v-if="item.group_name" class="plan-fact">
                  <span>{{ t('planCatalog.group') }}</span>
                  <strong :title="item.group_name">{{ item.group_name }}</strong>
                </div>
                <div v-if="item.provider" class="plan-fact">
                  <span>{{ t('planCatalog.provider') }}</span>
                  <strong :title="item.provider">{{ item.provider }}</strong>
                </div>
                <div class="plan-fact">
                  <span>{{ t('planCatalog.rate') }}</span>
                  <strong>×{{ item.rate_multiplier || '1' }}</strong>
                </div>
                <div v-if="quotaPrimary(item)" class="plan-fact">
                  <span>{{ quotaPrimary(item)?.label }}</span>
                  <strong>{{ quotaPrimary(item)?.value }}</strong>
                </div>
                <div v-if="quotaSecondary(item)" class="plan-fact">
                  <span>{{ quotaSecondary(item)?.label }}</span>
                  <strong>{{ quotaSecondary(item)?.value }}</strong>
                </div>
                <div
                  v-if="item.daily_limit_usd == null && item.weekly_limit_usd == null && item.monthly_limit_usd == null"
                  class="plan-fact"
                >
                  <span>{{ t('planCatalog.quota') }}</span>
                  <strong>{{ t('planCatalog.unlimited') }}</strong>
                </div>
              </div>

              <ul class="plan-card__benefits">
                <li v-for="benefit in item.benefits" :key="benefit">
                  <span class="plan-card__benefit-icon"><Icon name="checkCircle" size="sm" aria-hidden="true" /></span>
                  <span>{{ benefit }}</span>
                </li>
                <li v-if="!item.benefits.length" class="plan-card__benefits-empty">
                  <span class="plan-card__benefit-icon"><Icon name="checkCircle" size="sm" aria-hidden="true" /></span>
                  <span>{{ t('planCatalog.defaultBenefit') }}</span>
                </li>
              </ul>

              <a
                :href="item.payment_url"
                target="_blank"
                rel="noopener noreferrer"
                class="plan-card__cta"
              >
                <Icon name="externalLink" size="sm" aria-hidden="true" />
                {{ t('planCatalog.cta') }}
              </a>
            </article>
          </div>

          <section class="plan-store-benefits" aria-labelledby="plan-store-benefits-title">
            <h2 id="plan-store-benefits-title" class="sr-only">{{ t('planCatalog.storeBenefitsTitle') }}</h2>
            <article>
              <span class="plan-store-benefit__icon"><Icon name="document" size="md" aria-hidden="true" /></span>
              <span><strong>{{ t('planCatalog.storeBenefitTransparent') }}</strong><small>{{ t('planCatalog.storeBenefitTransparentDesc') }}</small></span>
            </article>
            <article>
              <span class="plan-store-benefit__icon"><Icon name="refresh" size="md" aria-hidden="true" /></span>
              <span><strong>{{ t('planCatalog.storeBenefitFlexible') }}</strong><small>{{ t('planCatalog.storeBenefitFlexibleDesc') }}</small></span>
            </article>
            <article>
              <span class="plan-store-benefit__icon"><Icon name="shield" size="md" aria-hidden="true" /></span>
              <span><strong>{{ t('planCatalog.storeBenefitQuota') }}</strong><small>{{ t('planCatalog.storeBenefitQuotaDesc') }}</small></span>
            </article>
            <article>
              <span class="plan-store-benefit__icon"><Icon name="link" size="md" aria-hidden="true" /></span>
              <span><strong>{{ t('planCatalog.storeBenefitPayment') }}</strong><small>{{ t('planCatalog.storeBenefitPaymentDesc') }}</small></span>
            </article>
          </section>

          <section class="plan-store-footer">
            <span class="plan-store-footer__icon"><Icon name="lightbulb" size="md" aria-hidden="true" /></span>
            <div>
              <strong>{{ t('planCatalog.storeFooterTitle') }}</strong>
              <p>{{ t('planCatalog.storeFooterDescription') }}</p>
            </div>
          </section>
        </template>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { planCatalogAPI } from '@/api/planCatalog'
import { currencySymbol } from '@/components/payment/currency'
import type { PlanCatalogCurrency, PlanCatalogItem, PlanCatalogPeriod } from '@/types/planCatalog'

type CatalogFilter = 'all' | 'featured' | PlanCatalogPeriod
type QuotaDisplay = { label: string; value: string }

const { t } = useI18n()
const items = ref<PlanCatalogItem[]>([])
const loading = ref(true)
const error = ref('')
const activeFilter = ref<CatalogFilter>('all')

const currency = (value: PlanCatalogCurrency) => currencySymbol(value)
const period = (value: PlanCatalogPeriod) => t('planCatalog.period.' + value)

const periodOrder: readonly PlanCatalogPeriod[] = ['monthly', 'quarterly', 'yearly', 'one_time', 'custom']

const availableFilters = computed<CatalogFilter[]>(() => {
  const filters: CatalogFilter[] = ['all']
  if (items.value.some((item) => item.is_featured)) filters.push('featured')
  for (const value of periodOrder) {
    if (items.value.some((item) => item.billing_period === value)) filters.push(value)
  }
  return filters
})

const filteredItems = computed(() => {
  if (activeFilter.value === 'all') return items.value
  if (activeFilter.value === 'featured') return items.value.filter((item) => item.is_featured)
  return items.value.filter((item) => item.billing_period === activeFilter.value)
})

const heroProviders = computed(() => {
  const names = Array.from(new Set(items.value.map((item) => item.provider.trim()).filter(Boolean))).slice(0, 4)
  return names.length ? names : [t('planCatalog.genericProvider')]
})

const currencySummary = computed(() => {
  const currencies = Array.from(new Set(items.value.map((item) => item.currency)))
  return currencies.length === 1 ? currencies[0] : t('planCatalog.multiCurrency')
})

function filterLabel(filter: CatalogFilter): string {
  if (filter === 'all') return t('planCatalog.filters.all')
  if (filter === 'featured') return t('planCatalog.filters.featured')
  return t('planCatalog.filters.' + filter)
}

function filterCount(filter: CatalogFilter): number {
  if (filter === 'all') return items.value.length
  if (filter === 'featured') return items.value.filter((item) => item.is_featured).length
  return items.value.filter((item) => item.billing_period === filter).length
}

function discountText(item: PlanCatalogItem): string {
  const price = Number(item.price)
  const original = Number(item.original_price)
  if (!Number.isFinite(price) || !Number.isFinite(original) || original <= 0 || price >= original) return ''
  return '-' + Math.round((1 - price / original) * 100) + '%'
}

function quotaEntries(item: PlanCatalogItem): QuotaDisplay[] {
  const entries: QuotaDisplay[] = []
  if (item.daily_limit_usd != null) entries.push({ label: t('planCatalog.daily'), value: '$' + item.daily_limit_usd })
  if (item.weekly_limit_usd != null) entries.push({ label: t('planCatalog.weekly'), value: '$' + item.weekly_limit_usd })
  if (item.monthly_limit_usd != null) entries.push({ label: t('planCatalog.monthly'), value: '$' + item.monthly_limit_usd })
  return entries
}

function quotaPrimary(item: PlanCatalogItem): QuotaDisplay | undefined {
  return quotaEntries(item)[0]
}

function quotaSecondary(item: PlanCatalogItem): QuotaDisplay | undefined {
  return quotaEntries(item)[1]
}

async function loadPlans(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    items.value = (await planCatalogAPI.list()).data ?? []
    if (!availableFilters.value.includes(activeFilter.value)) activeFilter.value = 'all'
  } catch {
    error.value = t('planCatalog.loadError')
  } finally {
    loading.value = false
  }
}

onMounted(loadPlans)
</script>

<style scoped>
.plan-store-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.plan-store-shell {
  width: min(1480px, calc(100% - 40px));
  margin-inline: auto;
  padding: 26px 0 48px;
}

.plan-store-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(420px, .88fr);
  min-height: 320px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.plan-store-hero__copy {
  position: relative;
  z-index: 2;
  min-width: 0;
  padding: 34px 38px;
}

.plan-store-eyebrow {
  margin: 0;
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .72rem;
  font-weight: 750;
  letter-spacing: .1em;
}

.plan-store-title {
  max-width: 720px;
  margin: .85rem 0 0;
  font-size: clamp(2rem, 3.2vw, 3rem);
  font-weight: 720;
  line-height: 1.13;
  letter-spacing: -.045em;
}

.plan-store-title span {
  color: var(--color-primary);
}

.plan-store-description {
  max-width: 670px;
  margin: .8rem 0 0;
  color: var(--color-text-secondary);
  font-size: .9rem;
  line-height: 1.72;
}

.plan-store-highlights {
  display: grid;
  max-width: 760px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 1.5rem;
}

.plan-store-highlight {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
}

.plan-store-highlight__icon {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.plan-store-highlight__icon--accent {
  border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-border));
  background: var(--color-accent-soft);
  color: var(--color-accent);
}

.plan-store-highlight > span:last-child {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.plan-store-highlight strong {
  font-size: .76rem;
  font-weight: 680;
  white-space: nowrap;
}

.plan-store-highlight small {
  overflow: hidden;
  color: var(--color-text-muted);
  font-size: .66rem;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plan-store-hero__visual {
  position: relative;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}

.plan-store-hero__visual::before,
.plan-store-hero__visual::after {
  position: absolute;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  content: '';
  opacity: .5;
}

.plan-store-hero__visual::before {
  width: 360px;
  height: 360px;
  top: -80px;
  right: -40px;
}

.plan-store-hero__visual::after {
  width: 220px;
  height: 220px;
  right: 130px;
  bottom: -110px;
  border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-border));
}

.plan-store-visual-orbit {
  position: absolute;
  top: 50%;
  left: 48%;
  border: 1px dashed var(--color-primary-border);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.plan-store-visual-orbit--one {
  width: 260px;
  height: 150px;
}

.plan-store-visual-orbit--two {
  width: 390px;
  height: 235px;
  border-color: color-mix(in srgb, var(--color-accent) 26%, var(--color-border));
}

.plan-store-stack {
  position: absolute;
  top: 49%;
  left: 48%;
  width: 170px;
  height: 154px;
  transform: translate(-50%, -50%);
}

.plan-store-stack__layer {
  position: absolute;
  left: 50%;
  display: grid;
  width: 132px;
  height: 94px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 20px;
  background: var(--color-surface-raised);
  box-shadow: var(--shadow-md);
  transform: translateX(-50%) rotate(45deg) skew(-7deg, -7deg);
}

.plan-store-stack__layer--back {
  top: 46px;
  opacity: .5;
}

.plan-store-stack__layer--mid {
  top: 24px;
  opacity: .75;
}

.plan-store-stack__layer--front {
  top: 2px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.plan-store-stack__mark {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 1.6rem;
  font-weight: 800;
  transform: skew(7deg, 7deg) rotate(-45deg);
}

.plan-store-provider {
  position: absolute;
  z-index: 2;
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  gap: 7px;
  max-width: 150px;
  padding: 0 10px;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--color-surface);
  background: var(--glass-bg);
  box-shadow: var(--shadow-xs);
  color: var(--color-text-secondary);
  font-size: .7rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
  -webkit-backdrop-filter: blur(14px);
  backdrop-filter: blur(14px);
}

.plan-store-provider__dot {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 50%;
  background: var(--color-primary);
}

.plan-store-provider--1 { top: 23%; left: 8%; }
.plan-store-provider--2 { top: 21%; right: 7%; }
.plan-store-provider--3 { bottom: 20%; left: 12%; }
.plan-store-provider--4 { right: 8%; bottom: 23%; }

.plan-store-provider--2 .plan-store-provider__dot,
.plan-store-provider--3 .plan-store-provider__dot {
  background: var(--color-accent);
}

.plan-store-visual-caption {
  position: absolute;
  right: 18px;
  bottom: 14px;
  display: grid;
  justify-items: end;
  gap: 2px;
}

.plan-store-visual-caption strong {
  color: var(--color-text-secondary);
  font-size: .7rem;
  font-weight: 700;
}

.plan-store-visual-caption span {
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .62rem;
  letter-spacing: .04em;
}

.plan-store-toolbar {
  display: flex;
  min-height: 60px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 18px;
  padding: 10px 12px;
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  background: var(--color-surface);
  background: var(--glass-bg);
  box-shadow: var(--glass-shadow);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.plan-store-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.plan-store-filter {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  gap: 7px;
  padding: 0 13px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 600;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.plan-store-filter span {
  min-width: 18px;
  padding: 2px 5px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .66rem;
  text-align: center;
}

.plan-store-filter:hover {
  border-color: var(--color-primary-border);
  color: var(--color-text-primary);
}

.plan-store-filter.is-active {
  border-color: var(--color-primary);
  background: var(--color-primary);
  color: var(--color-surface);
}

.plan-store-filter.is-active span {
  background: color-mix(in srgb, var(--color-surface) 18%, transparent);
  color: var(--color-surface);
}

.plan-store-filter:focus-visible,
.plan-card__cta:focus-visible,
.plan-store-state__button:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

.plan-store-summary {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
  color: var(--color-text-muted);
  font-size: .72rem;
}

.plan-store-summary__icon {
  display: inline-flex;
  color: var(--color-accent);
}

.plan-store-summary__divider {
  width: 1px;
  height: 18px;
  background: var(--color-border);
}

.plan-store-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: stretch;
  gap: 14px;
  margin-top: 14px;
}

.plan-store-grid--compact {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.plan-card {
  --plan-accent: var(--color-primary);
  --plan-soft: var(--color-primary-soft);
  --plan-border: var(--color-primary-border);
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 470px;
  flex-direction: column;
  overflow: hidden;
  padding: 20px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
  transition:
    border-color var(--motion-base) var(--ease-standard),
    box-shadow var(--motion-base) var(--ease-standard),
    transform var(--motion-base) var(--ease-standard);
}

.plan-card--emerald {
  --plan-accent: var(--color-accent);
  --plan-soft: var(--color-accent-soft);
  --plan-border: color-mix(in srgb, var(--color-accent) 28%, var(--color-border));
}

.plan-card--amber {
  --plan-accent: var(--color-warning);
  --plan-soft: color-mix(in srgb, var(--color-warning) 9%, var(--color-surface));
  --plan-border: color-mix(in srgb, var(--color-warning) 28%, var(--color-border));
}

.plan-card--rose {
  --plan-accent: var(--color-danger);
  --plan-soft: color-mix(in srgb, var(--color-danger) 8%, var(--color-surface));
  --plan-border: color-mix(in srgb, var(--color-danger) 25%, var(--color-border));
}

.plan-card--slate {
  --plan-accent: var(--color-text-secondary);
  --plan-soft: var(--color-surface-soft);
  --plan-border: var(--color-border-strong);
}

.plan-card:hover {
  border-color: var(--plan-border);
  box-shadow: var(--shadow-md);
  transform: translateY(-2px);
}

.plan-card--featured {
  border-color: var(--plan-border);
}

.plan-card__accent {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: var(--plan-accent);
}

.plan-card__header {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.plan-card__identity {
  min-width: 0;
  flex: 1 1 auto;
}

.plan-card__badges {
  display: flex;
  min-height: 24px;
  flex-wrap: wrap;
  gap: 6px;
}

.plan-badge {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  gap: 5px;
  max-width: 100%;
  padding: 0 8px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--plan-soft);
  color: var(--plan-accent);
  font-size: .68rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plan-badge--featured {
  background: color-mix(in srgb, var(--color-warning) 10%, var(--color-surface));
  color: var(--color-warning);
}

.plan-card__identity h2 {
  margin: 10px 0 0;
  overflow-wrap: anywhere;
  color: var(--color-text-primary);
  font-size: 1.08rem;
  font-weight: 700;
  line-height: 1.35;
}

.plan-card__identity p {
  margin: 4px 0 0;
  color: var(--color-text-muted);
  font-size: .76rem;
  line-height: 1.45;
}

.plan-card__price {
  flex: 0 0 auto;
  text-align: right;
}

.plan-card__price-main {
  display: flex;
  align-items: baseline;
  justify-content: flex-end;
  gap: 4px;
  color: var(--plan-accent);
}

.plan-card__price-main strong {
  font-size: 1.75rem;
  font-weight: 760;
  letter-spacing: -.035em;
}

.plan-card__price-main > span:last-child {
  color: var(--color-text-muted);
  font-size: .68rem;
  font-weight: 600;
}

.plan-card__currency {
  font-size: .72rem;
}

.plan-card__price-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
  margin-top: 4px;
}

.plan-period,
.plan-discount {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 7px;
  border-radius: 999px;
  background: var(--plan-soft);
  color: var(--plan-accent);
  font-size: .64rem;
  font-weight: 650;
}

.plan-original {
  align-self: center;
  color: var(--color-text-disabled);
  font-size: .65rem;
  text-decoration: line-through;
}

.plan-discount {
  background: color-mix(in srgb, var(--color-success) 10%, var(--color-surface));
  color: var(--color-success);
}

.plan-card__description {
  margin-top: 14px;
  padding: 10px 11px;
  border: 1px solid var(--plan-border);
  border-radius: 10px;
  background: var(--plan-soft);
  color: var(--color-text-secondary);
  font-size: .75rem;
  line-height: 1.55;
}

.plan-card__facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 11px;
  background: var(--color-bg-subtle);
}

.plan-fact {
  display: flex;
  min-width: 0;
  min-height: 43px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-right: 1px solid var(--color-border-subtle);
  border-bottom: 1px solid var(--color-border-subtle);
}

.plan-fact:nth-child(2n) {
  border-right: 0;
}

.plan-fact:nth-last-child(-n + 2) {
  border-bottom: 0;
}

.plan-fact span {
  color: var(--color-text-muted);
  font-size: .68rem;
}

.plan-fact strong {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: .71rem;
  font-weight: 650;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plan-card__benefits {
  display: grid;
  gap: 7px;
  margin: 14px 0 18px;
  padding: 0;
  list-style: none;
}

.plan-card__benefits li {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  color: var(--color-text-secondary);
  font-size: .73rem;
  line-height: 1.45;
}

.plan-card__benefit-icon {
  display: inline-flex;
  flex: 0 0 auto;
  margin-top: 1px;
  color: var(--plan-accent);
}

.plan-card__benefits-empty {
  color: var(--color-text-muted) !important;
}

.plan-card__cta {
  display: flex;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: auto;
  border: 1px solid var(--plan-accent);
  border-radius: 10px;
  background: var(--plan-accent);
  color: var(--color-surface);
  font-size: .82rem;
  font-weight: 680;
  text-decoration: none;
  transition:
    filter var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.plan-card__cta:hover {
  filter: brightness(.96);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.plan-store-benefits {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-top: 18px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-surface);
}

.plan-store-benefits article {
  display: flex;
  min-width: 0;
  min-height: 88px;
  align-items: center;
  gap: 11px;
  padding: 16px 18px;
  border-right: 1px solid var(--color-border-subtle);
}

.plan-store-benefits article:last-child {
  border-right: 0;
}

.plan-store-benefit__icon {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border-radius: 11px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.plan-store-benefits article:nth-child(2n) .plan-store-benefit__icon {
  background: var(--color-accent-soft);
  color: var(--color-accent);
}

.plan-store-benefits article > span:last-child {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.plan-store-benefits strong {
  font-size: .78rem;
  font-weight: 680;
}

.plan-store-benefits small {
  color: var(--color-text-muted);
  font-size: .68rem;
  line-height: 1.4;
}

.plan-store-footer {
  display: flex;
  min-height: 84px;
  align-items: center;
  gap: 12px;
  margin-top: 14px;
  padding: 16px 20px;
  border: 1px solid var(--color-primary-border);
  border-radius: 14px;
  background: var(--color-primary-soft);
}

.plan-store-footer__icon {
  display: grid;
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  place-items: center;
  border-radius: 11px;
  background: var(--color-surface);
  color: var(--color-primary);
}

.plan-store-footer strong {
  color: var(--color-text-primary);
  font-size: .88rem;
  font-weight: 680;
}

.plan-store-footer p {
  margin: 4px 0 0;
  color: var(--color-text-secondary);
  font-size: .74rem;
  line-height: 1.5;
}

.plan-store-state {
  display: grid;
  width: min(560px, 100%);
  min-height: 330px;
  justify-items: center;
  align-content: center;
  gap: 9px;
  margin: 24px auto 0;
  padding: 32px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  text-align: center;
}

.plan-store-state__icon {
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  border-radius: 14px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.plan-store-state__icon--error {
  background: color-mix(in srgb, var(--color-danger) 9%, var(--color-surface));
  color: var(--color-danger);
}

.plan-store-state strong {
  font-size: .95rem;
  font-weight: 700;
}

.plan-store-state p {
  max-width: 420px;
  margin: 0;
  color: var(--color-text-muted);
  font-size: .78rem;
  line-height: 1.55;
}

.plan-store-state__button {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 7px;
  margin-top: 8px;
  padding: 0 14px;
  border: 1px solid var(--color-primary);
  border-radius: 9px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 650;
}

.plan-store-toolbar--loading {
  justify-content: flex-start;
}

.plan-store-skeleton-chip,
.plan-card-skeleton {
  display: block;
  border-radius: 8px;
  background: var(--color-surface-soft);
  animation: plan-store-pulse 1.5s ease-in-out infinite;
}

.plan-store-skeleton-chip {
  width: 100px;
  height: 34px;
}

.plan-card--skeleton {
  gap: 14px;
}

.plan-card-skeleton--short { width: 84px; height: 24px; }
.plan-card-skeleton--title { width: 58%; height: 24px; }
.plan-card-skeleton--price { width: 42%; height: 38px; }
.plan-card-skeleton--block { width: 100%; height: 170px; margin-top: 10px; }
.plan-card-skeleton--button { width: 100%; height: 42px; margin-top: auto; }

@keyframes plan-store-pulse {
  0%, 100% { opacity: .55; }
  50% { opacity: 1; }
}

@media (max-width: 1240px) {
  .plan-store-hero {
    grid-template-columns: minmax(0, 1fr) minmax(350px, .7fr);
  }

  .plan-store-highlights {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .plan-store-grid,
  .plan-store-grid--compact {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .plan-store-shell {
    width: min(100% - 28px, 820px);
  }

  .plan-store-hero {
    grid-template-columns: 1fr;
  }

  .plan-store-hero__visual {
    min-height: 260px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .plan-store-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .plan-store-summary {
    width: 100%;
  }

  .plan-store-benefits {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .plan-store-benefits article:nth-child(2) {
    border-right: 0;
  }

  .plan-store-benefits article:nth-child(-n + 2) {
    border-bottom: 1px solid var(--color-border-subtle);
  }
}

@media (max-width: 640px) {
  .plan-store-shell {
    width: min(100% - 20px, 560px);
    padding-top: 18px;
  }

  .plan-store-hero {
    border-radius: 14px;
  }

  .plan-store-hero__copy {
    padding: 24px 20px;
  }

  .plan-store-title {
    font-size: 2rem;
  }

  .plan-store-highlights {
    grid-template-columns: 1fr 1fr;
  }

  .plan-store-highlight small {
    white-space: normal;
  }

  .plan-store-hero__visual {
    display: none;
  }

  .plan-store-filter {
    min-height: 40px;
  }

  .plan-store-summary__divider,
  .plan-store-summary span:last-child {
    display: none;
  }

  .plan-store-grid,
  .plan-store-grid--compact {
    grid-template-columns: 1fr;
  }

  .plan-card {
    min-height: 0;
    padding: 18px;
  }

  .plan-card__header {
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .plan-card__price {
    text-align: left;
  }

  .plan-card__price-main,
  .plan-card__price-meta {
    justify-content: flex-start;
  }

  .plan-store-benefits {
    grid-template-columns: 1fr;
  }

  .plan-store-benefits article,
  .plan-store-benefits article:nth-child(2) {
    border-right: 0;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .plan-store-benefits article:last-child {
    border-bottom: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .plan-card,
  .plan-card__cta,
  .plan-store-filter {
    transition-duration: .01ms;
  }

  .plan-card:hover,
  .plan-card__cta:hover {
    transform: none;
  }

  .plan-store-skeleton-chip,
  .plan-card-skeleton {
    animation: none;
  }
}
</style>
