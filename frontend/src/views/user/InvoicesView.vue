<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="card p-4 sm:p-5" :aria-busy="eligibilityLoading">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0 space-y-1">
            <h2 class="text-base font-semibold">{{ t('invoices.quotaTitle') }}</h2>
            <p class="max-w-2xl text-sm text-[var(--color-text-secondary)]">{{ t('invoices.rules', { minimum: formatInvoiceAmount(minimum) }) }}</p>
          </div>
          <button class="btn btn-primary" data-testid="new-invoice" :disabled="!canApply || eligibilityLoading" @click="openApplication">
            {{ t('invoices.apply') }}
          </button>
        </div>
        <dl class="mt-5 grid gap-4 sm:grid-cols-3">
          <div v-for="metric in metrics" :key="metric.key" class="min-w-0">
            <dt class="text-sm text-[var(--color-text-secondary)]">{{ t(`invoices.${metric.key}`) }}</dt>
            <dd class="mt-2 text-2xl font-semibold tabular-nums" :class="metric.key === 'available' ? 'text-[var(--color-primary)]' : ''">
              {{ eligibility ? formatInvoiceAmount(metric.value) : '—' }}
            </dd>
          </div>
        </dl>
        <p class="mt-4 text-xs leading-relaxed text-[var(--color-text-muted)]">{{ t('invoices.quotaNote') }}</p>
        <div v-if="eligibilityError" class="mt-4 flex flex-wrap items-center gap-3 text-sm text-[var(--color-danger)]" role="alert">
          <span>{{ eligibilityError }}</span>
          <button class="btn btn-secondary" :disabled="eligibilityLoading" @click="loadEligibility">{{ t('invoices.retry') }}</button>
        </div>
        <p v-else-if="blockedReason" class="mt-4 rounded-lg bg-[var(--color-surface-soft)] p-3 text-sm text-[var(--color-text-secondary)]" role="status">{{ blockedReason }}</p>
      </section>

      <section class="min-w-0 space-y-3" :aria-label="t('invoices.records')">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-base font-semibold">{{ t('invoices.records') }}</h2>
          <button class="btn btn-secondary" :disabled="loading || eligibilityLoading" @click="refresh">{{ t('common.refresh') }}</button>
        </div>
        <p v-if="listError" class="text-sm text-[var(--color-danger)]" role="alert">{{ listError }}</p>
        <InvoiceTable :items="items" :loading="loading">
          <template #actions="{ row }">
            <div class="flex items-center gap-2 whitespace-nowrap">
              <button class="btn btn-secondary btn-sm" :disabled="detailLoading" @click="openDetails(row.id)">{{ t('invoices.details') }}</button>
              <button v-if="row.status === 'pending'" class="btn btn-secondary btn-sm" @click="cancelTarget = row">{{ t('invoices.cancel') }}</button>
            </div>
          </template>
        </InvoiceTable>
        <Pagination v-if="total > 0" :page="page" :total="total" :page-size="pageSize" @update:page="changePage" @update:pageSize="changePageSize" />
      </section>
    </div>

    <BaseDialog :show="showApplication" :title="t('invoices.apply')" width="wide" :show-close-button="!submitting" :close-on-escape="!submitting" @close="showApplication = false">
      <form id="invoice-application" class="space-y-5" @submit.prevent="submitApplication">
        <p class="text-sm text-[var(--color-text-secondary)]">{{ t('invoices.applicationHelp') }}</p>
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label for="invoice-amount" class="input-label">{{ t('invoices.fields.amount') }} <span aria-hidden="true">*</span></label>
            <input id="invoice-amount" v-model="amount" class="input mt-1" type="text" inputmode="decimal" required :aria-invalid="!!amountError" aria-describedby="invoice-amount-help invoice-amount-error" data-testid="invoice-amount" />
            <p id="invoice-amount-help" class="mt-1 text-xs text-[var(--color-text-muted)]">{{ t('invoices.amountHelp', { minimum: formatInvoiceAmount(minimum), available: formatInvoiceAmount(eligibility?.available_amount_cents ?? 0) }) }}</p>
            <p v-if="amountError" id="invoice-amount-error" class="mt-1 text-sm text-[var(--color-danger)]" role="alert">{{ amountError }}</p>
          </div>
          <div>
            <label for="invoice-type" class="input-label">{{ t('invoices.fields.invoice_type') }}</label>
            <select id="invoice-type" v-model="form.invoice_type" class="input mt-1" data-testid="invoice-type">
              <option value="vat_normal">{{ t('invoices.types.vat_normal') }}</option>
              <option value="vat_special">{{ t('invoices.types.vat_special') }}</option>
            </select>
          </div>
          <div>
            <label for="invoice-title-type" class="input-label">{{ t('invoices.fields.title_type') }}</label>
            <select id="invoice-title-type" v-model="form.title_type" class="input mt-1" :disabled="isSpecial" data-testid="invoice-title-type">
              <option value="personal">{{ t('invoices.titleTypes.personal') }}</option>
              <option value="enterprise">{{ t('invoices.titleTypes.enterprise') }}</option>
            </select>
            <p v-if="isSpecial" class="mt-1 text-xs text-[var(--color-text-muted)]">{{ t('invoices.specialHelp') }}</p>
          </div>
          <div>
            <label for="invoice-title" class="input-label">{{ t('invoices.fields.title') }} <span aria-hidden="true">*</span></label>
            <input id="invoice-title" v-model="form.title" class="input mt-1" maxlength="200" required data-testid="invoice-title" />
          </div>
          <div v-if="form.title_type === 'enterprise'">
            <label for="invoice-tax" class="input-label">{{ t('invoices.fields.tax_number') }} <span aria-hidden="true">*</span></label>
            <input id="invoice-tax" v-model="form.tax_number" class="input mt-1" maxlength="32" required data-testid="invoice-tax" />
          </div>
          <div class="sm:col-span-2">
            <label for="invoice-email" class="input-label">{{ t('invoices.fields.email') }} <span aria-hidden="true">*</span></label>
            <input id="invoice-email" v-model="form.email" type="email" class="input mt-1" maxlength="254" required data-testid="invoice-email" />
          </div>
          <template v-if="form.title_type === 'enterprise'">
            <div v-for="field in enterpriseFields" :key="field.key" :class="field.key === 'address' ? 'sm:col-span-2' : ''">
              <label :for="`invoice-${field.key}`" class="input-label">{{ t(`invoices.fields.${field.key}`) }} <span v-if="isSpecial" aria-hidden="true">*</span></label>
              <input :id="`invoice-${field.key}`" v-model="form[field.key]" class="input mt-1" :type="field.key === 'phone' ? 'tel' : 'text'" :maxlength="field.max" :required="isSpecial" />
            </div>
          </template>
          <div class="sm:col-span-2">
            <label for="invoice-remark" class="input-label">{{ t('invoices.fields.remark') }}</label>
            <textarea id="invoice-remark" v-model="form.remark" class="input mt-1" rows="3" maxlength="1000" />
          </div>
        </div>
        <p v-if="formError" ref="formErrorElement" tabindex="-1" class="rounded-lg bg-[var(--color-surface-soft)] p-3 text-sm text-[var(--color-danger)]" role="alert">{{ formError }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="submitting" @click="showApplication = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="submit" form="invoice-application" :disabled="submitting || !canApply" data-testid="submit-invoice">{{ submitting ? t('common.processing') : t('invoices.submit') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!selected" :title="t('invoices.details')" width="wide" @close="selected = null">
      <InvoiceDetails v-if="selected" :invoice="selected" />
    </BaseDialog>

    <BaseDialog :show="!!cancelTarget" :title="t('invoices.cancel')" width="narrow" :show-close-button="!cancelling" :close-on-escape="!cancelling" @close="cancelTarget = null">
      <p class="text-sm text-[var(--color-text-secondary)]">{{ t('invoices.cancelConfirm') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="cancelling" @click="cancelTarget = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" :disabled="cancelling" data-testid="confirm-cancel-invoice" @click="cancelApplication">{{ cancelling ? t('common.processing') : t('invoices.cancel') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import InvoiceDetails from '@/components/invoices/InvoiceDetails.vue'
import InvoiceTable from '@/components/invoices/InvoiceTable.vue'
import { invoicesAPI } from '@/api/invoices'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatInvoiceAmount, parseInvoiceAmount } from '@/utils/invoice'
import type { InvoiceApplication, InvoiceApplicationInput, InvoiceEligibility } from '@/types/invoice'

const { t } = useI18n()
const appStore = useAppStore()
const eligibility = ref<InvoiceEligibility | null>(null)
const eligibilityLoading = ref(true)
const eligibilityError = ref('')
const loading = ref(true)
const listError = ref('')
const items = ref<InvoiceApplication[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
let listRequest = 0
const selected = ref<InvoiceApplication | null>(null)
const detailLoading = ref(false)
const cancelTarget = ref<InvoiceApplication | null>(null)
const cancelling = ref(false)
const showApplication = ref(false)
const submitting = ref(false)
const formError = ref('')
const formErrorElement = ref<HTMLElement | null>(null)
const amount = ref('')
const amountError = ref('')
const freshForm = (): InvoiceApplicationInput => ({ amount_cents: 0, invoice_type: 'vat_normal', title_type: 'enterprise', title: '', tax_number: '', email: '', address: '', phone: '', bank_name: '', bank_account: '', remark: '' })
const form = reactive(freshForm())
const enterpriseFields = [{ key: 'address', max: 500 }, { key: 'phone', max: 50 }, { key: 'bank_name', max: 200 }, { key: 'bank_account', max: 100 }] as const
const minimum = computed(() => eligibility.value?.minimum_amount_cents ?? 30000)
const canApply = computed(() => !!eligibility.value?.enabled && eligibility.value.total_recharged_cents >= minimum.value && eligibility.value.available_amount_cents >= minimum.value)
const isSpecial = computed(() => form.invoice_type === 'vat_special')
const metrics = computed(() => [
  { key: 'totalRecharged', value: eligibility.value?.total_recharged_cents ?? 0 },
  { key: 'reserved', value: eligibility.value?.reserved_amount_cents ?? 0 },
  { key: 'available', value: eligibility.value?.available_amount_cents ?? 0 },
])
const blockedReason = computed(() => {
  if (!eligibility.value) return ''
  if (!eligibility.value.enabled) return t('invoices.disabled')
  if (eligibility.value.total_recharged_cents < minimum.value) return t('invoices.insufficientRecharge', { minimum: formatInvoiceAmount(minimum.value) })
  if (eligibility.value.available_amount_cents < minimum.value) return t('invoices.insufficientAvailable', { minimum: formatInvoiceAmount(minimum.value) })
  return ''
})
watch(isSpecial, value => { if (value) form.title_type = 'enterprise' })
watch(amount, () => { amountError.value = '' })

async function loadEligibility() {
  eligibilityLoading.value = true
  eligibilityError.value = ''
  try { eligibility.value = (await invoicesAPI.eligibility()).data }
  catch (error) {
    eligibility.value = null
    eligibilityError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed'))
  } finally { eligibilityLoading.value = false }
}

async function loadRecords() {
  const request = ++listRequest
  loading.value = true
  listError.value = ''
  try {
    const { data } = await invoicesAPI.list({ page: page.value, page_size: pageSize.value })
    if (request !== listRequest) return
    items.value = data.items || []
    total.value = data.total
  } catch (error) {
    if (request === listRequest) listError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed'))
  } finally { if (request === listRequest) loading.value = false }
}

async function refresh() { await Promise.all([loadEligibility(), loadRecords()]) }
function changePage(value: number) { page.value = value; void loadRecords() }
function changePageSize(value: number) { pageSize.value = value; changePage(1) }
function openApplication() {
  if (!canApply.value) return
  Object.assign(form, freshForm())
  amount.value = ''
  formError.value = ''
  amountError.value = ''
  showApplication.value = true
}

async function submitApplication() {
  if (submitting.value || !canApply.value) return
  const cents = parseInvoiceAmount(amount.value)
  if (cents === null || cents < minimum.value || cents > (eligibility.value?.available_amount_cents ?? 0)) {
    amountError.value = t('invoices.amountInvalid', { minimum: formatInvoiceAmount(minimum.value), available: formatInvoiceAmount(eligibility.value?.available_amount_cents ?? 0) })
    await nextTick()
    document.getElementById('invoice-amount')?.focus()
    return
  }
  submitting.value = true
  formError.value = ''
  try {
    const payload: InvoiceApplicationInput = { ...form, amount_cents: cents }
    for (const key of ['title', 'tax_number', 'email', 'address', 'phone', 'bank_name', 'bank_account', 'remark'] as const) payload[key] = payload[key].trim()
    if (payload.title_type === 'personal') {
      payload.tax_number = ''; payload.address = ''; payload.phone = ''; payload.bank_name = ''; payload.bank_account = ''
    }
    await invoicesAPI.create(payload)
    showApplication.value = false
    appStore.showSuccess(t('invoices.submitted'))
    page.value = 1
    await refresh()
  } catch (error) {
    formError.value = extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.submitFailed'))
    await loadEligibility()
    await nextTick()
    formErrorElement.value?.focus()
  } finally { submitting.value = false }
}

async function openDetails(id: number) {
  if (detailLoading.value) return
  detailLoading.value = true
  try { selected.value = (await invoicesAPI.detail(id)).data }
  catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.loadFailed'))) }
  finally { detailLoading.value = false }
}

async function cancelApplication() {
  if (!cancelTarget.value || cancelling.value) return
  cancelling.value = true
  try {
    await invoicesAPI.cancel(cancelTarget.value.id)
    cancelTarget.value = null
    appStore.showSuccess(t('invoices.cancelled'))
    await refresh()
  } catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'invoices.errors', t('invoices.updateFailed'))) }
  finally { cancelling.value = false }
}

onMounted(refresh)
</script>
