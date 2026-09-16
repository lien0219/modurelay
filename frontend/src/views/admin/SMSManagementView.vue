<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1600px] space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">{{ t('sms.admin.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.smsManagement') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('sms.admin.description') }}</p>
        </div>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
          @click="load"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" />
        </button>
      </header>

      <section class="card flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('sms.admin.enableTitle') }}</h2>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('sms.admin.enableDescription') }}</p>
        </div>
        <label class="inline-flex min-h-11 shrink-0 cursor-pointer items-center gap-3 self-start sm:self-center" :class="savingEnabled ? 'cursor-wait opacity-70' : ''">
          <input
            v-model="enabled"
            type="checkbox"
            class="peer sr-only"
            :disabled="savingEnabled"
            :aria-label="t('sms.admin.switchLabel')"
            @change="toggle"
          />
          <span class="relative h-6 w-11 rounded-full bg-gray-300 transition-[background-color] duration-200 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow-sm after:transition-transform after:duration-200 peer-checked:bg-primary-600 peer-checked:after:translate-x-5 peer-focus-visible:ring-2 peer-focus-visible:ring-primary-500 peer-focus-visible:ring-offset-2 dark:bg-dark-600 dark:peer-focus-visible:ring-offset-dark-800" />
          <span class="min-w-12 text-sm font-medium text-gray-700 dark:text-gray-200">{{ enabled ? t('common.enabled') : t('common.disabled') }}</span>
        </label>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div v-for="item in statItems" :key="item.key" class="card p-4">
          <div class="text-xs uppercase tracking-wide text-gray-500">{{ item.label }}</div>
          <div class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('sms.admin.providers') }}</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800">
              <tr>
                <th class="px-5 py-3">{{ t('sms.admin.name') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.code') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.baseUrl') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.credential') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.health') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.enabled') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="provider in providers" :key="provider.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="px-5 py-3 font-medium text-gray-900 dark:text-white">
                  <div class="flex min-w-40 items-center gap-2">
                    <span>{{ provider.name }}</span>
                    <a
                      v-if="providerPortalUrl(provider)"
                      :href="providerPortalUrl(provider)"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="inline-flex min-h-8 items-center gap-1 rounded-md px-2 text-xs font-medium text-primary-600 transition-colors hover:bg-primary-50 hover:text-primary-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-primary-300 dark:hover:bg-primary-950/30 dark:hover:text-primary-200"
                      :title="t('sms.admin.openProvider')"
                    >
                      <Icon name="externalLink" size="xs" aria-hidden="true" />
                      <span>{{ t('sms.admin.portal') }}</span>
                    </a>
                  </div>
                </td>
                <td class="px-5 py-3 font-mono text-xs text-gray-600 dark:text-gray-300">{{ provider.code }}</td>
                <td class="px-5 py-3"><input v-model="provider.base_url" class="input min-w-52" :aria-label="`${t('sms.admin.baseUrl')} - ${provider.name}`" /></td>
                <td class="px-5 py-3">
                  <input v-model="credentialDrafts[provider.id]" type="password" autocomplete="new-password" class="input min-w-52" :placeholder="t('sms.admin.credentialPlaceholder')" :aria-label="`${t('sms.admin.credential')} - ${provider.name}`" />
                </td>
                <td class="px-5 py-3"><span class="badge" :class="provider.health_status === 'healthy' ? 'badge-success' : 'badge-warning'">{{ healthLabel(provider.health_status) }}</span></td>
                <td class="px-5 py-3"><input v-model="provider.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :aria-label="t('sms.admin.providerEnabled', { name: provider.name })" @change="saveProvider(provider)" /></td>
                <td class="px-5 py-3"><div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary btn-sm" @click="saveProvider(provider)">{{ t('sms.admin.save') }}</button><button v-if="supportsTestConnection(provider)" type="button" class="btn btn-secondary btn-sm" :disabled="testingProviderId === provider.id" :aria-busy="testingProviderId === provider.id" :title="t('sms.admin.testRequestNotice')" @click="testProvider(provider)">{{ testingProviderId === provider.id ? t('sms.admin.testing') : t('sms.admin.testConnection') }}</button></div></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('sms.admin.channels') }}</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800">
              <tr>
                <th class="px-5 py-3">{{ t('sms.admin.channels') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.providers') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.role') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.visible') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.health') }}</th>
                <th class="px-5 py-3">{{ t('sms.admin.enabled') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="channel in channels" :key="channel.id" class="border-t border-gray-100 dark:border-dark-700">
                <td class="px-5 py-3 font-medium text-gray-900 dark:text-white"><span>{{ channel.public_name }}</span> <span class="font-mono text-xs text-gray-500">{{ channel.code }}</span></td>
                <td class="px-5 py-3"><select v-model="channel.provider_id" class="input" :aria-label="`${t('sms.admin.providers')} - ${channel.public_name}`"><option v-for="provider in providers" :key="provider.id" :value="provider.id">{{ provider.name }}</option></select></td>
                <td class="px-5 py-3">{{ roleLabel(channel.role) }}</td>
                <td class="px-5 py-3"><input v-model="channel.visible" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :aria-label="t('sms.admin.channelVisible', { name: channel.public_name })" @change="saveChannel(channel)" /></td>
                <td class="px-5 py-3"><input v-model="channel.healthy" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :aria-label="t('sms.admin.channelHealthy', { name: channel.public_name })" @change="saveChannel(channel)" /></td>
                <td class="px-5 py-3"><input v-model="channel.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :aria-label="t('sms.admin.channelEnabled', { name: channel.public_name })" @change="saveChannel(channel)" /></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import adminSMS from '@/api/admin/sms'
import type { SMSChannelAdmin, SMSProviderAdmin } from '@/api/admin/sms'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()
const providers = ref<SMSProviderAdmin[]>([])
const channels = ref<SMSChannelAdmin[]>([])
const stats = ref<Record<string, number | boolean>>({})
const enabled = ref(false)
const loading = ref(false)
const savingEnabled = ref(false)
const testingProviderId = ref<number | null>(null)
const credentialDrafts = ref<Record<number, string>>({})

const providerPortals: Record<string, string> = {
  '5sim': 'https://5sim.net/',
  smspool: 'https://smspool.net/',
  sms_activate: 'https://sms-activate.org/',
  onlinesim: 'https://onlinesim.io/',
  pingme: 'https://pingme.tel/',
}
const statOrder = ['completed', 'feature_enabled', 'orders', 'refunded', 'revenue'] as const
const statItems = computed(() => statOrder.filter((key) => key in stats.value).map((key) => ({
  key,
  label: t(`sms.admin.stats.${key}`),
  value: key === 'feature_enabled' ? (stats.value[key] ? t('common.enabled') : t('common.disabled')) : stats.value[key],
})))

function providerPortalUrl(provider: SMSProviderAdmin) {
  return providerPortals[provider.code] || ''
}

function supportsTestConnection(provider: SMSProviderAdmin) {
  return ['5sim', 'onlinesim', 'smspool'].includes(provider.code.toLowerCase())
}

function errorMessage(error: unknown, fallback: string) {
  return (error as { message?: string })?.message || fallback
}

function healthLabel(status: string) {
  const labels: Record<string, string> = {
    healthy: t('sms.admin.status.healthy'),
    degraded: t('sms.admin.status.degraded'),
    unavailable: t('sms.admin.status.unavailable'),
    disabled: t('sms.admin.status.disabled'),
    unknown: t('sms.admin.status.unknown'),
  }
  return labels[status] || status
}

function roleLabel(role: string) {
  return role === 'primary' ? t('sms.admin.primary') : role === 'backup' ? t('sms.admin.backup') : role
}

async function load() {
  loading.value = true
  try {
    const [nextProviders, nextChannels, nextStats] = await Promise.all([adminSMS.providers(), adminSMS.channels(), adminSMS.stats()])
    providers.value = nextProviders
    credentialDrafts.value = Object.fromEntries(nextProviders.map((provider) => [provider.id, '']))
    channels.value = nextChannels
    stats.value = nextStats
    enabled.value = Boolean(stats.value.feature_enabled)
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.unavailable')))
  } finally {
    loading.value = false
  }
}

async function toggle() {
  const nextValue = enabled.value
  savingEnabled.value = true
  try {
    await adminSMS.setEnabled(nextValue)
    await load()
  } catch (error) {
    enabled.value = !nextValue
    appStore.showError(errorMessage(error, t('sms.admin.enableTitle')))
  } finally {
    savingEnabled.value = false
  }
}

async function saveProvider(provider: SMSProviderAdmin) {
  const credential = (credentialDrafts.value[provider.id] || '').trim()
  if (!credential && !provider.credential_configured) {
    appStore.showError(t('sms.admin.credentialRequired'))
    return
  }
  try {
    await adminSMS.updateProvider(provider.id, { enabled: provider.enabled, base_url: provider.base_url, credential_ref: credential || undefined })
    if (credential) provider.credential_configured = true
    credentialDrafts.value[provider.id] = ''
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.admin.providers')))
  }
}

async function testProvider(provider: SMSProviderAdmin) {
  testingProviderId.value = provider.id
  try {
    const result = await adminSMS.testProvider(provider.id)
    provider.health_status = result.health_status
    appStore.showSuccess(`${t('sms.admin.testSuccess')} (${result.latency_ms}ms)`)
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.admin.testFailed')))
  } finally {
    testingProviderId.value = null
  }
}

async function saveChannel(channel: SMSChannelAdmin) {
  try {
    await adminSMS.updateChannel(channel.id, { enabled: channel.enabled, visible: channel.visible, healthy: channel.healthy, provider_id: channel.provider_id })
    await load()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.admin.channels')))
  }
}

onMounted(load)
</script>
