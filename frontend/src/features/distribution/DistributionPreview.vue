<template>
  <section class="distribution-preview">
    <div class="distribution-preview__content">
      <p class="distribution-preview__eyebrow">
        <Icon name="sparkles" size="sm" aria-hidden="true" />
        <span>{{ eyebrow }}</span>
      </p>

      <h1 class="distribution-preview__title">{{ title }}</h1>
      <p class="distribution-preview__description">{{ description }}</p>

      <ul class="distribution-preview__features">
        <li v-for="(item, index) in items" :key="item.title" class="distribution-preview__feature">
          <span class="distribution-preview__feature-icon" aria-hidden="true">
            <Icon :name="featureIcon(index)" size="sm" />
          </span>
          <span class="distribution-preview__feature-copy">
            <strong>{{ item.title }}</strong>
            <span>{{ item.description }}</span>
          </span>
        </li>
      </ul>

      <div v-if="$slots.action" class="distribution-preview__action">
        <slot name="action" />
      </div>
    </div>

    <figure
      class="distribution-preview__visual"
      role="img"
      :aria-label="illustrationLabel"
    >
      <span class="distribution-preview__orb distribution-preview__orb--one" aria-hidden="true"></span>
      <span class="distribution-preview__orb distribution-preview__orb--two" aria-hidden="true"></span>
      <span class="distribution-preview__orb distribution-preview__orb--three" aria-hidden="true"></span>

      <span class="distribution-preview__line distribution-preview__line--one" aria-hidden="true"></span>
      <span class="distribution-preview__line distribution-preview__line--two" aria-hidden="true"></span>
      <span class="distribution-preview__line distribution-preview__line--three" aria-hidden="true"></span>

      <span class="distribution-preview__node distribution-preview__node--top" aria-hidden="true">
        <span class="distribution-preview__node-icon"><Icon name="badge" size="md" /></span>
        <span>{{ items[0]?.title }}</span>
      </span>
      <span class="distribution-preview__node distribution-preview__node--right" aria-hidden="true">
        <span class="distribution-preview__node-icon distribution-preview__node-icon--accent"><Icon name="users" size="md" /></span>
        <span>{{ items[1]?.title }}</span>
      </span>
      <span class="distribution-preview__node distribution-preview__node--bottom" aria-hidden="true">
        <span class="distribution-preview__node-icon"><Icon name="trendingUp" size="md" /></span>
        <span>{{ items[2]?.title }}</span>
      </span>

      <span class="distribution-preview__hub" aria-hidden="true">
        <span class="distribution-preview__hub-back"></span>
        <span class="distribution-preview__hub-mid"></span>
        <span class="distribution-preview__hub-front">
          <Icon name="userPlus" size="xl" />
        </span>
      </span>

      <figcaption class="distribution-preview__status">
        <Icon name="calculator" size="sm" aria-hidden="true" />
        <span>{{ statusLabel }}</span>
      </figcaption>
    </figure>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

interface PreviewItem {
  title: string
  description: string
}

defineProps<{
  eyebrow: string
  title: string
  description: string
  illustrationLabel: string
  statusLabel: string
  items: PreviewItem[]
}>()

function featureIcon(index: number): 'badge' | 'trendingUp' | 'calculator' {
  return (['badge', 'trendingUp', 'calculator'] as const)[index % 3]
}
</script>

<style scoped>
.distribution-preview {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.16fr) minmax(420px, .84fr);
  min-height: 350px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background:
    radial-gradient(circle at 12% 14%, color-mix(in srgb, var(--color-primary) 6%, transparent), transparent 28%),
    var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.distribution-preview::after {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(color-mix(in srgb, var(--color-text-primary) 4%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--color-text-primary) 4%, transparent) 1px, transparent 1px);
  background-size: 40px 40px;
  content: '';
  opacity: .35;
  -webkit-mask-image: linear-gradient(90deg, #000 0%, transparent 60%);
  mask-image: linear-gradient(90deg, #000 0%, transparent 60%);
  pointer-events: none;
}

.distribution-preview__content {
  position: relative;
  z-index: 2;
  min-width: 0;
  align-self: center;
  padding: 34px 38px;
}

.distribution-preview__eyebrow {
  display: inline-flex;
  min-height: 30px;
  align-items: center;
  gap: 7px;
  margin: 0;
  padding: 0 10px;
  border: 1px solid var(--color-primary-border);
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .72rem;
  font-weight: 700;
}

.distribution-preview__title {
  max-width: 760px;
  margin: 1rem 0 0;
  color: var(--color-text-primary);
  font-size: clamp(2rem, 3.35vw, 3.1rem);
  font-weight: 720;
  line-height: 1.12;
  letter-spacing: -.048em;
  text-wrap: balance;
}

.distribution-preview__description {
  max-width: 720px;
  margin: .85rem 0 0;
  color: var(--color-text-secondary);
  font-size: .9rem;
  line-height: 1.72;
}

.distribution-preview__features {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 1.45rem 0 0;
  padding: 0;
  list-style: none;
}

.distribution-preview__feature {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 9px;
  padding: 10px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 11px;
  background: color-mix(in srgb, var(--color-surface) 82%, transparent);
  box-shadow: inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(10px);
  backdrop-filter: blur(10px);
}

.distribution-preview__feature-icon {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.distribution-preview__feature-copy {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.distribution-preview__feature-copy strong {
  color: var(--color-text-primary);
  font-size: .76rem;
  font-weight: 660;
  line-height: 1.35;
}

.distribution-preview__feature-copy span {
  color: var(--color-text-muted);
  font-size: .67rem;
  line-height: 1.45;
}

.distribution-preview__action {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 1.4rem;
}

.distribution-preview__visual {
  position: relative;
  z-index: 1;
  min-height: 350px;
  margin: 0;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background:
    radial-gradient(circle at 50% 46%, color-mix(in srgb, var(--color-primary) 15%, transparent), transparent 26%),
    radial-gradient(circle at 62% 68%, color-mix(in srgb, var(--color-accent) 10%, transparent), transparent 25%),
    var(--color-surface-soft);
}

.distribution-preview__visual::before {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(color-mix(in srgb, var(--color-text-primary) 5%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--color-text-primary) 5%, transparent) 1px, transparent 1px);
  background-size: 32px 32px;
  content: '';
  opacity: .5;
  -webkit-mask-image: radial-gradient(circle at 50% 50%, #000 10%, transparent 76%);
  mask-image: radial-gradient(circle at 50% 50%, #000 10%, transparent 76%);
}

.distribution-preview__orb {
  position: absolute;
  z-index: 0;
  top: 50%;
  left: 50%;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.distribution-preview__orb--one {
  width: 170px;
  height: 170px;
}

.distribution-preview__orb--two {
  width: 250px;
  height: 250px;
  border-style: dashed;
  opacity: .78;
}

.distribution-preview__orb--three {
  width: 340px;
  height: 340px;
  border-color: color-mix(in srgb, var(--color-accent) 22%, var(--color-border));
  border-style: dashed;
  opacity: .55;
}

.distribution-preview__line {
  position: absolute;
  z-index: 1;
  top: 50%;
  left: 50%;
  width: 145px;
  height: 1px;
  background: linear-gradient(90deg, var(--color-primary-border), transparent);
  transform-origin: left center;
}

.distribution-preview__line--one {
  transform: rotate(-92deg);
}

.distribution-preview__line--two {
  transform: rotate(8deg);
}

.distribution-preview__line--three {
  transform: rotate(135deg);
}

.distribution-preview__node {
  position: absolute;
  z-index: 3;
  display: inline-flex;
  min-height: 48px;
  align-items: center;
  gap: 8px;
  max-width: 180px;
  padding: 7px 10px;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  background: var(--color-surface);
  background: var(--glass-bg);
  box-shadow: var(--shadow-sm);
  color: var(--color-text-secondary);
  font-size: .68rem;
  font-weight: 630;
  -webkit-backdrop-filter: blur(14px);
  backdrop-filter: blur(14px);
}

.distribution-preview__node-icon {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  place-items: center;
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.distribution-preview__node-icon--accent {
  background: var(--color-accent-soft);
  color: var(--color-accent);
}

.distribution-preview__node--top {
  top: 40px;
  left: 50%;
  transform: translateX(-50%);
}

.distribution-preview__node--right {
  top: 50%;
  right: 24px;
  transform: translateY(-50%);
}

.distribution-preview__node--bottom {
  bottom: 46px;
  left: 46px;
}

.distribution-preview__hub {
  position: absolute;
  z-index: 4;
  top: 50%;
  left: 50%;
  width: 122px;
  height: 112px;
  transform: translate(-50%, -50%);
}

.distribution-preview__hub-back,
.distribution-preview__hub-mid,
.distribution-preview__hub-front {
  position: absolute;
  top: 50%;
  left: 50%;
  display: grid;
  width: 88px;
  height: 88px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 22px;
  background: var(--color-surface-raised);
  box-shadow: var(--shadow-sm);
}

.distribution-preview__hub-back {
  transform: translate(-64%, -36%) rotate(-8deg);
  opacity: .5;
}

.distribution-preview__hub-mid {
  transform: translate(-36%, -62%) rotate(7deg);
  opacity: .72;
}

.distribution-preview__hub-front {
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 68%, var(--color-surface-raised));
  box-shadow: var(--shadow-md);
  transform: translate(-50%, -50%);
}

.distribution-preview__status {
  position: absolute;
  z-index: 5;
  right: 16px;
  bottom: 14px;
  display: inline-flex;
  min-height: 30px;
  align-items: center;
  gap: 6px;
  padding: 0 9px;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: var(--color-surface-overlay);
  color: var(--color-text-secondary);
  font-size: .68rem;
  font-weight: 620;
  box-shadow: var(--shadow-xs);
}

@media (max-width: 1080px) {
  .distribution-preview {
    grid-template-columns: minmax(0, 1fr) 360px;
  }

  .distribution-preview__features {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .distribution-preview {
    grid-template-columns: 1fr;
  }

  .distribution-preview__visual {
    min-height: 300px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .distribution-preview__features {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .distribution-preview {
    min-height: auto;
    border-radius: 14px;
  }

  .distribution-preview__content {
    padding: 22px 20px;
  }

  .distribution-preview__title {
    font-size: 1.9rem;
  }

  .distribution-preview__features {
    grid-template-columns: 1fr;
  }

  .distribution-preview__visual {
    display: none;
  }

  .distribution-preview__action :deep(button) {
    width: 100%;
  }
}
</style>
