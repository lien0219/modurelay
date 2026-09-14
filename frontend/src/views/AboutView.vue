<template>
  <AppLayout :enable-onboarding="false">
    <main data-testid="about-page" class="about-page mx-auto w-full max-w-6xl space-y-6">
      <header class="about-heading">
        <div class="min-w-0">
          <p class="about-eyebrow">{{ t('about.eyebrow') }}</p>
          <h1 class="about-title">{{ t('about.title') }}</h1>
          <p class="about-lede">{{ t('about.description') }}</p>
        </div>
        <span class="about-scope">{{ t('about.scopeLabel') }}</span>
      </header>

      <section class="about-hero" aria-labelledby="about-hero-title">
        <div class="about-hero-copy">
          <div class="about-kicker">
            <Icon name="sparkles" size="sm" aria-hidden="true" />
            <span>{{ t('about.heroKicker') }}</span>
          </div>
          <h2 id="about-hero-title" class="about-hero-title">{{ t('about.heroTitle') }}</h2>
          <p class="about-hero-description">{{ t('about.heroDescription') }}</p>
          <div class="about-hero-points">
            <span v-for="point in heroPoints" :key="point" class="about-point">
              <Icon name="checkCircle" size="sm" aria-hidden="true" />
              {{ t(`about.heroPoints.${point}`) }}
            </span>
          </div>
        </div>

        <div class="about-visual" role="img" :aria-label="t('about.visualAlt')">
          <svg class="about-visual-art" viewBox="0 0 440 300" fill="none" aria-hidden="true">
            <defs>
              <linearGradient id="about-core-gradient" x1="145" y1="78" x2="303" y2="215" gradientUnits="userSpaceOnUse">
                <stop stop-color="var(--color-primary)" />
                <stop offset="1" stop-color="var(--color-accent)" />
              </linearGradient>
              <filter id="about-core-shadow" x="120" y="54" width="200" height="206" filterUnits="userSpaceOnUse">
                <feDropShadow dx="0" dy="16" stdDeviation="16" flood-color="var(--color-primary)" flood-opacity=".18" />
              </filter>
            </defs>

            <ellipse cx="220" cy="150" rx="158" ry="91" stroke="var(--color-primary-border)" stroke-width="1.5" stroke-dasharray="4 9" opacity=".75" />
            <ellipse cx="220" cy="150" rx="118" ry="66" stroke="var(--color-border-strong)" stroke-width="1" opacity=".8" />
            <path d="M74 150h72m148 0h72M220 60v38m0 104v38" stroke="var(--color-border)" stroke-width="1.5" stroke-linecap="round" />

            <path d="M143 104 91 83m206 21 52-21M143 196l-52 21m206-21 52 21" stroke="var(--color-primary-border)" stroke-width="1.5" stroke-linecap="round" />
            <circle cx="91" cy="83" r="7" fill="var(--color-surface)" stroke="var(--color-accent)" stroke-width="3" />
            <circle cx="349" cy="83" r="7" fill="var(--color-surface)" stroke="var(--color-primary)" stroke-width="3" />
            <circle cx="91" cy="217" r="7" fill="var(--color-surface)" stroke="var(--color-primary)" stroke-width="3" />
            <circle cx="349" cy="217" r="7" fill="var(--color-surface)" stroke="var(--color-accent)" stroke-width="3" />

            <g filter="url(#about-core-shadow)">
              <rect x="143" y="79" width="154" height="142" rx="28" fill="var(--color-surface-raised)" stroke="var(--color-primary-border)" stroke-width="1.5" />
              <rect x="163" y="99" width="114" height="102" rx="20" fill="url(#about-core-gradient)" opacity=".96" />
              <path d="M184 165 209 140l18 18 27-30" stroke="var(--color-surface)" stroke-width="5" stroke-linecap="round" stroke-linejoin="round" />
              <circle cx="184" cy="165" r="7" fill="var(--color-surface)" />
              <circle cx="209" cy="140" r="7" fill="var(--color-surface)" />
              <circle cx="227" cy="158" r="7" fill="var(--color-surface)" />
              <circle cx="254" cy="128" r="7" fill="var(--color-surface)" />
              <path d="M220 99v102" stroke="var(--color-surface)" stroke-width="1" opacity=".32" />
              <path d="M163 150h114" stroke="var(--color-surface)" stroke-width="1" opacity=".32" />
            </g>

            <circle cx="220" cy="60" r="5" fill="var(--color-accent)" />
            <circle cx="220" cy="240" r="5" fill="var(--color-primary)" />
            <circle cx="74" cy="150" r="5" fill="var(--color-primary)" />
            <circle cx="366" cy="150" r="5" fill="var(--color-accent)" />
          </svg>
          <div class="about-visual-caption">
            <span class="about-visual-dot"></span>
            <span>{{ t('about.visualCaption') }}</span>
          </div>
        </div>
      </section>

      <section aria-labelledby="about-services-title">
        <div class="about-section-heading">
          <div>
            <p class="about-eyebrow">{{ t('about.servicesEyebrow') }}</p>
            <h2 id="about-services-title" class="about-section-title">{{ t('about.servicesTitle') }}</h2>
          </div>
          <p class="about-section-description">{{ t('about.servicesDescription') }}</p>
        </div>
        <div class="about-services-grid">
          <article v-for="service in services" :key="service.id" class="about-service">
            <span class="about-service-icon"><Icon :name="service.icon" size="md" aria-hidden="true" /></span>
            <h3>{{ t(`about.services.${service.id}.title`) }}</h3>
            <p>{{ t(`about.services.${service.id}.description`) }}</p>
          </article>
        </div>
      </section>

      <section class="about-two-column" aria-labelledby="about-industries-title">
        <div class="about-section-block">
          <p class="about-eyebrow">{{ t('about.industriesEyebrow') }}</p>
          <h2 id="about-industries-title" class="about-section-title">{{ t('about.industriesTitle') }}</h2>
          <p class="about-section-description">{{ t('about.industriesDescription') }}</p>
          <div class="about-chips">
            <span v-for="industry in industries" :key="industry" class="about-chip">{{ t(`about.industries.${industry}`) }}</span>
          </div>
        </div>
        <div class="about-section-block">
          <p class="about-eyebrow">{{ t('about.deliveryEyebrow') }}</p>
          <h2 class="about-section-title">{{ t('about.deliveryTitle') }}</h2>
          <ol class="about-delivery-list">
            <li v-for="(step, index) in deliverySteps" :key="step">
              <span class="about-step-number">{{ String(index + 1).padStart(2, '0') }}</span>
              <span>{{ t(`about.delivery.${step}`) }}</span>
            </li>
          </ol>
        </div>
      </section>

      <section class="about-contact" aria-labelledby="about-contact-title">
        <div class="about-contact-copy">
          <p class="about-eyebrow">{{ t('about.contactEyebrow') }}</p>
          <h2 id="about-contact-title" class="about-contact-title">{{ t('about.contactTitle') }}</h2>
          <p class="about-contact-description">{{ t('about.contactDescription') }}</p>
        </div>
        <div class="about-contact-details">
          <a class="about-contact-item" href="tel:13017739011" :aria-label="t('about.phoneAria')">
            <span class="about-contact-icon"><Icon name="phone" size="md" aria-hidden="true" /></span>
            <span><strong>{{ t('about.phoneLabel') }}</strong><span>13017739011</span></span>
          </a>
          <div class="about-contact-item">
            <span class="about-contact-icon"><Icon name="chat" size="md" aria-hidden="true" /></span>
            <span><strong>{{ t('about.wechatLabel') }}</strong><span>13017739011</span></span>
          </div>
        </div>
      </section>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const heroPoints = ['delivery', 'integration', 'iteration'] as const
const industries = ['ai', 'ecommerce', 'livestream', 'metaverse', 'internet', 'education', 'crossBorder', 'payments'] as const
const deliverySteps = ['consulting', 'solution', 'engineering', 'integration', 'iteration'] as const
const services = [
  { id: 'aiProduct', icon: 'cpu' },
  { id: 'automation', icon: 'bolt' },
  { id: 'productivity', icon: 'trendingUp' },
  { id: 'workflow', icon: 'link' },
  { id: 'distilledResources', icon: 'beaker' },
  { id: 'tokenAssets', icon: 'database' },
  { id: 'saas', icon: 'cube' },
  { id: 'knowledgeBase', icon: 'book' },
  { id: 'customSoftware', icon: 'terminal' },
  { id: 'gameDevelopment', icon: 'sparkles' },
] as const
</script>

<style scoped>
.about-page {
  color: var(--color-text-primary);
}
.about-heading, .about-section-heading, .about-two-column, .about-contact {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1.5rem;
}
.about-eyebrow {
  margin: 0;
  color: var(--color-primary);
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.about-title {
  margin: 0.35rem 0 0;
  font-size: clamp(1.75rem, 3vw, 2.25rem);
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.about-lede, .about-section-description {
  max-width: 42rem;
  margin: 0.75rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.9375rem;
  line-height: 1.65;
}
.about-scope {
  flex-shrink: 0;
  border: 1px solid var(--color-primary-border);
  border-radius: 999px;
  padding: 0.45rem 0.8rem;
  color: var(--color-primary);
  background: var(--color-primary-soft);
  font-size: 0.75rem;
  font-weight: 600;
}
.about-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(300px, 0.95fr);
  gap: 1rem;
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 1.125rem;
  background: var(--glass-bg);
  box-shadow: var(--glass-shadow);
}
.about-hero-copy {
  min-width: 0;
  padding: clamp(1.25rem, 4vw, 2.5rem);
}
.about-kicker {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--color-accent);
  font-size: 0.8125rem;
  font-weight: 700;
}
.about-hero-title {
  max-width: 32rem;
  margin: 1rem 0 0;
  font-size: clamp(1.5rem, 3vw, 2.25rem);
  line-height: 1.25;
  font-weight: 700;
}
.about-hero-description {
  max-width: 36rem;
  margin: 1rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.9375rem;
  line-height: 1.7;
}
.about-hero-points {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem 1rem;
  margin-top: 1.35rem;
}
.about-point {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--color-text-secondary);
  font-size: 0.8125rem;
}
.about-point svg { color: var(--color-success); }
.about-visual {
  position: relative;
  display: flex;
  min-height: 18rem;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}
.about-visual-art {
  position: relative;
  z-index: 1;
  width: min(88%, 27rem);
  height: auto;
  filter: drop-shadow(0 1.2rem 1.75rem color-mix(in srgb, var(--color-primary) 18%, transparent));
}
.about-visual-caption {
  position: absolute;
  right: 1rem;
  bottom: 1rem;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--color-text-muted);
  font-size: 0.7rem;
}
.about-visual-dot { width: 0.4rem; height: 0.4rem; border-radius: 50%; background: var(--color-accent); }
.about-section-title {
  margin: 0.35rem 0 0;
  font-size: 1.25rem;
  line-height: 1.35;
  font-weight: 700;
}
.about-services-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 0.75rem;
  margin-top: 1rem;
}
.about-service, .about-section-block {
  min-width: 0;
  border: 1px solid var(--color-border);
  border-radius: 0.875rem;
  background: var(--color-surface);
  padding: 1rem;
}
.about-service { min-height: 9.75rem; }
.about-service-icon, .about-contact-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--color-primary);
}
.about-service h3 { margin: 0.75rem 0 0; font-size: 0.9375rem; font-weight: 650; }
.about-service p { margin: 0.45rem 0 0; color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.55; }
.about-two-column { align-items: stretch; }
.about-two-column > .about-section-block { flex: 1; }
.about-chips { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 1rem; }
.about-chip { border: 1px solid var(--color-border); border-radius: 999px; background: var(--color-surface-soft); padding: 0.4rem 0.7rem; color: var(--color-text-secondary); font-size: 0.8125rem; }
.about-delivery-list { display: grid; gap: 0.7rem; margin: 1rem 0 0; padding: 0; list-style: none; }
.about-delivery-list li { display: flex; align-items: center; gap: 0.7rem; color: var(--color-text-secondary); font-size: 0.875rem; }
.about-step-number { min-width: 2rem; color: var(--color-primary); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.75rem; font-weight: 700; }
.about-contact { align-items: center; border: 1px solid var(--color-primary-border); border-radius: 1rem; background: var(--color-primary-soft); padding: clamp(1.1rem, 3vw, 1.6rem); }
.about-contact-title { margin: 0.35rem 0 0; font-size: 1.35rem; font-weight: 700; }
.about-contact-description { max-width: 38rem; margin: 0.55rem 0 0; color: var(--color-text-secondary); font-size: 0.875rem; line-height: 1.6; }
.about-contact-details { display: flex; flex-wrap: wrap; gap: 0.7rem 1.25rem; }
.about-contact-item { display: inline-flex; min-height: 2.75rem; align-items: center; gap: 0.65rem; border-radius: 0.65rem; padding: 0.45rem 0.65rem; color: var(--color-text-primary); text-decoration: none; }
.about-contact-item:hover { background: color-mix(in srgb, var(--color-surface) 55%, transparent); }
.about-contact-item:focus-visible { outline: 3px solid var(--color-primary-ring); outline-offset: 2px; }
.about-contact-item > span:last-child { display: grid; gap: 0.1rem; }
.about-contact-item strong { font-size: 0.75rem; font-weight: 700; color: var(--color-text-secondary); }
.about-contact-item > span:last-child > span { font-size: 0.875rem; font-weight: 650; }
@media (max-width: 900px) {
  .about-hero { grid-template-columns: 1fr; }
  .about-visual { min-height: 15rem; border-top: 1px solid var(--color-border-subtle); border-left: 0; }
  .about-services-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .about-two-column, .about-contact { flex-direction: column; align-items: stretch; }
}
@media (max-width: 520px) {
  .about-heading, .about-section-heading { align-items: flex-start; flex-direction: column; gap: 0.75rem; }
  .about-services-grid { grid-template-columns: 1fr; }
  .about-service { min-height: auto; }
  .about-visual { min-height: 13rem; }
  .about-contact-details { flex-direction: column; align-items: flex-start; }
}
@media (prefers-reduced-motion: reduce) {
  .about-contact-item { transition: none; }
}
</style>
