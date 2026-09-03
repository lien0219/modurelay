<template>
  <AppLayout :enable-onboarding="false">
    <div class="distribution-admin">
      <header class="distribution-admin__hero">
        <div class="distribution-admin__hero-meta">
          <p class="distribution-admin__eyebrow">
            <Icon name="calculator" size="sm" aria-hidden="true" />
            <span>{{ t('admin.distribution.eyebrow') }}</span>
          </p>
          <span class="distribution-admin__status">{{ t('admin.distribution.status') }}</span>
        </div>
        <h1>{{ t('admin.distribution.headline') }}</h1>
        <p>{{ t('admin.distribution.intro') }}</p>
        <ul class="distribution-admin__principles" :aria-label="t('admin.distribution.principlesLabel')">
          <li v-for="principle in principles" :key="principle">
            <Icon name="checkCircle" size="sm" aria-hidden="true" />
            <span>{{ principle }}</span>
          </li>
        </ul>
      </header>

      <section class="distribution-admin__section" aria-labelledby="distribution-model-title">
        <div class="distribution-admin__section-heading">
          <div>
            <p>{{ t('admin.distribution.model.eyebrow') }}</p>
            <h2 id="distribution-model-title">{{ t('admin.distribution.model.title') }}</h2>
          </div>
          <span>{{ t('admin.distribution.model.constraint') }}</span>
        </div>

        <div class="distribution-admin__formula-panel">
          <div class="distribution-admin__formula-grid">
            <article v-for="item in formulaItems" :key="item.symbol">
              <span class="distribution-admin__formula-symbol">{{ item.symbol }}</span>
              <div>
                <h3>{{ item.title }}</h3>
                <p>{{ item.description }}</p>
                <code>{{ item.formula }}</code>
              </div>
            </article>
          </div>
          <p class="distribution-admin__formula-note">
            <Icon name="infoCircle" size="sm" aria-hidden="true" />
            <span>{{ t('admin.distribution.model.note') }}</span>
          </p>
        </div>
      </section>

      <section class="distribution-admin__section" aria-labelledby="distribution-flow-title">
        <div class="distribution-admin__section-heading">
          <div>
            <p>{{ t('admin.distribution.flow.eyebrow') }}</p>
            <h2 id="distribution-flow-title">{{ t('admin.distribution.flow.title') }}</h2>
          </div>
        </div>

        <ol class="distribution-admin__flow">
          <li v-for="(step, index) in flowSteps" :key="step.title">
            <span class="distribution-admin__step-index">{{ String(index + 1).padStart(2, '0') }}</span>
            <div>
              <h3>{{ step.title }}</h3>
              <p>{{ step.description }}</p>
            </div>
          </li>
        </ol>
      </section>

      <section class="distribution-admin__section" aria-labelledby="distribution-architecture-title">
        <div class="distribution-admin__section-heading">
          <div>
            <p>{{ t('admin.distribution.architecture.eyebrow') }}</p>
            <h2 id="distribution-architecture-title">{{ t('admin.distribution.architecture.title') }}</h2>
          </div>
          <span>{{ t('admin.distribution.architecture.scope') }}</span>
        </div>

        <div class="distribution-admin__architecture-grid">
          <article v-for="(layer, index) in architectureLayers" :key="layer.title">
            <header>
              <span>{{ String(index + 1).padStart(2, '0') }}</span>
              <Icon :name="layer.icon" size="sm" aria-hidden="true" />
            </header>
            <h3>{{ layer.title }}</h3>
            <p>{{ layer.description }}</p>
            <dl>
              <div>
                <dt>{{ t('admin.distribution.architecture.coreObjects') }}</dt>
                <dd>{{ layer.objects }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.distribution.architecture.responsibility') }}</dt>
                <dd>{{ layer.responsibility }}</dd>
              </div>
            </dl>
          </article>
        </div>
      </section>

      <section class="distribution-admin__section" aria-labelledby="distribution-boundaries-title">
        <div class="distribution-admin__section-heading">
          <div>
            <p>{{ t('admin.distribution.boundaries.eyebrow') }}</p>
            <h2 id="distribution-boundaries-title">{{ t('admin.distribution.boundaries.title') }}</h2>
          </div>
        </div>

        <ul class="distribution-admin__boundaries">
          <li v-for="boundary in boundaries" :key="boundary.title">
            <span class="distribution-admin__boundary-icon" aria-hidden="true">
              <Icon :name="boundary.icon" size="sm" />
            </span>
            <div>
              <h3>{{ boundary.title }}</h3>
              <p>{{ boundary.description }}</p>
            </div>
          </li>
        </ul>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

type IconName = 'badge' | 'link' | 'calculator' | 'database' | 'shield' | 'users'

const { t } = useI18n()

const principles = computed(() => [
  t('admin.distribution.principles.exclusiveMultiplier'),
  t('admin.distribution.principles.usagePricing'),
  t('admin.distribution.principles.independentLedger'),
])

const formulaItems = computed(() => [
  {
    symbol: 'B',
    title: t('admin.distribution.model.base.title'),
    description: t('admin.distribution.model.base.description'),
    formula: t('admin.distribution.model.base.formula'),
  },
  {
    symbol: 'A',
    title: t('admin.distribution.model.agent.title'),
    description: t('admin.distribution.model.agent.description'),
    formula: t('admin.distribution.model.agent.formula'),
  },
  {
    symbol: 'R',
    title: t('admin.distribution.model.retail.title'),
    description: t('admin.distribution.model.retail.description'),
    formula: t('admin.distribution.model.retail.formula'),
  },
  {
    symbol: 'Δ',
    title: t('admin.distribution.model.spread.title'),
    description: t('admin.distribution.model.spread.description'),
    formula: t('admin.distribution.model.spread.formula'),
  },
])

const flowSteps = computed(() => [
  {
    title: t('admin.distribution.flow.qualification.title'),
    description: t('admin.distribution.flow.qualification.description'),
  },
  {
    title: t('admin.distribution.flow.offer.title'),
    description: t('admin.distribution.flow.offer.description'),
  },
  {
    title: t('admin.distribution.flow.attribution.title'),
    description: t('admin.distribution.flow.attribution.description'),
  },
  {
    title: t('admin.distribution.flow.billing.title'),
    description: t('admin.distribution.flow.billing.description'),
  },
  {
    title: t('admin.distribution.flow.ledger.title'),
    description: t('admin.distribution.flow.ledger.description'),
  },
])

const architectureLayers = computed<Array<{
  icon: IconName
  title: string
  description: string
  objects: string
  responsibility: string
}>>(() => [
  {
    icon: 'badge',
    title: t('admin.distribution.architecture.qualification.title'),
    description: t('admin.distribution.architecture.qualification.description'),
    objects: t('admin.distribution.architecture.qualification.objects'),
    responsibility: t('admin.distribution.architecture.qualification.responsibility'),
  },
  {
    icon: 'calculator',
    title: t('admin.distribution.architecture.offer.title'),
    description: t('admin.distribution.architecture.offer.description'),
    objects: t('admin.distribution.architecture.offer.objects'),
    responsibility: t('admin.distribution.architecture.offer.responsibility'),
  },
  {
    icon: 'link',
    title: t('admin.distribution.architecture.attribution.title'),
    description: t('admin.distribution.architecture.attribution.description'),
    objects: t('admin.distribution.architecture.attribution.objects'),
    responsibility: t('admin.distribution.architecture.attribution.responsibility'),
  },
  {
    icon: 'users',
    title: t('admin.distribution.architecture.pricing.title'),
    description: t('admin.distribution.architecture.pricing.description'),
    objects: t('admin.distribution.architecture.pricing.objects'),
    responsibility: t('admin.distribution.architecture.pricing.responsibility'),
  },
  {
    icon: 'database',
    title: t('admin.distribution.architecture.ledger.title'),
    description: t('admin.distribution.architecture.ledger.description'),
    objects: t('admin.distribution.architecture.ledger.objects'),
    responsibility: t('admin.distribution.architecture.ledger.responsibility'),
  },
  {
    icon: 'shield',
    title: t('admin.distribution.architecture.governance.title'),
    description: t('admin.distribution.architecture.governance.description'),
    objects: t('admin.distribution.architecture.governance.objects'),
    responsibility: t('admin.distribution.architecture.governance.responsibility'),
  },
])

const boundaries = computed<Array<{ icon: IconName; title: string; description: string }>>(() => [
  {
    icon: 'calculator',
    title: t('admin.distribution.boundaries.pricing.title'),
    description: t('admin.distribution.boundaries.pricing.description'),
  },
  {
    icon: 'database',
    title: t('admin.distribution.boundaries.snapshot.title'),
    description: t('admin.distribution.boundaries.snapshot.description'),
  },
  {
    icon: 'shield',
    title: t('admin.distribution.boundaries.risk.title'),
    description: t('admin.distribution.boundaries.risk.description'),
  },
])
</script>

<style scoped>
.distribution-admin {
  display: grid;
  width: 100%;
  min-width: 0;
  gap: 2rem;
  padding-bottom: 2rem;
}

.distribution-admin__hero {
  padding: 1.5rem;
  border: 1px solid var(--color-border);
  border-radius: 1rem;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.distribution-admin__hero-meta,
.distribution-admin__section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.distribution-admin__eyebrow,
.distribution-admin__section-heading p {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin: 0;
  color: var(--color-primary);
  font-size: 0.75rem;
  font-weight: 700;
}

.distribution-admin__status,
.distribution-admin__section-heading > span {
  display: inline-flex;
  min-height: 1.75rem;
  flex: 0 0 auto;
  align-items: center;
  padding: 0.25rem 0.6rem;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  font-size: 0.75rem;
  font-weight: 600;
}

.distribution-admin__hero h1 {
  margin: 0.75rem 0 0;
  color: var(--color-text-primary);
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.3;
  text-wrap: balance;
}

.distribution-admin__hero > p {
  max-width: 52rem;
  margin: 0.75rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.875rem;
  line-height: 1.7;
}

.distribution-admin__principles {
  display: flex;
  flex-wrap: wrap;
  gap: 0.625rem 1.25rem;
  margin: 1.25rem 0 0;
  padding: 1rem 0 0;
  border-top: 1px solid var(--color-border-subtle);
  list-style: none;
}

.distribution-admin__principles li {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--color-text-secondary);
  font-size: 0.8125rem;
  font-weight: 600;
}

.distribution-admin__principles svg {
  color: var(--color-primary);
}

.distribution-admin__section {
  display: grid;
  min-width: 0;
  gap: 1rem;
}

.distribution-admin__section-heading h2 {
  margin: 0.3rem 0 0;
  color: var(--color-text-primary);
  font-size: 1.125rem;
  font-weight: 700;
  line-height: 1.4;
}

.distribution-admin__formula-panel,
.distribution-admin__boundaries {
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 1rem;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.distribution-admin__formula-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.distribution-admin__formula-grid article {
  display: flex;
  min-width: 0;
  gap: 0.75rem;
  padding: 1.25rem;
  border-right: 1px solid var(--color-border-subtle);
}

.distribution-admin__formula-grid article:last-child {
  border-right: 0;
}

.distribution-admin__formula-symbol {
  display: inline-flex;
  width: 2.25rem;
  height: 2.25rem;
  flex: 0 0 2.25rem;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 0.5rem;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.875rem;
  font-weight: 700;
}

.distribution-admin__formula-grid h3,
.distribution-admin__flow h3,
.distribution-admin__architecture-grid h3,
.distribution-admin__boundaries h3 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.45;
}

.distribution-admin__formula-grid p,
.distribution-admin__flow p,
.distribution-admin__architecture-grid > article > p,
.distribution-admin__boundaries p {
  margin: 0.35rem 0 0;
  color: var(--color-text-muted);
  font-size: 0.75rem;
  line-height: 1.55;
}

.distribution-admin__formula-grid code {
  display: block;
  margin-top: 0.65rem;
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.75rem;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.distribution-admin__formula-note {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin: 0;
  padding: 0.875rem 1.25rem;
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  font-size: 0.8125rem;
  line-height: 1.6;
}

.distribution-admin__formula-note svg {
  flex: 0 0 auto;
  margin-top: 0.15rem;
  color: var(--color-primary);
}

.distribution-admin__flow {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  margin: 0;
  padding: 0;
  border-top: 1px solid var(--color-border);
  border-bottom: 1px solid var(--color-border);
  list-style: none;
}

.distribution-admin__flow li {
  display: grid;
  min-width: 0;
  gap: 0.65rem;
  padding: 1.25rem;
  border-right: 1px solid var(--color-border-subtle);
}

.distribution-admin__flow li:last-child {
  border-right: 0;
}

.distribution-admin__step-index {
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.75rem;
  font-weight: 700;
}

.distribution-admin__architecture-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}

.distribution-admin__architecture-grid > article {
  min-width: 0;
  padding: 1.25rem;
  border: 1px solid var(--color-border);
  border-radius: 0.75rem;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.distribution-admin__architecture-grid header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.8rem;
  color: var(--color-primary);
}

.distribution-admin__architecture-grid header span {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.75rem;
  font-weight: 700;
}

.distribution-admin__architecture-grid dl {
  display: grid;
  gap: 0.625rem;
  margin: 1rem 0 0;
  padding-top: 0.875rem;
  border-top: 1px solid var(--color-border-subtle);
}

.distribution-admin__architecture-grid dl div {
  display: grid;
  grid-template-columns: 4.25rem minmax(0, 1fr);
  gap: 0.5rem;
}

.distribution-admin__architecture-grid dt,
.distribution-admin__architecture-grid dd {
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.5;
}

.distribution-admin__architecture-grid dt {
  color: var(--color-text-muted);
}

.distribution-admin__architecture-grid dd {
  color: var(--color-text-secondary);
  overflow-wrap: anywhere;
}

.distribution-admin__boundaries {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
  padding: 0;
  list-style: none;
}

.distribution-admin__boundaries li {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 1.25rem;
  border-right: 1px solid var(--color-border-subtle);
}

.distribution-admin__boundaries li:last-child {
  border-right: 0;
}

.distribution-admin__boundary-icon {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: 0 0 2rem;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-surface-soft);
  color: var(--color-primary);
}

@media (max-width: 1100px) {
  .distribution-admin__formula-grid,
  .distribution-admin__architecture-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .distribution-admin__formula-grid article:nth-child(2) {
    border-right: 0;
  }

  .distribution-admin__formula-grid article:nth-child(-n + 2) {
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .distribution-admin__flow {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .distribution-admin__flow li:nth-child(3) {
    border-right: 0;
  }

  .distribution-admin__flow li:nth-child(-n + 3) {
    border-bottom: 1px solid var(--color-border-subtle);
  }
}

@media (max-width: 720px) {
  .distribution-admin {
    gap: 1.5rem;
  }

  .distribution-admin__hero {
    padding: 1.25rem;
  }

  .distribution-admin__hero-meta,
  .distribution-admin__section-heading {
    align-items: flex-start;
  }

  .distribution-admin__formula-grid,
  .distribution-admin__flow,
  .distribution-admin__architecture-grid,
  .distribution-admin__boundaries {
    grid-template-columns: minmax(0, 1fr);
  }

  .distribution-admin__formula-grid article,
  .distribution-admin__formula-grid article:nth-child(2),
  .distribution-admin__flow li,
  .distribution-admin__flow li:nth-child(3),
  .distribution-admin__boundaries li {
    border-right: 0;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .distribution-admin__formula-grid article:last-child,
  .distribution-admin__flow li:last-child,
  .distribution-admin__boundaries li:last-child {
    border-bottom: 0;
  }
}

@media (max-width: 420px) {
  .distribution-admin__hero-meta,
  .distribution-admin__section-heading {
    flex-direction: column;
  }
}
</style>
