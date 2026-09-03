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
      <span class="distribution-preview__line distribution-preview__line--one" aria-hidden="true"></span>
      <span class="distribution-preview__line distribution-preview__line--two" aria-hidden="true"></span>
      <span class="distribution-preview__line distribution-preview__line--three" aria-hidden="true"></span>

      <span class="distribution-preview__node distribution-preview__node--top" aria-hidden="true">
        <Icon name="users" size="lg" />
      </span>
      <span class="distribution-preview__node distribution-preview__node--right" aria-hidden="true">
        <Icon name="trendingUp" size="lg" />
      </span>
      <span class="distribution-preview__node distribution-preview__node--bottom" aria-hidden="true">
        <Icon name="link" size="lg" />
      </span>
      <span class="distribution-preview__hub" aria-hidden="true">
        <Icon name="userPlus" size="xl" />
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
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(17.5rem, 0.92fr);
  gap: 2rem;
  overflow: hidden;
  padding: 1.75rem;
  border: 1px solid var(--color-border);
  border-radius: 1rem;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.distribution-preview__content {
  min-width: 0;
  align-self: center;
}

.distribution-preview__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0;
  color: var(--color-primary);
  font-size: 0.8125rem;
  font-weight: 600;
}

.distribution-preview__title {
  margin: 0.75rem 0 0;
  color: var(--color-text-primary);
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.3;
  text-wrap: balance;
}

.distribution-preview__description {
  max-width: 38rem;
  margin: 0.75rem 0 0;
  color: var(--color-text-secondary);
  font-size: 0.875rem;
  line-height: 1.7;
}

.distribution-preview__features {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
  margin: 1.5rem 0 0;
  padding: 0;
  list-style: none;
}

.distribution-preview__feature {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 0.625rem;
}

.distribution-preview__feature-icon {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: 0 0 2rem;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 0.5rem;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.distribution-preview__feature-copy {
  display: grid;
  min-width: 0;
  gap: 0.2rem;
}

.distribution-preview__feature-copy strong {
  color: var(--color-text-primary);
  font-size: 0.8125rem;
  font-weight: 600;
  line-height: 1.4;
}

.distribution-preview__feature-copy span {
  color: var(--color-text-muted);
  font-size: 0.75rem;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.distribution-preview__action {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
  margin-top: 1.5rem;
}

.distribution-preview__visual {
  position: relative;
  min-height: 19rem;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--color-border-subtle);
  border-radius: 0.875rem;
  background: var(--color-surface-soft);
  isolation: isolate;
}

.distribution-preview__visual::before,
.distribution-preview__visual::after {
  position: absolute;
  width: 9rem;
  height: 9rem;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  content: '';
}

.distribution-preview__visual::before {
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}

.distribution-preview__visual::after {
  top: 50%;
  left: 50%;
  width: 14rem;
  height: 14rem;
  border-color: var(--color-border);
  transform: translate(-50%, -50%);
}

.distribution-preview__line {
  position: absolute;
  z-index: 1;
  top: 50%;
  left: 50%;
  width: 7.5rem;
  height: 1px;
  background: var(--color-primary-border);
  transform-origin: left center;
}

.distribution-preview__line--one {
  transform: rotate(-90deg);
}

.distribution-preview__line--two {
  transform: rotate(12deg);
}

.distribution-preview__line--three {
  transform: rotate(126deg);
}

.distribution-preview__node,
.distribution-preview__hub {
  position: absolute;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-border);
  background: var(--color-surface-raised);
  color: var(--color-text-secondary);
  box-shadow: var(--shadow-sm);
}

.distribution-preview__node {
  width: 3.25rem;
  height: 3.25rem;
  border-radius: 0.75rem;
}

.distribution-preview__node--top {
  top: calc(50% - 8.625rem);
  left: calc(50% - 1.625rem);
}

.distribution-preview__node--right {
  top: calc(50% - 0.5rem);
  right: calc(50% - 9.25rem);
}

.distribution-preview__node--bottom {
  bottom: calc(50% - 7.875rem);
  left: calc(50% - 7rem);
}

.distribution-preview__hub {
  top: 50%;
  left: 50%;
  width: 5rem;
  height: 5rem;
  border-color: var(--color-primary-border);
  border-radius: 1rem;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  transform: translate(-50%, -50%);
}

.distribution-preview__status {
  position: absolute;
  z-index: 3;
  right: 1rem;
  bottom: 1rem;
  display: inline-flex;
  min-height: 2rem;
  align-items: center;
  gap: 0.4rem;
  padding: 0.35rem 0.65rem;
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: var(--color-surface-overlay);
  color: var(--color-text-secondary);
  font-size: 0.75rem;
  font-weight: 600;
  box-shadow: var(--shadow-xs);
}

@media (max-width: 900px) {
  .distribution-preview {
    grid-template-columns: minmax(0, 1fr);
  }

  .distribution-preview__visual {
    min-height: 17.5rem;
  }
}

@media (max-width: 640px) {
  .distribution-preview {
    gap: 1.5rem;
    padding: 1.25rem;
  }

  .distribution-preview__features {
    grid-template-columns: minmax(0, 1fr);
  }

  .distribution-preview__visual {
    min-height: 17rem;
  }

  .distribution-preview__visual::after {
    width: 12rem;
    height: 12rem;
  }

  .distribution-preview__line {
    width: 6.5rem;
  }

  .distribution-preview__node--top {
    top: calc(50% - 7.875rem);
  }

  .distribution-preview__node--right {
    right: calc(50% - 7.75rem);
  }

  .distribution-preview__node--bottom {
    bottom: calc(50% - 7.125rem);
  }
}
</style>
