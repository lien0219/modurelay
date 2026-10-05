<template>
  <AppLayout>
    <section class="sa-admin">
      <header><div><h1>{{ t('serviceAccounts.adminTitle') }}</h1><p>{{ t('serviceAccounts.adminDescription') }}</p></div><div class="sa-admin__tools"><input v-model="search" class="input" :placeholder="t('serviceAccounts.search')" @keydown.enter="load"><button class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ t('common.search') }}</button></div></header>
      <p v-if="error" role="alert">{{ t('serviceAccounts.loadError') }}</p><p v-else-if="loading" role="status">{{ t('common.loading') }}</p>
      <div v-else class="sa-admin__table"><table><thead><tr><th>{{ t('common.name') }}</th><th>{{ t('workspace.slug') }}</th><th>{{ t('workspace.workspace') }}</th><th>{{ t('workspace.project') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr></thead><tbody><tr v-for="item in accounts" :key="item.id"><td>{{ item.name }}<small>#{{ item.id }}</small></td><td><code>{{ item.slug }}</code></td><td>#{{ item.workspace_id }}</td><td>#{{ item.project_id }}</td><td>{{ item.status }}</td><td><div class="sa-admin__tools"><button class="btn btn-secondary btn-sm" @click="inspect(item.id)">{{ t('workspace.inspect') }}</button><button v-if="item.status === 'active'" class="btn btn-ghost btn-sm" @click="disable(item)">{{ t('serviceAccounts.disable') }}</button></div></td></tr></tbody></table></div>
      <section v-if="detail" class="sa-admin__inspect"><h2>{{ detail.service_account.name }}</h2><p>{{ detail.service_account.description }}</p><div class="sa-admin__table"><table><thead><tr><th>{{ t('workspace.keyName') }}</th><th>{{ t('serviceAccounts.masked') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr></thead><tbody><tr v-for="key in detail.credentials" :key="key.id"><td>{{ key.name }}</td><td><code>••••{{ key.key_suffix }}</code></td><td>{{ key.status }}</td><td><button v-if="key.status !== 'revoked'" class="btn btn-ghost btn-sm" @click="revoke(detail!.service_account.id,key.id)">{{ t('serviceAccounts.revoke') }}</button></td></tr></tbody></table></div></section>
    </section>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { serviceAccountsAPI, type ServiceAccount, type ServiceAccountCredential } from '@/api/serviceAccounts'
import { useAppStore } from '@/stores/app'
const { t } = useI18n(), app = useAppStore()
const accounts = ref<ServiceAccount[]>([]), detail = ref<{ service_account: ServiceAccount; credentials: ServiceAccountCredential[] } | null>(null), loading = ref(false), error = ref(false), search = ref('')
async function load() { loading.value = true; error.value = false; try { accounts.value = (await serviceAccountsAPI.adminList({ search: search.value || undefined })).items || [] } catch { error.value = true } finally { loading.value = false } }
async function inspect(id: number) { try { detail.value = await serviceAccountsAPI.adminInspect(id) } catch (cause) { app.showError((cause as { message?: string })?.message || t('serviceAccounts.loadError')) } }
async function disable(item: ServiceAccount) { try { await serviceAccountsAPI.adminDisable(item.id); item.status = 'disabled'; if (detail.value?.service_account.id === item.id) detail.value.service_account.status = 'disabled' } catch (cause) { app.showError((cause as { message?: string })?.message || t('serviceAccounts.saveError')) } }
async function revoke(sa: number, credential: number) { try { await serviceAccountsAPI.adminRevoke(sa, credential); if (detail.value) { const item = detail.value.credentials.find(key => key.id === credential); if (item) item.status = 'revoked' } } catch (cause) { app.showError((cause as { message?: string })?.message || t('serviceAccounts.saveError')) } }
onMounted(load)
</script>
<style scoped>
.sa-admin { max-width: 1440px; margin: 0 auto; }.sa-admin header { display:flex; align-items:flex-end; justify-content:space-between; gap:16px; padding:8px 0 20px; }.sa-admin h1 { margin:0; color:var(--color-text-primary); font-size:24px; }.sa-admin p { color:var(--color-text-secondary); }.sa-admin__tools { display:flex; flex-wrap:wrap; gap:8px; }.sa-admin__table { overflow-x:auto; border:1px solid var(--color-border); border-radius:10px; background:var(--color-surface); }.sa-admin table { width:100%; min-width:760px; border-collapse:collapse; }.sa-admin th,.sa-admin td { padding:12px; border-bottom:1px solid var(--color-border-subtle); text-align:left; color:var(--color-text-secondary); font-size:13px; }.sa-admin th { color:var(--color-text-muted); }.sa-admin small { display:block; margin-top:3px; }.sa-admin__inspect { margin-top:20px; padding:16px; border:1px solid var(--color-border); border-radius:10px; background:var(--color-surface); }.sa-admin__inspect h2 { margin:0; font-size:17px; color:var(--color-text-primary); }@media(max-width:700px){.sa-admin header{align-items:stretch;flex-direction:column}}
</style>
