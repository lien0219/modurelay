<template>
  <AppLayout :enable-onboarding="false">
    <main data-testid="about-page" class="about-page">
      <div class="about-ambient about-ambient-one" aria-hidden="true"></div>
      <div class="about-ambient about-ambient-two" aria-hidden="true"></div>

      <div class="about-shell">
        <header class="about-heading">
          <div class="about-heading-copy">
            <div class="about-eyebrow-row">
              <span class="about-eyebrow-dot" aria-hidden="true"></span>
              <p class="about-eyebrow">{{ t('about.eyebrow') }}</p>
            </div>
            <h1 class="about-title">{{ t('about.title') }}</h1>
            <p class="about-lede">{{ t('about.description') }}</p>
          </div>
          <span class="about-scope">
            <span class="about-scope-dot" aria-hidden="true"></span>
            {{ t('about.scopeLabel') }}
          </span>
        </header>

        <section class="about-hero" aria-labelledby="about-hero-title">
          <div class="about-hero-copy">
            <div class="about-kicker">
              <Icon name="sparkles" size="sm" aria-hidden="true" />
              <span>{{ t('about.heroKicker') }}</span>
            </div>

            <h2 id="about-hero-title" class="about-hero-title">
              {{ t('about.heroTitle') }}
            </h2>

            <p class="about-hero-description">
              {{ t('about.heroDescription') }}
            </p>

            <div class="about-hero-points">
              <span v-for="point in heroPoints" :key="point" class="about-point">
                <span class="about-point-icon">
                  <Icon name="checkCircle" size="sm" aria-hidden="true" />
                </span>
                {{ t('about.heroPoints.' + point) }}
              </span>
            </div>
          </div>

          <div class="about-visual" role="img" :aria-label="t('about.visualAlt')">
            <div class="about-visual-grid" aria-hidden="true"></div>
            <div class="about-orbit about-orbit-one" aria-hidden="true"></div>
            <div class="about-orbit about-orbit-two" aria-hidden="true"></div>
            <div class="about-orbit about-orbit-three" aria-hidden="true"></div>

            <div class="about-signal about-signal-model">
              <span class="about-signal-icon"><Icon name="cpu" size="sm" /></span>
              <span>{{ t('about.services.aiProduct.title') }}</span>
            </div>
            <div class="about-signal about-signal-workflow">
              <span class="about-signal-icon"><Icon name="link" size="sm" /></span>
              <span>{{ t('about.services.workflow.title') }}</span>
            </div>
            <div class="about-signal about-signal-token">
              <span class="about-signal-icon"><Icon name="database" size="sm" /></span>
              <span>{{ t('about.services.tokenAssets.title') }}</span>
            </div>
            <div class="about-signal about-signal-software">
              <span class="about-signal-icon"><Icon name="terminal" size="sm" /></span>
              <span>{{ t('about.services.customSoftware.title') }}</span>
            </div>

            <div class="about-core">
              <div class="about-core-halo"></div>
              <div class="about-core-icon">
                <Icon name="sparkles" size="lg" aria-hidden="true" />
              </div>
              <strong>AI × Engineering</strong>
              <span>Operations</span>
            </div>

            <div class="about-pulse about-pulse-a" aria-hidden="true"></div>
            <div class="about-pulse about-pulse-b" aria-hidden="true"></div>
            <div class="about-pulse about-pulse-c" aria-hidden="true"></div>
            <div class="about-pulse about-pulse-d" aria-hidden="true"></div>

            <div class="about-visual-caption">
              <span class="about-visual-dot"></span>
              <span>{{ t('about.visualCaption') }}</span>
            </div>
          </div>
        </section>

        <section class="about-capabilities" aria-labelledby="about-services-title">
          <div class="about-section-heading">
            <div>
              <p class="about-eyebrow">{{ t('about.servicesEyebrow') }}</p>
              <h2 id="about-services-title" class="about-section-title">
                {{ t('about.servicesTitle') }}
              </h2>
            </div>
            <p class="about-section-description">{{ t('about.servicesDescription') }}</p>
          </div>

          <div class="about-services-grid">
            <article
              v-for="(service, index) in services"
              :key="service.id"
              class="about-service"
              :class="{ 'about-service-featured': index < 2 }"
            >
              <div class="about-service-topline">
                <span class="about-service-index">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="about-service-icon">
                  <Icon :name="service.icon" size="md" aria-hidden="true" />
                </span>
              </div>
              <div>
                <h3>{{ t('about.services.' + service.id + '.title') }}</h3>
                <p>{{ t('about.services.' + service.id + '.description') }}</p>
              </div>
              <div class="about-service-accent" aria-hidden="true"></div>
            </article>
          </div>
        </section>

        <section class="about-operating-model" aria-labelledby="about-industries-title">
          <div class="about-industries-card">
            <div class="about-section-card-heading">
              <div>
                <p class="about-eyebrow">{{ t('about.industriesEyebrow') }}</p>
                <h2 id="about-industries-title" class="about-section-title">
                  {{ t('about.industriesTitle') }}
                </h2>
              </div>
              <span class="about-section-chip">01</span>
            </div>

            <p class="about-section-description about-section-description-wide">
              {{ t('about.industriesDescription') }}
            </p>

            <div class="about-chips">
              <span v-for="industry in industries" :key="industry" class="about-chip">
                <span class="about-chip-dot" aria-hidden="true"></span>
                {{ t('about.industries.' + industry) }}
              </span>
            </div>
          </div>

          <div class="about-delivery-card">
            <div class="about-section-card-heading">
              <div>
                <p class="about-eyebrow">{{ t('about.deliveryEyebrow') }}</p>
                <h2 class="about-section-title">{{ t('about.deliveryTitle') }}</h2>
              </div>
              <span class="about-section-chip">02</span>
            </div>

            <ol class="about-delivery-list">
              <li v-for="(step, index) in deliverySteps" :key="step">
                <span class="about-step-number">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="about-step-line" aria-hidden="true"></span>
                <span class="about-step-copy">{{ t('about.delivery.' + step) }}</span>
              </li>
            </ol>
          </div>
        </section>

        <section class="about-contact" aria-labelledby="about-contact-title">
          <div class="about-contact-glow" aria-hidden="true"></div>

          <div class="about-contact-copy">
            <p class="about-eyebrow about-contact-eyebrow">{{ t('about.contactEyebrow') }}</p>
            <h2 id="about-contact-title" class="about-contact-title">
              {{ t('about.contactTitle') }}
            </h2>
            <p class="about-contact-description">
              {{ t('about.contactDescription') }}
            </p>
          </div>

          <div class="about-contact-details">
            <a class="about-contact-item" href="tel:13017739011" :aria-label="t('about.phoneAria')">
              <span class="about-contact-icon">
                <Icon name="phone" size="md" aria-hidden="true" />
              </span>
              <span class="about-contact-value">
                <strong>{{ t('about.phoneLabel') }}</strong>
                <span>13017739011</span>
              </span>
            </a>

            <div class="about-contact-item">
              <span class="about-contact-icon">
                <Icon name="chat" size="md" aria-hidden="true" />
              </span>
              <span class="about-contact-value">
                <strong>{{ t('about.wechatLabel') }}</strong>
                <span>13017739011</span>
              </span>
            </div>
          </div>
        </section>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const heroPoints = ['delivery', 'integration', 'iteration'] as const
const industries = [
  'ai',
  'ecommerce',
  'livestream',
  'metaverse',
  'internet',
  'education',
  'crossBorder',
  'payments',
] as const
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
  --about-panel: color-mix(in srgb, var(--color-surface) 88%, transparent);
  --about-panel-soft: color-mix(in srgb, var(--color-surface-raised) 76%, transparent);
  --about-border: color-mix(in srgb, var(--color-border) 86%, transparent);
  --about-border-strong: color-mix(in srgb, var(--color-border-strong) 78%, transparent);
  --about-grid: color-mix(in srgb, var(--color-text-primary) 6%, transparent);
  position: relative;
  min-height: calc(100dvh - 4rem);
  overflow: hidden;
  color: var(--color-text-primary);
}

.about-page::before {
  position: absolute;
  inset: 0;
  z-index: 0;
  background-image:
    linear-gradient(var(--about-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--about-grid) 1px, transparent 1px);
  background-size: 44px 44px;
  content: '';
  opacity: 0.5;
  -webkit-mask-image: linear-gradient(#000 0%, rgba(0, 0, 0, 0.8) 34%, transparent 86%);
  mask-image: linear-gradient(#000 0%, rgba(0, 0, 0, 0.8) 34%, transparent 86%);
  pointer-events: none;
}

.about-shell {
  position: relative;
  z-index: 1;
  width: min(1360px, calc(100% - 40px));
  margin-inline: auto;
  padding: clamp(28px, 4vw, 54px) 0 56px;
}

.about-ambient {
  position: absolute;
  z-index: 0;
  width: 420px;
  height: 420px;
  border-radius: 999px;
  filter: blur(100px);
  opacity: 0.13;
  pointer-events: none;
}

.about-ambient-one {
  top: -220px;
  left: 12%;
  background: #6366f1;
}

.about-ambient-two {
  top: 520px;
  right: -180px;
  background: #14b8a6;
}

.about-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 2rem;
}

.about-heading-copy {
  min-width: 0;
}

.about-eyebrow-row {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.about-eyebrow-dot,
.about-scope-dot,
.about-chip-dot,
.about-visual-dot {
  display: inline-block;
  flex: 0 0 auto;
  border-radius: 999px;
}

.about-eyebrow-dot {
  width: 7px;
  height: 7px;
  background: var(--color-accent);
  box-shadow: 0 0 0 5px color-mix(in srgb, var(--color-accent) 12%, transparent);
}

.about-eyebrow {
  margin: 0;
  color: var(--color-primary);
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.09em;
  text-transform: uppercase;
}

.about-title {
  margin: 0.55rem 0 0;
  font-size: clamp(2.25rem, 4vw, 3.5rem);
  line-height: 1.02;
  font-weight: 820;
  letter-spacing: -0.055em;
}

.about-lede {
  max-width: 52rem;
  margin: 1rem 0 0;
  color: var(--color-text-secondary);
  font-size: clamp(0.95rem, 1.1vw, 1.08rem);
  line-height: 1.72;
}

.about-scope {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.6rem;
  min-height: 38px;
  padding: 0 0.9rem;
  border: 1px solid color-mix(in srgb, var(--color-primary) 26%, var(--about-border));
  border-radius: 999px;
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 58%, transparent);
  box-shadow: inset 0 1px 0 var(--glass-highlight);
  font-size: 0.76rem;
  font-weight: 700;
  -webkit-backdrop-filter: blur(14px);
  backdrop-filter: blur(14px);
}

.about-scope-dot {
  width: 7px;
  height: 7px;
  background: #22c55e;
  box-shadow: 0 0 0 5px color-mix(in srgb, #22c55e 11%, transparent);
}

.about-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(420px, 0.92fr);
  min-height: 500px;
  margin-top: 34px;
  overflow: hidden;
  border: 1px solid var(--about-border);
  border-radius: 26px;
  background: var(--about-panel);
  box-shadow:
    0 30px 72px color-mix(in srgb, #0f172a 10%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(22px) saturate(130%);
  backdrop-filter: blur(22px) saturate(130%);
}

.about-hero-copy {
  position: relative;
  z-index: 2;
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  padding: clamp(34px, 5vw, 70px);
}

.about-kicker {
  display: inline-flex;
  width: fit-content;
  min-height: 34px;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.8rem;
  border: 1px solid color-mix(in srgb, var(--color-accent) 30%, var(--about-border));
  border-radius: 999px;
  color: var(--color-accent);
  background: color-mix(in srgb, var(--color-accent) 7%, transparent);
  font-size: 0.76rem;
  font-weight: 750;
}

.about-hero-title {
  max-width: 690px;
  margin: 1.35rem 0 0;
  font-size: clamp(2rem, 3.7vw, 3.55rem);
  line-height: 1.1;
  font-weight: 820;
  letter-spacing: -0.045em;
  text-wrap: balance;
}

.about-hero-title::after {
  display: block;
  width: 86px;
  height: 4px;
  margin-top: 24px;
  border-radius: 999px;
  background: linear-gradient(90deg, #5b5ce8, #3b82f6, #14b8a6);
  content: '';
}

.about-hero-description {
  max-width: 650px;
  margin: 1.35rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.96rem;
  line-height: 1.78;
}

.about-hero-points {
  display: flex;
  flex-wrap: wrap;
  gap: 0.7rem;
  margin-top: 1.55rem;
}

.about-point {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 0.55rem;
  padding: 0 0.82rem;
  border: 1px solid var(--about-border);
  border-radius: 11px;
  color: var(--color-text-secondary);
  background: color-mix(in srgb, var(--about-panel-soft) 86%, transparent);
  box-shadow: inset 0 1px 0 var(--glass-highlight);
  font-size: 0.78rem;
  font-weight: 600;
}

.about-point-icon {
  display: inline-flex;
  color: #16a34a;
}

.about-visual {
  position: relative;
  min-height: 500px;
  overflow: hidden;
  border-left: 1px solid var(--about-border);
  background:
    radial-gradient(circle at 50% 48%, color-mix(in srgb, #4f46e5 16%, transparent), transparent 24%),
    radial-gradient(circle at 66% 68%, color-mix(in srgb, #14b8a6 12%, transparent), transparent 26%),
    color-mix(in srgb, var(--color-surface-soft) 82%, transparent);
}

.about-visual-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(var(--about-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--about-grid) 1px, transparent 1px);
  background-size: 34px 34px;
  opacity: 0.65;
  -webkit-mask-image: radial-gradient(circle at 50% 50%, #000 20%, transparent 78%);
  mask-image: radial-gradient(circle at 50% 50%, #000 20%, transparent 78%);
}

.about-orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  border: 1px solid color-mix(in srgb, var(--color-primary) 26%, transparent);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.about-orbit-one {
  width: 180px;
  height: 180px;
}

.about-orbit-two {
  width: 300px;
  height: 300px;
  border-style: dashed;
  animation: about-orbit-spin 20s linear infinite;
}

.about-orbit-three {
  width: 410px;
  height: 410px;
  border-color: color-mix(in srgb, var(--color-accent) 24%, transparent);
  border-style: dashed;
  animation: about-orbit-spin-reverse 28s linear infinite;
}

.about-core {
  position: absolute;
  z-index: 5;
  top: 50%;
  left: 50%;
  display: grid;
  width: 150px;
  min-height: 150px;
  place-items: center;
  align-content: center;
  gap: 0.35rem;
  border: 1px solid color-mix(in srgb, var(--color-primary) 38%, var(--about-border));
  border-radius: 30px;
  transform: translate(-50%, -50%);
  background: color-mix(in srgb, var(--color-surface) 92%, transparent);
  box-shadow:
    0 30px 70px color-mix(in srgb, #4f46e5 24%, transparent),
    0 0 0 9px color-mix(in srgb, var(--color-primary) 4%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(22px) saturate(135%);
  backdrop-filter: blur(22px) saturate(135%);
}

.about-core-halo {
  position: absolute;
  inset: -22px;
  z-index: -1;
  border-radius: 40px;
  background: radial-gradient(circle, color-mix(in srgb, var(--color-primary) 14%, transparent), transparent 72%);
  filter: blur(8px);
}

.about-core-icon {
  display: grid;
  width: 62px;
  height: 62px;
  place-items: center;
  border-radius: 18px;
  color: white;
  background: linear-gradient(135deg, #5a4de6 0%, #3b82f6 52%, #0f9f9a 100%);
  box-shadow: 0 16px 32px color-mix(in srgb, #4f46e5 26%, transparent);
}

.about-core strong {
  max-width: 120px;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 0.72rem;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.about-core span {
  color: var(--color-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.58rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.about-signal {
  position: absolute;
  z-index: 4;
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  gap: 0.5rem;
  max-width: 180px;
  padding: 0.4rem 0.68rem;
  border: 1px solid var(--about-border-strong);
  border-radius: 12px;
  color: var(--color-text-secondary);
  background: color-mix(in srgb, var(--color-surface) 90%, transparent);
  box-shadow:
    0 12px 28px color-mix(in srgb, #0f172a 9%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  font-size: 0.7rem;
  font-weight: 650;
  -webkit-backdrop-filter: blur(14px);
  backdrop-filter: blur(14px);
  animation: about-float 6s ease-in-out infinite;
}

.about-signal-icon {
  display: inline-flex;
  color: var(--color-primary);
}

.about-signal-model {
  top: 16%;
  left: 8%;
}

.about-signal-workflow {
  top: 19%;
  right: 7%;
  animation-delay: -1.5s;
}

.about-signal-token {
  bottom: 16%;
  left: 9%;
  animation-delay: -3s;
}

.about-signal-software {
  right: 8%;
  bottom: 13%;
  animation-delay: -4.5s;
}

.about-pulse {
  position: absolute;
  z-index: 2;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-accent);
  box-shadow: 0 0 0 5px color-mix(in srgb, var(--color-accent) 12%, transparent);
}

.about-pulse-a {
  top: 19%;
  left: 48%;
}

.about-pulse-b {
  top: 47%;
  right: 12%;
  background: var(--color-primary);
}

.about-pulse-c {
  bottom: 20%;
  left: 46%;
  background: var(--color-primary);
}

.about-pulse-d {
  top: 48%;
  left: 12%;
}

.about-visual-caption {
  position: absolute;
  right: 1rem;
  bottom: 1rem;
  z-index: 7;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--color-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.65rem;
  letter-spacing: 0.035em;
}

.about-visual-dot {
  width: 6px;
  height: 6px;
  background: var(--color-accent);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--color-accent) 10%, transparent);
}

.about-capabilities {
  margin-top: clamp(46px, 6vw, 78px);
}

.about-section-heading {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 0.8fr);
  align-items: end;
  gap: 2rem;
}

.about-section-title {
  margin: 0.45rem 0 0;
  color: var(--color-text-primary);
  font-size: clamp(1.45rem, 2.2vw, 2rem);
  line-height: 1.2;
  font-weight: 790;
  letter-spacing: -0.03em;
}

.about-section-description {
  max-width: 44rem;
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 0.9rem;
  line-height: 1.7;
}

.about-section-description-wide {
  max-width: none;
  margin-top: 1rem;
}

.about-services-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.9rem;
  margin-top: 1.35rem;
}

.about-service {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 190px;
  flex-direction: column;
  justify-content: space-between;
  overflow: hidden;
  padding: 1.15rem;
  border: 1px solid var(--about-border);
  border-radius: 17px;
  background: var(--about-panel);
  box-shadow:
    0 12px 28px color-mix(in srgb, #0f172a 5%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  transition:
    transform 200ms var(--ease-standard),
    border-color 200ms var(--ease-standard),
    box-shadow 200ms var(--ease-standard);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.about-service-featured {
  grid-column: span 2;
  min-height: 210px;
}

.about-service:hover {
  z-index: 2;
  border-color: color-mix(in srgb, var(--color-primary) 32%, var(--about-border));
  transform: translateY(-4px);
  box-shadow:
    0 20px 44px color-mix(in srgb, #0f172a 10%, transparent),
    inset 0 1px 0 var(--glass-highlight);
}

.about-service-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.about-service-index {
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.08em;
}

.about-service-icon {
  display: inline-flex;
  width: 38px;
  height: 38px;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, var(--color-primary) 22%, var(--about-border));
  border-radius: 11px;
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 60%, transparent);
}

.about-service h3 {
  margin: 1.2rem 0 0;
  color: var(--color-text-primary);
  font-size: 0.95rem;
  font-weight: 730;
}

.about-service-featured h3 {
  font-size: 1.08rem;
}

.about-service p {
  max-width: 34rem;
  margin: 0.5rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.8rem;
  line-height: 1.62;
}

.about-service-accent {
  position: absolute;
  right: -26px;
  bottom: -26px;
  width: 92px;
  height: 92px;
  border-radius: 999px;
  background: radial-gradient(circle, color-mix(in srgb, var(--color-primary) 10%, transparent), transparent 70%);
  pointer-events: none;
}

.about-operating-model {
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(360px, 0.88fr);
  gap: 0.95rem;
  margin-top: clamp(46px, 6vw, 76px);
}

.about-industries-card,
.about-delivery-card {
  min-width: 0;
  padding: clamp(22px, 3vw, 34px);
  border: 1px solid var(--about-border);
  border-radius: 20px;
  background: var(--about-panel);
  box-shadow:
    0 16px 36px color-mix(in srgb, #0f172a 6%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.about-section-card-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.about-section-chip {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--about-border);
  border-radius: 10px;
  color: var(--color-text-disabled);
  background: var(--about-panel-soft);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.65rem;
  font-weight: 800;
}

.about-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-top: 1.35rem;
}

.about-chip {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.78rem;
  border: 1px solid var(--about-border);
  border-radius: 999px;
  color: var(--color-text-secondary);
  background: var(--about-panel-soft);
  font-size: 0.78rem;
  font-weight: 600;
}

.about-chip-dot {
  width: 5px;
  height: 5px;
  background: linear-gradient(135deg, var(--color-primary), var(--color-accent));
}

.about-delivery-list {
  display: grid;
  gap: 0;
  margin: 1.4rem 0 0;
  padding: 0;
  list-style: none;
}

.about-delivery-list li {
  display: grid;
  min-height: 50px;
  grid-template-columns: 2.3rem 1px minmax(0, 1fr);
  align-items: center;
  gap: 0.9rem;
  color: var(--color-text-secondary);
  font-size: 0.84rem;
}

.about-step-number {
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.68rem;
  font-weight: 800;
}

.about-step-line {
  align-self: stretch;
  width: 1px;
  background: linear-gradient(
    180deg,
    transparent,
    color-mix(in srgb, var(--color-primary) 34%, var(--about-border)),
    transparent
  );
}

.about-step-copy {
  line-height: 1.5;
}

.about-contact {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(420px, 0.8fr);
  align-items: center;
  gap: 2rem;
  margin-top: clamp(46px, 6vw, 76px);
  overflow: hidden;
  padding: clamp(28px, 4vw, 46px);
  border: 1px solid color-mix(in srgb, var(--color-primary) 30%, var(--about-border));
  border-radius: 24px;
  background:
    linear-gradient(
      120deg,
      color-mix(in srgb, var(--color-primary-soft) 74%, var(--about-panel)),
      color-mix(in srgb, var(--color-accent) 7%, var(--about-panel)) 60%,
      var(--about-panel)
    );
  box-shadow:
    0 22px 52px color-mix(in srgb, #4f46e5 9%, transparent),
    inset 0 1px 0 var(--glass-highlight);
}

.about-contact-glow {
  position: absolute;
  top: -130px;
  left: 42%;
  width: 320px;
  height: 320px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-primary) 11%, transparent);
  filter: blur(70px);
  pointer-events: none;
}

.about-contact-copy,
.about-contact-details {
  position: relative;
  z-index: 1;
}

.about-contact-eyebrow {
  color: var(--color-accent);
}

.about-contact-title {
  max-width: 680px;
  margin: 0.5rem 0 0;
  color: var(--color-text-primary);
  font-size: clamp(1.55rem, 2.5vw, 2.25rem);
  line-height: 1.2;
  font-weight: 800;
  letter-spacing: -0.035em;
}

.about-contact-description {
  max-width: 660px;
  margin: 0.85rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.88rem;
  line-height: 1.7;
}

.about-contact-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.about-contact-item {
  display: flex;
  min-width: 0;
  min-height: 84px;
  align-items: center;
  gap: 0.8rem;
  padding: 0.8rem;
  border: 1px solid var(--about-border);
  border-radius: 15px;
  color: var(--color-text-primary);
  background: color-mix(in srgb, var(--color-surface) 82%, transparent);
  box-shadow: inset 0 1px 0 var(--glass-highlight);
  text-decoration: none;
  transition:
    transform 180ms var(--ease-standard),
    border-color 180ms var(--ease-standard),
    background-color 180ms var(--ease-standard);
  -webkit-backdrop-filter: blur(14px);
  backdrop-filter: blur(14px);
}

.about-contact-item:hover {
  border-color: color-mix(in srgb, var(--color-primary) 34%, var(--about-border));
  background: color-mix(in srgb, var(--color-primary-soft) 40%, var(--color-surface));
  transform: translateY(-2px);
}

.about-contact-item:focus-visible {
  outline: 3px solid var(--color-primary-ring);
  outline-offset: 3px;
}

.about-contact-icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border-radius: 12px;
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 68%, transparent);
}

.about-contact-value {
  display: grid;
  min-width: 0;
  gap: 0.2rem;
}

.about-contact-value strong {
  color: var(--color-text-muted);
  font-size: 0.68rem;
  font-weight: 750;
}

.about-contact-value > span {
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 0.86rem;
  font-weight: 720;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.dark) .about-page {
  --about-panel: rgba(15, 22, 34, 0.76);
  --about-panel-soft: rgba(18, 28, 42, 0.78);
  --about-border: rgba(124, 145, 177, 0.18);
  --about-border-strong: rgba(124, 145, 177, 0.24);
  --about-grid: rgba(124, 145, 177, 0.08);
}

:global(.dark) .about-hero,
:global(.dark) .about-service,
:global(.dark) .about-industries-card,
:global(.dark) .about-delivery-card,
:global(.dark) .about-contact {
  box-shadow:
    0 28px 64px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.025);
}

:global(.dark) .about-core {
  background: rgba(16, 24, 37, 0.9);
}

:global(.dark) .about-signal,
:global(.dark) .about-contact-item {
  background: rgba(17, 26, 40, 0.82);
}

@keyframes about-orbit-spin {
  to {
    transform: translate(-50%, -50%) rotate(360deg);
  }
}

@keyframes about-orbit-spin-reverse {
  to {
    transform: translate(-50%, -50%) rotate(-360deg);
  }
}

@keyframes about-float {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-5px);
  }
}

@media (max-width: 1180px) {
  .about-hero {
    grid-template-columns: minmax(0, 1fr) minmax(360px, 0.82fr);
  }

  .about-services-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .about-service-featured {
    grid-column: span 1;
  }

  .about-operating-model {
    grid-template-columns: 1fr 1fr;
  }

  .about-contact {
    grid-template-columns: 1fr;
  }

  .about-contact-details {
    max-width: 620px;
  }
}

@media (max-width: 900px) {
  .about-shell {
    width: min(100% - 28px, 760px);
  }

  .about-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 1rem;
  }

  .about-hero {
    grid-template-columns: 1fr;
  }

  .about-visual {
    min-height: 420px;
    border-top: 1px solid var(--about-border);
    border-left: 0;
  }

  .about-section-heading {
    grid-template-columns: 1fr;
    gap: 0.8rem;
  }

  .about-services-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .about-operating-model {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 620px) {
  .about-shell {
    width: min(100% - 20px, 560px);
    padding-top: 22px;
  }

  .about-title {
    font-size: 2.15rem;
  }

  .about-hero {
    min-height: auto;
    border-radius: 20px;
  }

  .about-hero-copy {
    padding: 26px 20px 30px;
  }

  .about-hero-title {
    font-size: 2rem;
  }

  .about-visual {
    min-height: 350px;
  }

  .about-orbit-three {
    width: 330px;
    height: 330px;
  }

  .about-orbit-two {
    width: 245px;
    height: 245px;
  }

  .about-signal {
    max-width: 136px;
    font-size: 0.62rem;
  }

  .about-signal-model {
    top: 12%;
    left: 4%;
  }

  .about-signal-workflow {
    top: 14%;
    right: 4%;
  }

  .about-signal-token {
    bottom: 12%;
    left: 4%;
  }

  .about-signal-software {
    right: 4%;
    bottom: 10%;
  }

  .about-services-grid {
    grid-template-columns: 1fr;
  }

  .about-service,
  .about-service-featured {
    min-height: 170px;
  }

  .about-industries-card,
  .about-delivery-card,
  .about-contact {
    border-radius: 18px;
  }

  .about-contact-details {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .about-orbit-two,
  .about-orbit-three,
  .about-signal {
    animation: none;
  }

  .about-service,
  .about-contact-item {
    transition: none;
  }

  .about-service:hover,
  .about-contact-item:hover {
    transform: none;
  }
}
</style>
