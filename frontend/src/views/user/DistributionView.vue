<template>
  <AppLayout>
    <DistributionPreview
      :eyebrow="t('distribution.eyebrow')"
      :title="t('distribution.headline')"
      :description="t('distribution.intro')"
      :illustration-label="t('distribution.illustrationLabel')"
      :status-label="t('distribution.status')"
      :items="previewItems"
    >
      <template #action>
        <button type="button" class="btn btn-primary min-h-11" @click="requestAccess">
          <Icon name="userPlus" size="sm" aria-hidden="true" />
          <span>{{ t('distribution.cta') }}</span>
        </button>
      </template>
    </DistributionPreview>
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

function requestAccess(): void {
  appStore.showInfo(t('distribution.permissionNotice'))
}
</script>
