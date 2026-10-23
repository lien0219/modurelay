<template>
  <section class="space-y-3 border-t border-[var(--color-border)] pt-5" :aria-label="t('invoices.documentTitle')" :aria-busy="loading">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="min-w-0">
        <h4 class="text-base font-semibold">{{ t('invoices.documentTitle') }}</h4>
        <p class="mt-1 break-words text-sm [overflow-wrap:anywhere]">{{ document.filename }}</p>
        <p class="mt-1 text-xs text-[var(--color-text-muted)]">{{ formatBytes(document.size_bytes) }} · {{ formatDateTime(document.uploaded_at) }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-secondary" :disabled="loading" data-testid="preview-invoice-pdf" @click="togglePreview">{{ previewURL ? t('invoices.hidePreview') : t('invoices.previewPdf') }}</button>
        <button class="btn btn-primary" :disabled="loading" data-testid="download-invoice-pdf" @click="downloadPdf">{{ t('invoices.downloadPdf') }}</button>
      </div>
    </div>
    <p v-if="error" class="text-sm text-[var(--color-danger)]" role="alert">{{ error }}</p>
    <template v-if="previewURL">
      <p class="text-xs text-[var(--color-text-secondary)]">{{ t('invoices.previewHelp') }}</p>
      <iframe :src="previewURL" :title="t('invoices.documentTitle')" class="h-[60vh] min-h-80 w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)]" data-testid="invoice-pdf-preview" />
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import type { InvoiceDocument } from '@/types/invoice'
import { formatDateTime } from '@/utils/format'
import { extractI18nErrorMessage } from '@/utils/apiError'

const props = defineProps<{ document: InvoiceDocument; loadDocument: (download?: boolean) => Promise<{ data: Blob }> }>()
const { t } = useI18n()
const previewURL = ref('')
const loading = ref(false)
const error = ref('')
let disposed = false

function formatBytes(size: number) { return `${(size / 1024).toFixed(1)} KB` }
function clearPreview() {
  if (previewURL.value) URL.revokeObjectURL(previewURL.value)
  previewURL.value = ''
}
async function togglePreview() {
  if (loading.value) return
  if (previewURL.value) { clearPreview(); return }
  loading.value = true
  error.value = ''
  try {
    const { data } = await props.loadDocument()
    if (!disposed) previewURL.value = URL.createObjectURL(new Blob([data], { type: 'application/pdf' }))
  } catch (cause) { if (!disposed) error.value = extractI18nErrorMessage(cause, t, 'invoices.errors', t('invoices.documentLoadFailed')) }
  finally { if (!disposed) loading.value = false }
}
async function downloadPdf() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    const { data } = await props.loadDocument(true)
    if (!disposed) saveAs(new Blob([data], { type: 'application/pdf' }), props.document.filename)
  } catch (cause) { if (!disposed) error.value = extractI18nErrorMessage(cause, t, 'invoices.errors', t('invoices.documentLoadFailed')) }
  finally { if (!disposed) loading.value = false }
}
onBeforeUnmount(() => { disposed = true; clearPreview() })
</script>
