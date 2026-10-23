<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="card flex flex-col items-stretch justify-between gap-4 p-4 sm:flex-row sm:items-center sm:p-5" :aria-busy="configLoading">
        <div class="min-w-0 flex-1 space-y-1">
          <h2 class="text-base font-semibold">{{ t('invoices.visibilityTitle') }}</h2>
          <p class="max-w-2xl text-sm text-[var(--color-text-secondary)]">{{ t('invoices.visibilityHelp') }}</p>
          <p v-if="configError" class="text-sm text-[var(--color-danger)]" role="alert">{{ configError }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-4">
          <label class="flex min-h-11 cursor-pointer items-center gap-2 text-sm">
            <input v-model="enabled" type="checkbox" class="h-4 w-4 accent-[var(--color-primary)]" :disabled="configLoading || savingConfig || !!configError" data-testid="invoice-enabled" />
            {{ t('invoices.enabled') }}
          </label>
          <button v-if="configError" class="btn btn-secondary" :disabled="configLoading" @click="loadConfig">{{ t('invoices.retry') }}</button>
          <button v-else class="btn btn-primary" :disabled="configLoading || savingConfig || enabled === savedEnabled" data-testid="save-invoice-config" @click="saveConfig">{{ savingConfig ? t('common.processing') : t('common.save') }}</button>
        </div>
      </section>

      <form class="glass-panel flex flex-wrap items-end gap-3 p-4" @submit.prevent="applyFilters">
        <div class="min-w-48 flex-1">
          <label for="invoice-search" class="input-label">{{ t('invoices.search') }}</label>
          <input id="invoice-search" v-model="searchInput" type="search" class="input mt-1" :placeholder="t('invoices.searchPlaceholder')" maxlength="200" />
        </div>
        <div class="w-full sm:w-40">
          <label for="invoice-status-filter" class="input-label">{{ t('invoices.fields.status') }}</label>
          <select id="invoice-status-filter" v-model="statusInput" class="input mt-1">
            <option value="">{{ t('invoices.allStatuses') }}</option>
            <option v-for="status in statuses" :key="status" :value="status">{{ t(`invoices.status.${status}`) }}</option>
          </select>
        </div>
        <button class="btn btn-primary" type="submit" :disabled="loading">{{ t('invoices.search') }}</button>
        <button class="btn btn-secondary" type="button" :disabled="loading" @click="loadRecords">{{ t('common.refresh') }}</button>
        <button class="btn btn-secondary" type="button" :disabled="exporting" data-testid="export-invoices" @click="exportRecords">{{ exporting ? t('common.processing') : t('invoices.export') }}</button>
        <p class="w-full text-xs text-[var(--color-text-muted)]">{{ t('invoices.exportHelp') }}</p>
      </form>

      <section class="min-w-0 space-y-3" :aria-label="t('invoices.records')">
        <p v-if="listError" class="text-sm text-[var(--color-danger)]" role="alert">{{ listError }}</p>
        <InvoiceTable :items="items" :loading="loading" show-user>
          <template #actions="{ row }">
            <button class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="detailLoading" @click="openDetails(row.id)">{{ t('invoices.details') }}</button>
          </template>
        </InvoiceTable>
        <Pagination v-if="total > 0" :page="page" :total="total" :page-size="pageSize" @update:page="changePage" @update:pageSize="changePageSize" />
      </section>
    </div>

    <BaseDialog :show="!!selected" :title="t('invoices.details')" width="wide" :show-close-button="!updating" :close-on-escape="!updating" @close="selected = null">
      <div v-if="selected" class="space-y-6">
        <InvoiceDetails :invoice="selected" show-user />
        <form v-if="canProcess" id="invoice-processing" class="space-y-4 border-t border-[var(--color-border)] pt-5" @submit.prevent="updateInvoice">
          <h4 class="text-base font-semibold">{{ t('invoices.process') }}</h4>
          <div>
            <label for="invoice-next-status" class="input-label">{{ t('invoices.nextStatus') }}</label>
            <select id="invoice-next-status" v-model="updateForm.status" class="input mt-1" data-testid="invoice-next-status">
              <option v-if="selected.status === 'pending'" value="processing">{{ t('invoices.status.processing') }}</option>
              <option value="issued">{{ t('invoices.status.issued') }}</option>
              <option value="rejected">{{ t('invoices.status.rejected') }}</option>
            </select>
          </div>
          <div v-if="updateForm.status === 'issued'">
            <label for="invoice-number" class="input-label">{{ t('invoices.fields.invoice_number') }} <span aria-hidden="true">*</span></label>
            <input id="invoice-number" v-model="updateForm.invoice_number" class="input mt-1" maxlength="100" required aria-describedby="invoice-number-help" />
            <p id="invoice-number-help" class="mt-1 text-xs text-[var(--color-text-muted)]">{{ t('invoices.invoiceNumberHelp') }}</p>
          </div>
          <div>
            <label for="invoice-admin-remark" class="input-label">{{ t('invoices.fields.admin_remark') }} <span v-if="updateForm.status === 'rejected'" aria-hidden="true">*</span></label>
            <textarea id="invoice-admin-remark" v-model="updateForm.admin_remark" class="input mt-1" rows="3" maxlength="1000" :required="updateForm.status === 'rejected'" aria-describedby="invoice-admin-remark-help" />
            <p id="invoice-admin-remark-help" class="mt-1 text-xs text-[var(--color-text-muted)]">{{ t('invoices.adminRemarkHelp') }}</p>
          </div>
          <p v-if="updateError" ref="updateErrorElement" class="text-sm text-[var(--color-danger)]" role="alert" tabindex="-1">{{ updateError }}</p>
        </form>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="updating" @click="selected = null">{{ t('common.close') }}</button>
          <button v-if="canProcess" class="btn btn-primary" type="submit" form="invoice-processing" :disabled="updating" data-testid="update-invoice">{{ updating ? t('common.processing') : t('invoices.saveStatus') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import InvoiceDetails from '@/components/invoices/InvoiceDetails.vue'
import InvoiceTable from '@/components/invoices/InvoiceTable.vue'
import { adminInvoicesAPI } from '@/api/invoices'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { InvoiceApplication, InvoiceStatus, InvoiceUpdateInput } from '@/types/invoice'

const { t } = useI18n()
const appStore = useAppStore()
const configLoading = ref(true)
const configError = ref('')
const savingConfig = ref(false)
const enabled = ref(false)
const savedEnabled = ref(false)
const loading = ref(true)
const listError = ref('')
const items = ref<InvoiceApplication[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const searchInput = ref('')
const statusInput = ref<InvoiceStatus | ''>('')
const filters = reactive<{ status: InvoiceStatus | ''; search: string }>({ status: '', search: '' })
const statuses: InvoiceStatus[] = ['pending', 'processing', 'issued', 'rejected', 'cancelled']
let listRequest = 0
const exporting = ref(false)
const selected = ref<InvoiceApplication | null>(null)
const detailLoading = ref(false)
const updating = ref(false)
const updateError = ref('')
const updateErrorElement = ref<HTMLElement | null>(null)
const updateForm = reactive<InvoiceUpdateInput>({ status: 'processing', admin_remark: '', invoice_number: '' })
const canProcess = computed(() => selected.value?.status === 'pending' || selected.value?.status === 'processing')

async function loadConfig() {
  configLoading.value = true
  configError.value = ''
  try {
    const { data } = await adminInvoicesAPI.config()
    enabled.value = savedEnabled.value = data.enabled
  } catch (error) { configError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed')) }
  finally { configLoading.value = false }
}

async function saveConfig() {
  if (savingConfig.value) return
  savingConfig.value = true
  try {
    const { data } = await adminInvoicesAPI.updateConfig(enabled.value)
    enabled.value = savedEnabled.value = data.enabled
    if (appStore.cachedPublicSettings) appStore.cachedPublicSettings.invoice_enabled = data.enabled
    await appStore.fetchPublicSettings(true)
    // A public-settings request started before this save may have been reused.
    if (appStore.cachedPublicSettings) appStore.cachedPublicSettings.invoice_enabled = data.enabled
    appStore.showSuccess(t('invoices.configSaved'))
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.updateFailed'))) }
  finally { savingConfig.value = false }
}

async function loadRecords() {
  const request = ++listRequest
  loading.value = true
  listError.value = ''
  try {
    const { data } = await adminInvoicesAPI.list({ ...filters, page: page.value, page_size: pageSize.value })
    if (request !== listRequest) return
    items.value = data.items || []
    total.value = data.total
  } catch (error) {
    if (request === listRequest) listError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed'))
  } finally { if (request === listRequest) loading.value = false }
}

function applyFilters() { filters.search = searchInput.value.trim(); filters.status = statusInput.value; changePage(1) }
function changePage(value: number) { page.value = value; void loadRecords() }
function changePageSize(value: number) { pageSize.value = value; changePage(1) }

async function exportRecords() {
  if (exporting.value) return
  exporting.value = true
  try {
    const { data } = await adminInvoicesAPI.export({ ...filters })
    saveAs(data, `invoice-applications-${new Date().toISOString().slice(0, 10)}.csv`)
    appStore.showSuccess(t('invoices.exported'))
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.exportFailed'))) }
  finally { exporting.value = false }
}

async function openDetails(id: number) {
  if (detailLoading.value) return
  detailLoading.value = true
  updateError.value = ''
  try {
    selected.value = (await adminInvoicesAPI.detail(id)).data
    updateForm.status = selected.value.status === 'pending' ? 'processing' : 'issued'
    updateForm.admin_remark = selected.value.admin_remark || ''
    updateForm.invoice_number = selected.value.invoice_number || ''
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed'))) }
  finally { detailLoading.value = false }
}

async function updateInvoice() {
  if (!selected.value || !canProcess.value || updating.value) return
  const payload = { ...updateForm, admin_remark: updateForm.admin_remark.trim(), invoice_number: updateForm.invoice_number.trim() }
  if ((payload.status === 'issued' && !payload.invoice_number) || (payload.status === 'rejected' && !payload.admin_remark)) {
    updateError.value = t('invoices.processingRequired')
    await nextTick()
    updateErrorElement.value?.focus()
    return
  }
  updating.value = true
  updateError.value = ''
  try {
    selected.value = (await adminInvoicesAPI.update(selected.value.id, payload)).data
    if (selected.value.status === 'processing') updateForm.status = 'issued'
    appStore.showSuccess(t('invoices.updated'))
    await loadRecords()
  } catch (error) {
    updateError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.updateFailed'))
    await nextTick()
    updateErrorElement.value?.focus()
  } finally { updating.value = false }
}

onMounted(() => { void loadConfig(); void loadRecords() })
</script>
