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
        <button class="btn btn-secondary" type="button" :disabled="exporting" data-testid="export-invoices" @click="openExport">{{ exporting ? t('common.processing') : t('invoices.export') }}</button>
        <p class="w-full text-xs text-[var(--color-text-muted)]">{{ t('invoices.exportHelp') }}</p>
      </form>

      <section class="min-w-0 space-y-3" :aria-label="t('invoices.records')">
        <p v-if="listError" class="text-sm text-[var(--color-danger)]" role="alert">{{ listError }}</p>
        <div class="flex flex-wrap items-center gap-3 text-sm" role="status">
          <span>{{ t('invoices.selectedCount', { count: selectedIds.length }) }}</span>
          <button v-if="selectedIds.length" class="btn btn-secondary btn-sm" @click="selectedIds = []">{{ t('invoices.clearSelection') }}</button>
        </div>
        <InvoiceTable v-model:selected-keys="selectedIds" :items="items" :loading="loading" show-user selectable @detail="openDetails">
          <template #actions="{ row }">
            <button class="btn btn-secondary btn-sm whitespace-nowrap" :disabled="detailLoading" @click="openDetails(row.id)">{{ t('invoices.details') }}</button>
          </template>
        </InvoiceTable>
        <Pagination v-if="total > 0" :page="page" :total="total" :page-size="pageSize" @update:page="changePage" @update:pageSize="changePageSize" />
      </section>
    </div>

    <BaseDialog :show="!!selected" :title="t('invoices.details')" width="wide" :show-close-button="!updating && !uploading" :close-on-escape="!updating && !uploading" @close="selected = null">
      <div v-if="selected" class="space-y-6">
        <InvoiceDetails :invoice="selected" show-user />
        <InvoiceAccount :key="selected.id" :load="loadAccount" />
        <InvoiceDocument v-if="selected.document" :key="`${selected.id}-${selected.document.sha256}`" :document="selected.document" :load-document="loadDocument" />
        <section v-if="canUpload" class="space-y-3 border-t border-[var(--color-border)] pt-5" :aria-busy="uploading">
          <h4 class="text-base font-semibold">{{ t('invoices.uploadTitle') }}</h4>
          <p class="text-sm text-[var(--color-text-secondary)]">{{ selected.status === 'issued' ? t('invoices.backfillHelp') : t('invoices.uploadHelp') }}</p>
          <label for="invoice-pdf-file" class="input-label">{{ t('invoices.pdfFile') }}</label>
          <div class="flex flex-wrap items-center gap-3">
            <input id="invoice-pdf-file" :key="selected.id" ref="fileInput" type="file" accept=".pdf,application/pdf" class="input min-w-0 flex-1" :disabled="uploading || updating" aria-describedby="invoice-pdf-help" data-testid="invoice-pdf-file" @change="selectPdf" />
            <button class="btn btn-primary" :disabled="!pdfFile || uploading || updating" data-testid="upload-invoice-pdf" @click="uploadPdf">{{ uploading ? t('common.processing') : t('invoices.uploadPdf') }}</button>
          </div>
          <p id="invoice-pdf-help" class="text-xs text-[var(--color-text-muted)]">{{ t('invoices.pdfLimit') }}</p>
          <p v-if="uploadError" class="text-sm text-[var(--color-danger)]" role="alert">{{ uploadError }}</p>
        </section>
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
            <p v-if="!selected.document" class="mt-2 text-sm text-[var(--color-warning)]" role="status">{{ t('invoices.pdfRequired') }}</p>
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
          <button class="btn btn-secondary" :disabled="updating || uploading" @click="selected = null">{{ t('common.close') }}</button>
          <button v-if="canProcess" class="btn btn-primary" type="submit" form="invoice-processing" :disabled="updating || uploading || (updateForm.status === 'issued' && !selected?.document)" data-testid="update-invoice">{{ updating ? t('common.processing') : t('invoices.saveStatus') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showExport" :title="t('invoices.export')" width="wide" :show-close-button="!exporting" :close-on-escape="!exporting" @close="showExport = false">
      <form id="invoice-export" class="space-y-5" @submit.prevent="exportRecords">
        <fieldset class="space-y-2">
          <legend class="mb-2 text-sm font-semibold">{{ t('invoices.exportScope') }}</legend>
          <label class="flex min-h-11 items-center gap-2 text-sm"><input v-model="exportScope" type="radio" value="selected" class="accent-[var(--color-primary)]" :disabled="!selectedIds.length || exporting" data-testid="export-scope-selected" />{{ t('invoices.exportSelected', { count: selectedIds.length }) }}</label>
          <label class="flex min-h-11 items-center gap-2 text-sm"><input v-model="exportScope" type="radio" value="filtered" class="accent-[var(--color-primary)]" :disabled="exporting" data-testid="export-scope-filtered" />{{ t('invoices.exportFiltered', { count: total }) }}</label>
        </fieldset>
        <fieldset :disabled="exporting">
          <legend class="mb-3 text-sm font-semibold">{{ t('invoices.exportColumns') }}</legend>
          <div class="mb-3 flex flex-wrap gap-3">
            <button class="btn btn-secondary btn-sm" type="button" @click="exportColumns = [...INVOICE_EXPORT_COLUMNS]">{{ t('common.selectAll') }}</button>
            <button class="btn btn-secondary btn-sm" type="button" @click="exportColumns = []">{{ t('invoices.clearColumns') }}</button>
          </div>
          <div class="grid gap-x-5 sm:grid-cols-2">
            <label v-for="column in INVOICE_EXPORT_COLUMNS" :key="column" class="flex min-h-11 min-w-0 items-center gap-2 text-sm">
              <input v-model="exportColumns" :value="column" type="checkbox" class="h-4 w-4 accent-[var(--color-primary)]" :data-testid="`export-column-${column}`" />{{ t(`invoices.fields.${column}`) }}
            </label>
          </div>
        </fieldset>
        <p v-if="!exportColumns.length" class="text-sm text-[var(--color-danger)]" role="alert">{{ t('invoices.exportColumnRequired') }}</p>
        <p class="text-xs text-[var(--color-text-muted)]">{{ t('invoices.exportHelp') }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="exporting" @click="showExport = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="submit" form="invoice-export" :disabled="exporting || !exportColumns.length || (exportScope === 'selected' && !selectedIds.length)" data-testid="confirm-export-invoices">{{ exporting ? t('common.processing') : t('invoices.export') }}</button>
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
import InvoiceDocument from '@/components/invoices/InvoiceDocument.vue'
import InvoiceAccount from '@/components/invoices/InvoiceAccount.vue'
import InvoiceTable from '@/components/invoices/InvoiceTable.vue'
import { adminInvoicesAPI } from '@/api/invoices'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { INVOICE_EXPORT_COLUMNS, type InvoiceApplication, type InvoiceStatus, type InvoiceUpdateInput, type InvoiceExportColumn } from '@/types/invoice'

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
const selectedIds = ref<number[]>([])
const showExport = ref(false)
const exportScope = ref<'selected' | 'filtered'>('filtered')
const exportColumns = ref<InvoiceExportColumn[]>([...INVOICE_EXPORT_COLUMNS])
const selected = ref<InvoiceApplication | null>(null)
const detailLoading = ref(false)
const updating = ref(false)
const updateError = ref('')
const updateErrorElement = ref<HTMLElement | null>(null)
const updateForm = reactive<InvoiceUpdateInput>({ status: 'processing', admin_remark: '', invoice_number: '' })
const canProcess = computed(() => selected.value?.status === 'pending' || selected.value?.status === 'processing')
const canUpload = computed(() => canProcess.value || (selected.value?.status === 'issued' && !selected.value.document))
const uploading = ref(false)
const uploadError = ref('')
const pdfFile = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

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

function applyFilters() { filters.search = searchInput.value.trim(); filters.status = statusInput.value; selectedIds.value = []; changePage(1) }
function changePage(value: number) { page.value = value; void loadRecords() }
function changePageSize(value: number) { pageSize.value = value; changePage(1) }

function openExport() { exportScope.value = selectedIds.value.length ? 'selected' : 'filtered'; showExport.value = true }
async function exportRecords() {
  if (exporting.value || !exportColumns.value.length || (exportScope.value === 'selected' && !selectedIds.value.length)) return
  exporting.value = true
  try {
    const columns = INVOICE_EXPORT_COLUMNS.filter(column => exportColumns.value.includes(column))
    const scope = exportScope.value === 'selected' ? { ids: [...selectedIds.value] } : { ...filters }
    const { data } = await adminInvoicesAPI.export({ ...scope, columns })
    saveAs(data, `invoice-applications-${new Date().toISOString().slice(0, 10)}.csv`)
    showExport.value = false
    appStore.showSuccess(t('invoices.exported'))
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.exportFailed'))) }
  finally { exporting.value = false }
}

async function openDetails(id: number) {
  if (detailLoading.value) return
  detailLoading.value = true
  updateError.value = ''
  uploadError.value = ''
  pdfFile.value = null
  try {
    selected.value = (await adminInvoicesAPI.detail(id)).data
    updateForm.status = selected.value.status === 'pending' ? 'processing' : 'issued'
    updateForm.admin_remark = selected.value.admin_remark || ''
    updateForm.invoice_number = selected.value.invoice_number || ''
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed'))) }
  finally { detailLoading.value = false }
}

async function updateInvoice() {
  if (!selected.value || !canProcess.value || updating.value || uploading.value) return
  const payload = { ...updateForm, admin_remark: updateForm.admin_remark.trim(), invoice_number: updateForm.invoice_number.trim() }
  if ((payload.status === 'issued' && (!payload.invoice_number || !selected.value.document)) || (payload.status === 'rejected' && !payload.admin_remark)) {
    updateError.value = payload.status === 'issued' && !selected.value.document ? t('invoices.pdfRequired') : t('invoices.processingRequired')
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

function loadDocument(download = false) {
  if (!selected.value) return Promise.reject(new Error(t('invoices.documentLoadFailed')))
  return adminInvoicesAPI.document(selected.value.id, download)
}
function loadAccount(params: { page: number; page_size: number }) {
  if (!selected.value) return Promise.reject(new Error(t('invoices.loadFailed')))
  return adminInvoicesAPI.account(selected.value.id, params)
}
function selectPdf(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  uploadError.value = ''
  pdfFile.value = null
  if (!file) return
  if (!/\.pdf$/i.test(file.name) || file.size === 0 || file.size > 10 * 1024 * 1024) {
    uploadError.value = t('invoices.pdfInvalid')
    input.value = ''
    return
  }
  pdfFile.value = file
}
async function uploadPdf() {
  if (!selected.value || !pdfFile.value || !canUpload.value || uploading.value || updating.value) return
  uploading.value = true
  uploadError.value = ''
  try {
    selected.value = (await adminInvoicesAPI.uploadDocument(selected.value.id, pdfFile.value)).data
    pdfFile.value = null
    if (fileInput.value) fileInput.value.value = ''
    appStore.showSuccess(t('invoices.documentUploaded'))
    await loadRecords()
  } catch (error) { uploadError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.documentUploadFailed')) }
  finally { uploading.value = false }
}

onMounted(() => { void loadConfig(); void loadRecords() })
</script>
