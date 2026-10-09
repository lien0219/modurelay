<template>
  <WorkspaceFrame section="webhooks">
    <div class="workspace-page">
      <section v-if="canRead" class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><h2>{{ t('workspace.webhooks') }}</h2><p>{{ t('workspace.webhooksDescription') }}</p></div>
          <div class="workspace-actions">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || !!busy" @click="load">{{ t('common.refresh') }}</button>
            <button v-if="store.can('webhook.create')" type="button" class="btn btn-primary btn-sm" data-testid="webhook-create-toggle" :disabled="!!busy" @click="openCreate">{{ t('workspace.createWebhook') }}</button>
          </div>
        </div>

        <form v-if="formOpen" class="workspace-form" @submit.prevent="save">
          <label>
            <span>{{ t('workspace.webhookName') }}</span>
            <input v-model="form.name" name="webhook-name" class="input" required maxlength="100" :disabled="!!busy" :aria-invalid="!!fieldErrors.name" :aria-describedby="fieldErrors.name ? 'webhook-name-error' : undefined">
            <span v-if="fieldErrors.name" id="webhook-name-error" class="workspace-field-error" role="alert">{{ t('workspace.webhookNameRequired') }}</span>
          </label>
          <label>
            <span>{{ t('workspace.webhookUrl') }}</span>
            <input v-model="form.url" name="webhook-url" class="input" type="url" required maxlength="2048" placeholder="https://example.com/webhooks" :disabled="!!busy" :aria-invalid="!!fieldErrors.url" :aria-describedby="fieldErrors.url ? 'webhook-url-hint webhook-url-error' : 'webhook-url-hint'">
            <span id="webhook-url-hint" class="workspace-hint">{{ t('workspace.webhookUrlHint') }}</span>
            <span v-if="fieldErrors.url" id="webhook-url-error" class="workspace-field-error" role="alert">{{ t('workspace.webhookUrlInvalid') }}</span>
          </label>
          <fieldset class="workspace-events" :disabled="!!busy" :aria-describedby="fieldErrors.events ? 'webhook-events-error' : undefined">
            <legend>{{ t('workspace.webhookEvents') }}</legend>
            <div class="workspace-event-options">
              <label v-for="event in allowedEvents" :key="event" class="workspace-checkbox">
                <input v-model="form.eventTypes" type="checkbox" name="webhook-events" :value="event">
                <code>{{ event }}</code>
              </label>
            </div>
            <p v-if="fieldErrors.events" id="webhook-events-error" class="workspace-field-error" role="alert">{{ t('workspace.webhookEventsRequired') }}</p>
          </fieldset>
          <label v-if="editing" class="workspace-checkbox"><input v-model="form.enabled" name="webhook-enabled" type="checkbox" :disabled="!!busy"> {{ t('workspace.webhookEnabled') }}</label>
          <div class="workspace-actions">
            <button class="btn btn-primary" type="submit" :disabled="!!busy">{{ busy === 'save' ? t('common.saving') : t('workspace.saveWebhook') }}</button>
            <button type="button" class="btn btn-secondary" :disabled="!!busy" @click="closeForm">{{ t('common.cancel') }}</button>
          </div>
        </form>

        <div v-if="secret" class="workspace-token" data-testid="webhook-secret">
          <div class="min-w-0"><strong>{{ t('workspace.webhookSecret') }}: {{ secretName }}</strong><p role="status">{{ t('workspace.secretShownOnce') }}</p></div>
          <code>{{ secret }}</code>
          <button type="button" class="btn btn-secondary btn-sm" @click="copySecret">{{ t('workspace.copySecret') }}</button>
          <button type="button" class="btn btn-ghost btn-sm" data-testid="webhook-secret-dismiss" @click="clearSecret">{{ t('common.close') }}</button>
        </div>

        <div v-if="listError" class="workspace-error" role="alert">
          <span>{{ t('workspace.webhooksLoadError') }}</span>
          <button type="button" class="btn btn-secondary btn-sm" data-testid="webhook-list-retry" :disabled="loading" @click="load">{{ t('common.retry') }}</button>
        </div>
        <div v-if="loading && !webhooks.length" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="webhooks.length" class="workspace-table-wrap" :aria-busy="loading">
          <table class="workspace-table">
            <thead><tr><th scope="col">{{ t('workspace.webhookName') }}</th><th scope="col">{{ t('workspace.webhookUrl') }}</th><th scope="col">{{ t('workspace.webhookEvents') }}</th><th scope="col">{{ t('workspace.status') }}</th><th scope="col">{{ t('common.actions') }}</th></tr></thead>
            <tbody>
              <tr v-for="webhook in webhooks" :key="webhook.id">
                <td><strong>{{ webhook.name }}</strong><small>#{{ webhook.id }}</small><small v-if="webhook.updated_at">{{ t('workspace.webhookUpdated') }}: <time :datetime="webhook.updated_at">{{ formatDate(webhook.updated_at) }}</time></small></td>
                <td><code class="workspace-breakable">{{ webhook.url }}</code><small v-if="webhook.previous_secret_until">{{ t('workspace.previousSecretUntil', { date: formatDate(webhook.previous_secret_until) }) }}</small></td>
                <td><div class="workspace-chips"><span v-for="event in webhook.event_types" :key="event" class="workspace-chip">{{ event }}</span></div></td>
                <td><span class="workspace-status">{{ webhook.enabled ? t('common.enabled') : t('common.disabled') }}</span><small v-if="webhook.disabled_at">{{ t('workspace.webhookDisabledAt') }}: <time :datetime="webhook.disabled_at">{{ formatDate(webhook.disabled_at) }}</time></small></td>
                <td>
                  <div class="workspace-actions">
                    <button v-if="store.can('webhook.update')" type="button" class="btn btn-secondary btn-sm" :data-testid="`webhook-edit-${webhook.id}`" :disabled="!!busy" @click="openEdit(webhook)">{{ t('common.edit') }}</button>
                    <button v-if="store.can('webhook.secret.rotate')" type="button" class="btn btn-ghost btn-sm" :data-testid="`webhook-rotate-${webhook.id}`" :disabled="!!busy" @click="rotate(webhook)">{{ t('workspace.rotateSecret') }}</button>
                    <button v-if="store.can('webhook.test')" type="button" class="btn btn-ghost btn-sm" :data-testid="`webhook-test-${webhook.id}`" :disabled="!!busy" @click="sendTest(webhook.id)">{{ t('workspace.testWebhook') }}</button>
                    <button v-if="store.can('webhook.delete')" type="button" class="btn btn-ghost btn-sm workspace-danger" :data-testid="`webhook-delete-${webhook.id}`" :disabled="!!busy" @click="remove(webhook.id)">{{ t('workspace.deleteWebhook') }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else-if="!listError" class="workspace-state">{{ t('workspace.noWebhooks') }}</div>
      </section>
      <p v-else class="workspace-state">{{ t('workspace.webhooksUnavailable') }}</p>

      <section v-if="canRead && store.can('webhook.delivery.read') && selectedWebhook" class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><h2>{{ t('workspace.deliveries') }}</h2><p class="workspace-breakable">{{ selectedWebhook.url }}</p></div>
          <div class="workspace-actions">
            <select v-model.number="selectedWebhookId" class="input workspace-delivery-select" :aria-label="t('workspace.deliveryWebhook')">
              <option v-for="webhook in webhooks" :key="webhook.id" :value="webhook.id">{{ webhook.name }}</option>
            </select>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="deliveriesLoading || !!busy" @click="loadDeliveries">{{ t('common.refresh') }}</button>
          </div>
        </div>
        <div v-if="deliveryError" class="workspace-error" role="alert"><span>{{ t('workspace.deliveriesLoadError') }}</span><button type="button" class="btn btn-secondary btn-sm" :disabled="deliveriesLoading" @click="loadDeliveries">{{ t('common.retry') }}</button></div>
        <div v-if="deliveriesLoading && !deliveries.length" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="deliveries.length" class="workspace-table-wrap" :aria-busy="deliveriesLoading">
          <table class="workspace-table workspace-table--deliveries">
            <thead><tr><th scope="col">{{ t('workspace.webhookEvents') }}</th><th scope="col">{{ t('workspace.status') }}</th><th scope="col">{{ t('workspace.attempts') }}</th><th scope="col">{{ t('workspace.response') }}</th><th scope="col">{{ t('workspace.deliveryTiming') }}</th><th scope="col">{{ t('common.actions') }}</th></tr></thead>
            <tbody>
              <tr v-for="delivery in deliveries" :key="delivery.id">
                <td><code>{{ delivery.event_type }}</code><small class="workspace-breakable">{{ delivery.event_id }}</small></td>
                <td><span class="workspace-status">{{ deliveryStatus(delivery.status) }}</span></td>
                <td>{{ delivery.attempts }}</td>
                <td class="workspace-response">
                  <span>{{ delivery.response_status ?? '--' }}</span>
                  <small v-if="delivery.error_code"><code>{{ delivery.error_code }}</code></small>
                  <small v-if="delivery.last_error" class="workspace-breakable">{{ delivery.last_error }}</small>
                  <details v-if="delivery.response_preview"><summary>{{ t('workspace.responseDetails') }}</summary><pre>{{ delivery.response_preview }}</pre></details>
                </td>
                <td>
                  <small v-if="delivery.created_at">{{ t('workspace.created') }}: <time :datetime="delivery.created_at">{{ formatDate(delivery.created_at) }}</time></small>
                  <small v-if="delivery.delivered_at">{{ t('workspace.deliveredAt') }}: <time :datetime="delivery.delivered_at">{{ formatDate(delivery.delivered_at) }}</time></small>
                  <small v-if="delivery.last_attempt_at">{{ t('workspace.lastAttempt') }}: <time :datetime="delivery.last_attempt_at">{{ formatDate(delivery.last_attempt_at) }}</time></small>
                  <small v-if="delivery.next_attempt_at">{{ t('workspace.nextAttempt') }}: <time :datetime="delivery.next_attempt_at">{{ formatDate(delivery.next_attempt_at) }}</time></small>
                </td>
                <td><button v-if="store.can('webhook.delivery.retry') && retryable(delivery.status)" type="button" class="btn btn-ghost btn-sm" :data-testid="`webhook-retry-${delivery.id}`" :disabled="!!busy" @click="retry(delivery.id)">{{ t('workspace.retryDelivery') }}</button></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else-if="!deliveryError" class="workspace-state">{{ t('workspace.noDeliveries') }}</div>
        <div v-if="deliveryPages > 1" class="workspace-pagination">
          <span>{{ t('workspace.deliveryPage', { page: deliveryPage, pages: deliveryPages, total: deliveryTotal }) }}</span>
          <div class="workspace-actions">
            <button type="button" class="btn btn-secondary btn-sm" data-testid="webhook-deliveries-prev" :disabled="deliveriesLoading || deliveryPage <= 1 || !!busy" @click="changeDeliveryPage(-1)">{{ t('common.back') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" data-testid="webhook-deliveries-next" :disabled="deliveriesLoading || deliveryPage >= deliveryPages || !!busy" @click="changeDeliveryPage(1)">{{ t('common.next') }}</button>
          </div>
        </div>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { webhooksAPI, type WorkspaceWebhook, type WorkspaceWebhookDelivery } from '@/api/webhooks'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const allowedEvents = [
  'workspace.created', 'workspace.updated', 'workspace.suspended', 'workspace.resumed', 'workspace.archived',
  'member.invited', 'member.joined', 'member.role_changed', 'member.suspended', 'member.removed',
  'project.created', 'project.updated', 'project.archived', 'project.restored',
  'api_key.created', 'api_key.updated', 'api_key.revoked',
  'service_account.created', 'service_account.updated', 'service_account.disabled', 'service_account.enabled',
  'service_account.credential.created', 'service_account.credential.updated', 'service_account.credential.revoked', 'service_account.credential.rotated', 'service_account.credential.expiring', 'service_account.credential.expired',
  'budget.threshold_reached', 'budget.soft_limit_exceeded', 'budget.hard_limit_reached', 'budget.updated',
  'billing.settlement_pending', 'billing.settlement_recovered', 'quota.threshold_reached', 'quota.exhausted', 'webhook.test', 'webhook.administrator_retried',
]
const { t, te, locale } = useI18n()
const store = useWorkspaceStore()
const auth = useAuthStore()
const app = useAppStore()
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const canRead = computed(() => Boolean(workspaceId.value && auth.user?.id && store.can('webhook.read')))
const webhooks = ref<WorkspaceWebhook[]>([])
const deliveries = ref<WorkspaceWebhookDelivery[]>([])
const selectedWebhookId = ref<number | null>(null)
const selectedWebhook = computed(() => webhooks.value.find(item => item.id === selectedWebhookId.value) || null)
const deliveryPage = ref(1)
const deliveryPages = ref(1)
const deliveryTotal = ref(0)
const loading = ref(false)
const deliveriesLoading = ref(false)
const listError = ref(false)
const deliveryError = ref(false)
const busy = ref('')
const formOpen = ref(false)
const editing = ref<WorkspaceWebhook | null>(null)
const secret = ref('')
const secretName = ref('')
const form = reactive({ name: '', url: '', eventTypes: [] as string[], enabled: true })
const fieldErrors = reactive({ name: false, url: false, events: false })
let contextGeneration = 0
let generation = 0
let controller: AbortController | null = null
let deliveryGeneration = 0
let deliveryController: AbortController | null = null

function current(context: number, permission = 'webhook.read') { return context === contextGeneration && canRead.value && store.can(permission) }
function canMutate(permission: string) { return canRead.value && store.can(permission) && !busy.value }
function clearSecret() { secret.value = ''; secretName.value = '' }
function resetForm() { Object.assign(form, { name: '', url: '', eventTypes: [], enabled: true }); Object.assign(fieldErrors, { name: false, url: false, events: false }) }
function openCreate() { if (!canMutate('webhook.create')) return; editing.value = null; resetForm(); clearSecret(); formOpen.value = true }
function openEdit(webhook: WorkspaceWebhook) { if (!canMutate('webhook.update')) return; resetForm(); editing.value = webhook; clearSecret(); Object.assign(form, { name: webhook.name, url: webhook.url, eventTypes: [...webhook.event_types], enabled: webhook.enabled }); formOpen.value = true }
function closeForm() { formOpen.value = false; editing.value = null; resetForm() }
function canceled(error: unknown) { return (error as { code?: string })?.code === 'ERR_CANCELED' }
function failure(error: unknown) { app.showError((error as { message?: string })?.message || t('workspace.webhookError')) }

async function load() {
  const context = contextGeneration
  const id = workspaceId.value
  const request = ++generation
  controller?.abort()
  listError.value = false
  if (!canRead.value) { loading.value = false; return }
  controller = new AbortController()
  loading.value = true
  try {
    const result = await webhooksAPI.list(id, { signal: controller.signal })
    if (request !== generation || !current(context)) return
    webhooks.value = Array.isArray(result) ? result : result.items || []
    if (!webhooks.value.some(item => item.id === selectedWebhookId.value)) selectedWebhookId.value = webhooks.value[0]?.id || null
  } catch (error) { if (request === generation && current(context) && !canceled(error)) listError.value = true }
  finally { if (request === generation) loading.value = false }
}

async function loadDeliveries() {
  const context = contextGeneration
  const id = workspaceId.value
  const webhookId = selectedWebhookId.value
  const requestedPage = deliveryPage.value
  const request = ++deliveryGeneration
  deliveryController?.abort()
  deliveryError.value = false
  if (!canRead.value || !webhookId || !store.can('webhook.delivery.read')) { deliveries.value = []; deliveriesLoading.value = false; return }
  deliveryController = new AbortController()
  deliveriesLoading.value = true
  try {
    const result = await webhooksAPI.listDeliveries(id, webhookId, { page: requestedPage, page_size: 20, signal: deliveryController.signal })
    if (request !== deliveryGeneration || !current(context, 'webhook.delivery.read') || webhookId !== selectedWebhookId.value) return
    deliveries.value = result.items || []
    deliveryPage.value = result.page || requestedPage
    deliveryPages.value = Math.max(1, result.pages || 1)
    deliveryTotal.value = result.total || 0
  } catch (error) { if (request === deliveryGeneration && current(context, 'webhook.delivery.read') && !canceled(error)) deliveryError.value = true }
  finally { if (request === deliveryGeneration) deliveriesLoading.value = false }
}

function changeDeliveryPage(delta: number) {
  const page = deliveryPage.value + delta
  if (deliveriesLoading.value || busy.value || page < 1 || page > deliveryPages.value) return
  deliveryPage.value = page; deliveries.value = []; void loadDeliveries()
}

async function save() {
  const permission = editing.value ? 'webhook.update' : 'webhook.create'
  if (!canMutate(permission)) return
  const id = workspaceId.value
  const context = contextGeneration
  const webhookId = editing.value?.id
  const payload = { name: form.name.trim(), url: form.url.trim(), event_types: [...new Set(form.eventTypes)] }
  fieldErrors.name = !payload.name
  try { const url = new URL(payload.url); fieldErrors.url = url.protocol !== 'https:' || Boolean(url.username || url.password || url.hash) }
  catch { fieldErrors.url = true }
  fieldErrors.events = !payload.event_types.length || payload.event_types.some(event => !allowedEvents.includes(event))
  if (fieldErrors.name || fieldErrors.url || fieldErrors.events) return
  busy.value = 'save'
  clearSecret()
  try {
    if (webhookId) {
      await webhooksAPI.update(id, webhookId, { ...payload, enabled: form.enabled })
      if (!current(context, permission)) return
    } else {
      const result = await webhooksAPI.create(id, payload)
      if (!current(context, permission)) return
      secret.value = result.secret || ''; secretName.value = result.webhook.name
    }
    closeForm(); await load()
    if (current(context, permission)) app.showSuccess(t('workspace.webhookSaved'))
  } catch (error) { if (current(context, permission)) failure(error) }
  finally { if (context === contextGeneration) busy.value = '' }
}

async function rotate(webhook: WorkspaceWebhook) {
  const permission = 'webhook.secret.rotate'
  if (!canMutate(permission)) return
  const context = contextGeneration
  const id = workspaceId.value
  busy.value = `rotate-${webhook.id}`; clearSecret()
  try {
    const result = await webhooksAPI.rotateSecret(id, webhook.id)
    if (!current(context, permission)) return
    secret.value = result.secret || ''; secretName.value = result.webhook.name
    await load()
    if (current(context, permission)) app.showSuccess(t('workspace.webhookSecretRotated'))
  } catch (error) { if (current(context, permission)) failure(error) }
  finally { if (context === contextGeneration) busy.value = '' }
}

async function sendTest(webhookId: number) {
  if (!canMutate('webhook.test')) return
  const context = contextGeneration
  const id = workspaceId.value
  busy.value = `test-${webhookId}`
  try {
    await webhooksAPI.test(id, webhookId)
    if (!current(context, 'webhook.test')) return
    if (selectedWebhookId.value !== webhookId) selectedWebhookId.value = webhookId
    else { deliveryPage.value = 1; await loadDeliveries() }
    if (current(context, 'webhook.test')) app.showSuccess(t('workspace.webhookTestSent'))
  } catch (error) { if (current(context, 'webhook.test')) failure(error) }
  finally { if (context === contextGeneration) busy.value = '' }
}

async function remove(webhookId: number) {
  if (!canMutate('webhook.delete') || !window.confirm(t('workspace.confirmDeleteWebhook'))) return
  const context = contextGeneration
  const id = workspaceId.value
  busy.value = `delete-${webhookId}`; clearSecret()
  try {
    await webhooksAPI.remove(id, webhookId)
    if (!current(context, 'webhook.delete')) return
    await load()
    if (current(context, 'webhook.delete')) app.showSuccess(t('workspace.webhookDeleted'))
  } catch (error) { if (current(context, 'webhook.delete')) failure(error) }
  finally { if (context === contextGeneration) busy.value = '' }
}

function retryable(status: string) { return status === 'dead' }
async function retry(deliveryId: number) {
  const webhookId = selectedWebhookId.value
  if (!canMutate('webhook.delivery.retry') || !store.can('webhook.delivery.read') || !webhookId || !deliveries.value.some(item => item.id === deliveryId && retryable(item.status))) return
  const context = contextGeneration
  const id = workspaceId.value
  busy.value = `retry-${deliveryId}`
  try {
    await webhooksAPI.retry(id, webhookId, deliveryId)
    if (current(context, 'webhook.delivery.retry') && webhookId === selectedWebhookId.value) await loadDeliveries()
  } catch (error) { if (current(context, 'webhook.delivery.retry')) failure(error) }
  finally { if (context === contextGeneration) busy.value = '' }
}

async function copySecret() {
  if (!secret.value) return
  const context = contextGeneration
  try { await navigator.clipboard.writeText(secret.value); if (current(context)) app.showSuccess(t('common.copied')) }
  catch { if (current(context)) app.showError(t('common.copyFailed')) }
}
function formatDate(value?: string | null) { const date = value ? new Date(value) : null; return date && !Number.isNaN(date.getTime()) ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(date) : '--' }
function deliveryStatus(value: string) { return te(`workspace.${value}`) ? t(`workspace.${value}`) : value }

function resetContext() {
  ++contextGeneration; ++generation; ++deliveryGeneration
  controller?.abort(); deliveryController?.abort()
  webhooks.value = []; deliveries.value = []; selectedWebhookId.value = null
  deliveryPage.value = 1; deliveryPages.value = 1; deliveryTotal.value = 0
  loading.value = false; deliveriesLoading.value = false; busy.value = ''; listError.value = false; deliveryError.value = false
  clearSecret(); closeForm(); void load()
}
onMounted(load)
watch([workspaceId, () => auth.user?.id, () => store.permissions.join('\0')], resetContext, { flush: 'sync' })
watch(selectedWebhookId, () => { deliveryPage.value = 1; deliveryPages.value = 1; deliveryTotal.value = 0; deliveries.value = []; void loadDeliveries() })
onBeforeUnmount(() => { ++contextGeneration; ++generation; ++deliveryGeneration; controller?.abort(); deliveryController?.abort(); clearSecret() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-panel__heading > * { min-width: 0; }
.workspace-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.workspace-panel p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.workspace-form { display: grid; gap: 14px; max-width: 720px; margin-top: 18px; }
.workspace-form label { display: grid; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }
.workspace-form .workspace-checkbox, .workspace-checkbox { display: flex; align-items: center; gap: 8px; min-height: 36px; }
.workspace-checkbox input { width: 16px; height: 16px; flex: 0 0 16px; accent-color: var(--color-primary); }
.workspace-checkbox code { min-width: 0; overflow-wrap: anywhere; }
.workspace-events { min-width: 0; border: 1px solid var(--color-border); border-radius: 8px; padding: 12px; }
.workspace-events legend { color: var(--color-text-secondary); padding: 0 5px; font-size: 13px; }
.workspace-event-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); max-height: 260px; overflow-y: auto; gap: 4px 12px; }
.workspace-hint { color: var(--color-text-muted); font-size: 12px; }
.workspace-field-error { color: var(--color-danger); font-size: 13px; }
.workspace-actions { display: flex; min-width: 0; flex-wrap: wrap; gap: 7px; }
.workspace-token { display: flex; min-width: 0; align-items: center; flex-wrap: wrap; gap: 10px; margin-top: 18px; border: 1px solid var(--color-primary-border); border-radius: 8px; background: var(--color-primary-soft); padding: 12px; }
.workspace-token > div { flex: 1 1 180px; min-width: 0; }
.workspace-token p { margin: 4px 0 0; color: var(--color-text-secondary); font-size: 12px; }
.workspace-token code, .workspace-breakable { min-width: 0; overflow-wrap: anywhere; }
.workspace-table-wrap { min-width: 0; overflow-x: auto; margin-top: 18px; }
.workspace-table { width: 100%; min-width: 980px; border-collapse: collapse; }
.workspace-table--deliveries { min-width: 860px; }
.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; vertical-align: top; }
.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }
.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }
.workspace-table td:first-child { color: var(--color-text-primary); }
.workspace-table small { display: block; margin-top: 3px; color: var(--color-text-muted); }
.workspace-status { border-radius: 999px; background: var(--color-surface-soft); padding: 3px 8px; font-size: 12px; white-space: nowrap; }
.workspace-chips { display: flex; min-width: 180px; flex-wrap: wrap; gap: 4px; }
.workspace-chip { border: 1px solid var(--color-border-subtle); border-radius: 6px; background: var(--color-surface-soft); padding: 3px 6px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; overflow-wrap: anywhere; }
.workspace-danger { color: var(--color-danger); }
.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
.workspace-error, .workspace-pagination { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; color: var(--color-danger); font-size: 13px; }
.workspace-pagination { color: var(--color-text-secondary); }
.workspace-response { min-width: 220px; max-width: 360px; }
.workspace-response details { margin-top: 6px; }
.workspace-response summary { color: var(--color-primary); cursor: pointer; }
.workspace-response pre { white-space: pre-wrap; overflow-wrap: anywhere; font-size: 12px; }
.workspace-delivery-select { max-width: 260px; }
@media (max-width: 640px) {
  .workspace-panel__heading { align-items: stretch; flex-direction: column; }
  .workspace-event-options { grid-template-columns: minmax(0, 1fr); }
  .workspace-checkbox { min-height: 44px; }
}
</style>
