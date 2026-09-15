<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="search" class="input min-w-56 flex-1 sm:max-w-80" :placeholder="t('planCatalog.search')" />
          <Select v-model="statusFilter" :options="statusOptions" class="w-40" />
          <div ref="columnMenuRef" class="relative">
            <button class="btn btn-secondary" :aria-expanded="showColumns" @click="showColumns = !showColumns">{{ t('planCatalog.columns') }}</button>
            <div v-if="showColumns" class="absolute right-0 z-30 mt-2 w-56 rounded-xl border border-gray-200 bg-white p-2 shadow-lg dark:border-dark-600 dark:bg-dark-800">
              <label v-for="column in toggleableColumns" :key="column.key" class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm hover:bg-gray-50 dark:hover:bg-dark-700">
                <input type="checkbox" :checked="isColumnVisible(column.key)" @change="toggleColumn(column.key)" />
                <span>{{ column.label }}</span>
              </label>
            </div>
          </div>
          <button class="btn btn-secondary" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
          <button class="btn btn-primary" @click="openCreate">{{ t('planCatalog.newPlan') }}</button>
        </div>
        <div v-if="selectedKeys.length" class="mt-3 flex flex-wrap items-center gap-2 rounded-xl border border-primary-200 bg-primary-50/70 px-3 py-2 text-sm dark:border-primary-900/60 dark:bg-primary-900/20">
          <span class="font-medium">{{ t('planCatalog.selectedCount', { count: selectedKeys.length }) }}</span>
          <button class="btn btn-secondary btn-sm" @click="batchPublish(true)">{{ t('planCatalog.batchPublish') }}</button>
          <button class="btn btn-secondary btn-sm" @click="batchPublish(false)">{{ t('planCatalog.batchUnpublish') }}</button>
          <button class="btn btn-danger btn-sm" @click="batchDelete">{{ t('planCatalog.batchDelete') }}</button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="pagedItems" :loading="loading" row-key="id" selectable :selected-keys="selectedKeys" :server-side-sort="true" default-sort-key="sort_order" @update:selected-keys="selectedKeys = $event" @sort="handleSort">
          <template #cell-name="{ row }"><div class="min-w-40"><div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div><div class="text-xs text-gray-500">{{ row.subtitle }}</div></div></template>
          <template #cell-price="{ row }"><span class="tabular-nums">{{ row.currency }} {{ row.price }}</span><span v-if="discountText(row)" class="ml-2 rounded bg-emerald-100 px-1.5 py-0.5 text-[11px] font-semibold text-emerald-700">{{ discountText(row) }}</span></template>
          <template #cell-original_price="{ row }"><span class="tabular-nums">{{ row.original_price ? `${row.currency} ${row.original_price}` : '-' }}</span></template>
          <template #cell-billing_period="{ row }"><span class="whitespace-nowrap">{{ periodLabel(row.billing_period) }}</span></template>
          <template #cell-group_name="{ row }"><span>{{ row.group_name || '-' }}</span></template>
          <template #cell-provider="{ row }"><span>{{ row.provider || '-' }}</span></template>
          <template #cell-rate_multiplier="{ row }"><span class="tabular-nums">×{{ row.rate_multiplier || '1' }}</span></template>
          <template #cell-quota="{ row }"><div class="space-y-0.5 text-xs text-gray-600 dark:text-gray-300"><div>{{ t('planCatalog.daily') }}: {{ quota(row.daily_limit_usd) }}</div><div>{{ t('planCatalog.weekly') }}: {{ quota(row.weekly_limit_usd) }}</div><div>{{ t('planCatalog.monthly') }}: {{ quota(row.monthly_limit_usd) }}</div></div></template>
          <template #cell-badge="{ row }"><span v-if="row.badge" class="rounded-full bg-gray-100 px-2 py-0.5 text-xs dark:bg-dark-700">{{ row.badge }}</span><span v-else>-</span></template>
          <template #cell-benefits="{ row }"><span class="tabular-nums">{{ row.benefits.length }}</span></template>
          <template #cell-is_featured="{ row }"><span class="badge" :class="row.is_featured ? 'badge-warning' : 'badge-gray'">{{ row.is_featured ? t('planCatalog.featured') : '-' }}</span></template>
          <template #cell-status="{ row }"><button class="badge" :class="row.is_published ? 'badge-success' : 'badge-gray'" @click.stop="togglePublished(row)">{{ row.is_published ? t('planCatalog.published') : t('planCatalog.draft') }}</button></template>
          <template #cell-sort_order="{ row }"><span class="tabular-nums">{{ row.sort_order }}</span></template>
          <template #cell-actions="{ row }"><div class="flex items-center gap-2"><button class="text-primary-600 hover:underline" @click.stop="openEdit(row)">{{ t('common.edit') }}</button><button class="text-primary-600 hover:underline" @click.stop="openDuplicate(row)">{{ t('planCatalog.duplicate') }}</button><button class="text-red-600 hover:underline" @click.stop="remove(row)">{{ t('common.delete') }}</button></div></template>
          <template #empty><div class="py-10 text-center text-sm text-gray-500">{{ search ? t('planCatalog.noResults') : t('planCatalog.empty') }}</div></template>
        </DataTable>
      </template>

      <template #pagination><Pagination v-if="totalItems" :page="page" :total="totalItems" :page-size="pageSize" @update:page="page = $event" @update:pageSize="pageSize = $event; page = 1" /></template>
    </TablePageLayout>

    <BaseDialog :show="dialog" :title="dialogTitle" width="wide" @close="dialog = false">
      <form id="plan-catalog-form" class="space-y-4" @submit.prevent="save">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="input-label">{{ t('planCatalog.fields.name') }}<input v-model="form.name" class="input mt-1" required maxlength="80" /></label>
          <label class="input-label">{{ t('planCatalog.fields.subtitle') }}<input v-model="form.subtitle" class="input mt-1" maxlength="160" /></label>
          <label class="input-label sm:col-span-2">{{ t('planCatalog.fields.description') }}<textarea v-model="form.description" class="input mt-1 min-h-20" maxlength="600" /></label>
          <label class="input-label">{{ t('planCatalog.fields.price') }}<input v-model="form.price" class="input mt-1" type="number" min="0" step="0.0001" required /></label>
          <label class="input-label">{{ t('planCatalog.fields.originalPrice') }}<input v-model="form.original_price" class="input mt-1" type="number" min="0" step="0.0001" /></label>
          <label class="input-label">{{ t('planCatalog.fields.currency') }}<select v-model="form.currency" class="input mt-1"><option v-for="v in currencies" :key="v">{{ v }}</option></select></label>
          <label class="input-label">{{ t('planCatalog.fields.period') }}<select v-model="form.billing_period" class="input mt-1"><option v-for="v in periods" :key="v" :value="v">{{ t(`planCatalog.period.${v}`) }}</option></select></label>
          <label class="input-label">{{ t('planCatalog.fields.group') }}<input v-model="form.group_name" class="input mt-1" maxlength="120" /></label>
          <label class="input-label">{{ t('planCatalog.fields.provider') }}<input v-model="form.provider" class="input mt-1" maxlength="120" /></label>
          <label class="input-label">{{ t('planCatalog.fields.rate') }}<input v-model="form.rate_multiplier" class="input mt-1" type="number" min="0" step="0.0001" /></label>
          <label class="input-label">{{ t('planCatalog.fields.daily') }}<input v-model="form.daily_limit_usd" class="input mt-1" type="number" min="0" step="0.0001" /></label>
          <label class="input-label">{{ t('planCatalog.fields.weekly') }}<input v-model="form.weekly_limit_usd" class="input mt-1" type="number" min="0" step="0.0001" /></label>
          <label class="input-label">{{ t('planCatalog.fields.monthly') }}<input v-model="form.monthly_limit_usd" class="input mt-1" type="number" min="0" step="0.0001" /></label>
          <label class="input-label">{{ t('planCatalog.fields.badge') }}<input v-model="form.badge" class="input mt-1" maxlength="40" /></label>
          <label class="input-label">{{ t('planCatalog.fields.accent') }}<select v-model="form.accent" class="input mt-1"><option v-for="v in accents" :key="v" :value="v">{{ v }}</option></select></label>
          <label class="input-label">{{ t('planCatalog.fields.sortOrder') }}<input v-model.number="form.sort_order" class="input mt-1" type="number" step="1" /></label>
          <label class="input-label sm:col-span-2">{{ t('planCatalog.fields.paymentUrl') }}<input v-model="form.payment_url" class="input mt-1 font-mono text-sm" type="url" required /></label>
          <label class="input-label sm:col-span-2">{{ t('planCatalog.fields.benefits') }}<textarea v-model="benefitsText" class="input mt-1 min-h-28" :placeholder="t('planCatalog.fields.benefitsHint')" /></label>
        </div>
        <div class="flex flex-wrap gap-5"><label class="flex items-center gap-2 text-sm"><input v-model="form.is_published" type="checkbox" />{{ t('planCatalog.published') }}</label><label class="flex items-center gap-2 text-sm"><input v-model="form.is_featured" type="checkbox" />{{ t('planCatalog.featured') }}</label></div>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" @click="dialog = false">{{ t('common.cancel') }}</button><button form="plan-catalog-form" class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></div></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { adminPlanCatalogAPI } from '@/api/admin/planCatalog'
import type { PlanCatalogInput, PlanCatalogItem } from '@/types/planCatalog'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const items = ref<PlanCatalogItem[]>([]); const loading = ref(false); const dialog = ref(false); const editing = ref<number | null>(null); const duplicating = ref(false); const saving = ref(false); const search = ref(''); const statusFilter = ref(''); const page = ref(1); const pageSize = ref(10); const selectedKeys = ref<Array<string | number>>([]); const sortKey = ref('sort_order'); const sortOrder = ref<'asc' | 'desc'>('asc'); const showColumns = ref(false); const columnMenuRef = ref<HTMLElement | null>(null)
const currencies = ['CNY','USD','EUR','HKD'] as const; const periods = ['monthly','quarterly','yearly','one_time','custom'] as const; const accents = ['indigo','emerald','amber','rose','slate'] as const; const benefitsText = ref('')
const blank = (): PlanCatalogInput => ({ name:'', subtitle:'', description:'', price:'0', original_price:null, currency:'CNY', billing_period:'monthly', badge:'', accent:'indigo', group_name:'', provider:'', rate_multiplier:'1', daily_limit_usd:null, weekly_limit_usd:null, monthly_limit_usd:null, benefits:[], payment_url:'https://', is_published:false, is_featured:false, sort_order:0 })
const form = reactive<PlanCatalogInput>(blank())
const dialogTitle = computed(() => editing.value ? t('common.edit') : duplicating.value ? t('planCatalog.duplicatePlan') : t('planCatalog.newPlan'))
const allColumns = computed<Column[]>(() => [
  { key:'name', label:t('planCatalog.fields.name'), sortable:true },
  { key:'price', label:t('planCatalog.fields.price'), sortable:true },
  { key:'original_price', label:t('planCatalog.fields.originalPrice'), sortable:true },
  { key:'billing_period', label:t('planCatalog.fields.period'), sortable:true },
  { key:'group_name', label:t('planCatalog.group'), sortable:true },
  { key:'provider', label:t('planCatalog.provider'), sortable:true },
  { key:'rate_multiplier', label:t('planCatalog.rate'), sortable:true },
  { key:'quota', label:t('planCatalog.quota') },
  { key:'badge', label:t('planCatalog.fields.badge') },
  { key:'benefits', label:t('planCatalog.fields.benefits') },
  { key:'is_featured', label:t('planCatalog.featured'), sortable:true },
  { key:'status', label:t('planCatalog.fields.status'), sortable:true },
  { key:'sort_order', label:t('planCatalog.fields.sortOrder'), sortable:true },
  { key:'actions', label:t('common.actions') },
])
const ALWAYS_VISIBLE = new Set(['name','actions'])
const readHiddenColumns = (): string[] => {
  try {
    const parsed = JSON.parse(localStorage.getItem('plan-catalog-hidden-columns') || '[]')
    return Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === 'string') : []
  } catch {
    return []
  }
}
const hiddenColumns = ref<string[]>(readHiddenColumns())
const toggleableColumns = computed(() => allColumns.value.filter(c => !ALWAYS_VISIBLE.has(c.key)))
const isColumnVisible = (key:string) => !hiddenColumns.value.includes(key)
function toggleColumn(key:string){ hiddenColumns.value = isColumnVisible(key) ? [...hiddenColumns.value, key] : hiddenColumns.value.filter(v => v !== key); localStorage.setItem('plan-catalog-hidden-columns', JSON.stringify(hiddenColumns.value)) }
const columns = computed(() => allColumns.value.filter(c => ALWAYS_VISIBLE.has(c.key) || isColumnVisible(c.key)))
const statusOptions = computed(() => [{ value:'', label:t('planCatalog.allStatus') }, { value:'published', label:t('planCatalog.published') }, { value:'draft', label:t('planCatalog.draft') }])
const filteredItems = computed(() => { const query = search.value.trim().toLowerCase(); return items.value.filter(item => (!query || [item.name,item.subtitle,item.group_name,item.provider,item.badge].some(v => v?.toLowerCase().includes(query))) && (!statusFilter.value || (statusFilter.value === 'published' ? item.is_published : !item.is_published))).sort((a,b) => { const av:any = a[sortKey.value as keyof PlanCatalogItem] ?? ''; const bv:any = b[sortKey.value as keyof PlanCatalogItem] ?? ''; const result = String(av).localeCompare(String(bv), undefined, { numeric:true }); return sortOrder.value === 'asc' ? result : -result }) })
const totalItems = computed(() => filteredItems.value.length); const pagedItems = computed(() => filteredItems.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
function handleSort(key:string, order:'asc'|'desc'){ sortKey.value = key; sortOrder.value = order; page.value = 1 }
function quota(value?: string|null){ return value == null || value === '' ? t('planCatalog.unlimited') : `$${value}` }
function discountText(item:PlanCatalogItem){ const p=Number(item.price), o=Number(item.original_price); return Number.isFinite(p)&&Number.isFinite(o)&&o>p ? `-${Math.round((1-p/o)*100)}%` : '' }
function periodLabel(value: PlanCatalogItem['billing_period']) { return t(`planCatalog.period.${value}`) }
async function load(){ loading.value = true; try { items.value = (await adminPlanCatalogAPI.list()).data ?? []; selectedKeys.value = [] } finally { loading.value = false } }
function openCreate(){ Object.assign(form, blank()); benefitsText.value=''; editing.value=null; duplicating.value=false; dialog.value=true }
function openEdit(item:PlanCatalogItem){ Object.assign(form, { ...item, original_price:item.original_price ?? null, daily_limit_usd:item.daily_limit_usd ?? null, weekly_limit_usd:item.weekly_limit_usd ?? null, monthly_limit_usd:item.monthly_limit_usd ?? null }); benefitsText.value=item.benefits.join('\n'); editing.value=item.id; duplicating.value=false; dialog.value=true }
function openDuplicate(item:PlanCatalogItem){
  Object.assign(form, blank())
  Object.assign(form, {
    name: `${item.name} ${t('planCatalog.copySuffix')}`,
    subtitle: item.subtitle,
    description: item.description,
    price: item.price,
    original_price: item.original_price ?? null,
    currency: item.currency,
    billing_period: item.billing_period,
    badge: item.badge,
    accent: item.accent,
    group_name: item.group_name,
    provider: item.provider,
    rate_multiplier: item.rate_multiplier,
    daily_limit_usd: item.daily_limit_usd ?? null,
    weekly_limit_usd: item.weekly_limit_usd ?? null,
    monthly_limit_usd: item.monthly_limit_usd ?? null,
    benefits: [...item.benefits],
    is_published: false,
    is_featured: false,
    sort_order: item.sort_order + 1,
  })
  benefitsText.value=item.benefits.join('\n'); editing.value=null; duplicating.value=true; dialog.value=true
}
async function save(){ saving.value=true; try { form.benefits=benefitsText.value.split('\n').map(v=>v.trim()).filter(Boolean); const payload: PlanCatalogInput = { name:form.name, subtitle:form.subtitle, description:form.description, price:form.price, original_price:form.original_price ?? null, currency:form.currency, billing_period:form.billing_period, badge:form.badge, accent:form.accent, group_name:form.group_name, provider:form.provider, rate_multiplier:form.rate_multiplier, daily_limit_usd:form.daily_limit_usd ?? null, weekly_limit_usd:form.weekly_limit_usd ?? null, monthly_limit_usd:form.monthly_limit_usd ?? null, benefits:form.benefits, payment_url:form.payment_url, is_published:form.is_published, is_featured:form.is_featured, sort_order:form.sort_order }; if(editing.value) await adminPlanCatalogAPI.update(editing.value, payload); else await adminPlanCatalogAPI.create(payload); dialog.value=false; await load() } finally { saving.value=false } }
async function togglePublished(item:PlanCatalogItem){ await adminPlanCatalogAPI.update(item.id, { ...item, original_price:item.original_price ?? null, is_published:!item.is_published }); await load() }
async function remove(item:PlanCatalogItem){ if(window.confirm(t('planCatalog.deleteConfirm'))) { await adminPlanCatalogAPI.delete(item.id); await load() } }
async function batchPublish(value:boolean){ const ids = [...selectedKeys.value]; for(const id of ids){ const item=items.value.find(v=>v.id===Number(id)); if(item && item.is_published !== value) await adminPlanCatalogAPI.update(item.id, { ...item, original_price:item.original_price ?? null, is_published:value }) } await load() }
async function batchDelete(){ if(!selectedKeys.value.length || !window.confirm(t('planCatalog.deleteConfirm'))) return; for(const id of selectedKeys.value) await adminPlanCatalogAPI.delete(Number(id)); await load() }
watch([search, statusFilter], () => { page.value = 1 })
onMounted(load)
</script>
