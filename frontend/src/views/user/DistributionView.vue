<template>
  <AppLayout>
    <main class="distribution-page">
      <div class="distribution-page__shell">
        <DistributionPreview
          :eyebrow="t('distribution.eyebrow')"
          :title="t('distribution.headline')"
          :description="t('distribution.intro')"
          :illustration-label="t('distribution.illustrationLabel')"
          :status-label="t('distribution.status')"
          :items="previewItems"
        >
          <template #action>
            <button type="button" class="distribution-action distribution-action--primary" @click="requestAccess">
              <Icon name="userPlus" size="sm" aria-hidden="true" />
              <span>{{ t('distribution.cta') }}</span>
            </button>
            <button type="button" class="distribution-action distribution-action--secondary" @click="scrollToDetails">
              <span>{{ t('distribution.learnMore') }}</span>
              <Icon name="chevronRight" size="sm" aria-hidden="true" />
            </button>
          </template>
        </DistributionPreview>

        <section id="distribution-details" class="distribution-section distribution-mechanism">
          <header class="distribution-section__header">
            <div>
              <p class="distribution-section__eyebrow">{{ t('distribution.mechanismEyebrow') }}</p>
              <h2>{{ t('distribution.mechanismTitle') }}</h2>
            </div>
            <p>{{ t('distribution.mechanismDescription') }}</p>
          </header>

          <div class="distribution-mechanism__grid">
            <article
              v-for="(step, index) in processItems"
              :key="step.title"
              class="distribution-step"
            >
              <div class="distribution-step__topline">
                <span class="distribution-step__number">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="distribution-step__icon" aria-hidden="true">
                  <Icon :name="step.icon" size="md" />
                </span>
              </div>
              <h3>{{ step.title }}</h3>
              <p>{{ step.description }}</p>
              <span v-if="index < processItems.length - 1" class="distribution-step__connector" aria-hidden="true">
                <Icon name="chevronRight" size="sm" />
              </span>
            </article>
          </div>
        </section>

        <section class="distribution-section distribution-benefits">
          <header class="distribution-section__header">
            <div>
              <p class="distribution-section__eyebrow">{{ t('distribution.benefitsEyebrow') }}</p>
              <h2>{{ t('distribution.benefitsTitle') }}</h2>
            </div>
            <p>{{ t('distribution.benefitsDescription') }}</p>
          </header>

          <div class="distribution-benefits__grid">
            <article v-for="benefit in benefitItems" :key="benefit.title" class="distribution-benefit">
              <span class="distribution-benefit__icon" aria-hidden="true">
                <Icon :name="benefit.icon" size="md" />
              </span>
              <div>
                <h3>{{ benefit.title }}</h3>
                <p>{{ benefit.description }}</p>
              </div>
            </article>
          </div>
        </section>

        <section class="distribution-callout">
          <div class="distribution-callout__glow" aria-hidden="true"></div>
          <span class="distribution-callout__icon" aria-hidden="true">
            <Icon name="sparkles" size="lg" />
          </span>
          <div class="distribution-callout__copy">
            <p class="distribution-section__eyebrow">{{ t('distribution.ctaEyebrow') }}</p>
            <h2>{{ t('distribution.ctaTitle') }}</h2>
            <p>{{ t('distribution.ctaDescription') }}</p>
          </div>
          <button type="button" class="distribution-action distribution-action--primary" @click="requestAccess">
            <Icon name="userPlus" size="sm" aria-hidden="true" />
            <span>{{ t('distribution.cta') }}</span>
          </button>
        </section>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DistributionPreview from '@/features/distribution/DistributionPreview.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const previewItems = computed(() => [
  {
    title: t('distribution.features.agentMultiplier.title'),
    description: t('distribution.features.agentMultiplier.description'),
  },
  {
    title: t('distribution.features.retailMultiplier.title'),
    description: t('distribution.features.retailMultiplier.description'),
  },
  {
    title: t('distribution.features.spread.title'),
    description: t('distribution.features.spread.description'),
  },
])

const processItems = computed(() => [
  {
    icon: 'badge' as const,
    title: t('distribution.steps.obtain.title'),
    description: t('distribution.steps.obtain.description'),
  },
  {
    icon: 'users' as const,
    title: t('distribution.steps.configure.title'),
    description: t('distribution.steps.configure.description'),
  },
  {
    icon: 'trendingUp' as const,
    title: t('distribution.steps.grow.title'),
    description: t('distribution.steps.grow.description'),
  },
])

const benefitItems = computed(() => [
  {
    icon: 'calculator' as const,
    title: t('distribution.benefits.transparent.title'),
    description: t('distribution.benefits.transparent.description'),
  },
  {
    icon: 'trendingUp' as const,
    title: t('distribution.benefits.flexible.title'),
    description: t('distribution.benefits.flexible.description'),
  },
  {
    icon: 'link' as const,
    title: t('distribution.benefits.longTerm.title'),
    description: t('distribution.benefits.longTerm.description'),
  },
  {
    icon: 'users' as const,
    title: t('distribution.benefits.support.title'),
    description: t('distribution.benefits.support.description'),
  },
])

function requestAccess(): void {
  appStore.showInfo(t('distribution.permissionNotice'))
}

function scrollToDetails(): void {
  document.getElementById('distribution-details')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<style scoped>
.distribution-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.distribution-page__shell {
  width: min(1480px, calc(100% - 40px));
  margin-inline: auto;
  padding: 26px 0 48px;
}

.distribution-action {
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 15px;
  border-radius: 10px;
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 650;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.distribution-action--primary {
  border: 1px solid var(--color-primary);
  background: var(--color-primary);
  color: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.distribution-action--primary:hover {
  background: var(--color-primary-hover);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.distribution-action--secondary {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-secondary);
}

.distribution-action--secondary:hover {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.distribution-section {
  margin-top: 18px;
  padding: 24px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.distribution-section__header {
  display: grid;
  grid-template-columns: minmax(0, .9fr) minmax(320px, 1.1fr);
  align-items: end;
  gap: 28px;
}

.distribution-section__eyebrow {
  margin: 0;
  color: var(--color-primary);
  font-size: .7rem;
  font-weight: 750;
  letter-spacing: .08em;
  text-transform: uppercase;
}

.distribution-section__header h2,
.distribution-callout h2 {
  margin: .5rem 0 0;
  color: var(--color-text-primary);
  font-size: clamp(1.35rem, 2vw, 1.85rem);
  font-weight: 710;
  line-height: 1.25;
  letter-spacing: -.03em;
}

.distribution-section__header > p,
.distribution-callout__copy > p:last-child {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: .84rem;
  line-height: 1.68;
}

.distribution-mechanism__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 20px;
}

.distribution-step {
  position: relative;
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-surface-soft);
}

.distribution-step__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.distribution-step__number {
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .67rem;
  font-weight: 800;
  letter-spacing: .08em;
}

.distribution-step__icon,
.distribution-benefit__icon,
.distribution-callout__icon {
  display: grid;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.distribution-step__icon {
  width: 38px;
  height: 38px;
  border-radius: 11px;
}

.distribution-step h3,
.distribution-benefit h3 {
  margin: 1rem 0 0;
  color: var(--color-text-primary);
  font-size: .9rem;
  font-weight: 680;
}

.distribution-step p,
.distribution-benefit p {
  margin: .45rem 0 0;
  color: var(--color-text-muted);
  font-size: .75rem;
  line-height: 1.58;
}

.distribution-step__connector {
  position: absolute;
  top: 50%;
  right: -19px;
  z-index: 2;
  display: grid;
  width: 26px;
  height: 26px;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 50%;
  background: var(--color-surface);
  color: var(--color-primary);
  transform: translateY(-50%);
}

.distribution-benefits__grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 20px;
}

.distribution-benefit {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 11px;
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--color-border);
  border-radius: 13px;
  background: var(--color-surface);
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.distribution-benefit:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--shadow-sm);
  transform: translateY(-2px);
}

.distribution-benefit__icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
}

.distribution-benefit h3 {
  margin-top: 1px;
}

.distribution-callout {
  position: relative;
  display: grid;
  grid-template-columns: 50px minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  margin-top: 18px;
  overflow: hidden;
  padding: 22px 24px;
  border: 1px solid var(--color-primary-border);
  border-radius: 16px;
  background:
    linear-gradient(
      110deg,
      color-mix(in srgb, var(--color-primary-soft) 72%, var(--color-surface)),
      color-mix(in srgb, var(--color-accent-soft) 36%, var(--color-surface))
    );
  box-shadow: var(--shadow-xs);
}

.distribution-callout__glow {
  position: absolute;
  top: -110px;
  right: 13%;
  width: 280px;
  height: 280px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
  filter: blur(70px);
  pointer-events: none;
}

.distribution-callout__icon,
.distribution-callout__copy,
.distribution-callout .distribution-action {
  position: relative;
  z-index: 1;
}

.distribution-callout__icon {
  width: 50px;
  height: 50px;
  border-radius: 14px;
  background: var(--color-surface);
}

.distribution-callout__copy {
  min-width: 0;
}

.distribution-callout h2 {
  font-size: 1.1rem;
}

.distribution-callout__copy > p:last-child {
  margin-top: .4rem;
  max-width: 760px;
  font-size: .75rem;
}

.distribution-action:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

@media (max-width: 1080px) {
  .distribution-benefits__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .distribution-page__shell {
    width: min(100% - 28px, 820px);
  }

  .distribution-section__header {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .distribution-mechanism__grid {
    grid-template-columns: 1fr;
  }

  .distribution-step__connector {
    top: auto;
    right: 50%;
    bottom: -19px;
    transform: translateX(50%) rotate(90deg);
  }

  .distribution-callout {
    grid-template-columns: 50px minmax(0, 1fr);
  }

  .distribution-callout .distribution-action {
    grid-column: 1 / -1;
    justify-self: start;
  }
}

@media (max-width: 640px) {
  .distribution-page__shell {
    width: min(100% - 20px, 560px);
    padding-top: 18px;
  }

  .distribution-section {
    padding: 18px;
    border-radius: 14px;
  }

  .distribution-benefits__grid {
    grid-template-columns: 1fr;
  }

  .distribution-callout {
    grid-template-columns: 1fr;
    justify-items: start;
    padding: 18px;
  }

  .distribution-callout__icon {
    width: 44px;
    height: 44px;
  }

  .distribution-callout .distribution-action {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .distribution-action,
  .distribution-benefit {
    transition-duration: .01ms;
  }

  .distribution-action:hover,
  .distribution-benefit:hover {
    transform: none;
  }
}
</style>
