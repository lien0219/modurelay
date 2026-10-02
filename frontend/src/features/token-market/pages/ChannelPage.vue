<template>
  <div class="tm-catalog">
    <div class="tm-heading"><h1>{{ isSearch ? `“${query}”的搜索结果` : heading }}</h1><p>{{ isSearch ? `找到 ${products.length} 件商品与 ${merchants.length} 家商店` : subtitle }}</p></div>
    <div class="tm-tabs" role="tablist" aria-label="结果类型">
      <button v-for="tab in tabs" :key="tab" class="tm-tab" :class="{ 'is-active': activeTab === tab }" role="tab" type="button" :aria-selected="activeTab === tab" @click="activeTab=tab">{{ tab }}</button>
    </div>
    <div v-if="!isSearch && category === 'ai_credit'" class="tm-catalog-feature tm-panel">
      <img src="/token-market/art/chat-token.png" alt="" />
      <div><small>AI 额度</small><h2>从灵感到作品，按需选择模型额度</h2><p>模型、Input、Output 与 Cache 数量以商品详情及服务端报价为准。</p></div>
    </div>
    <div v-else-if="!isSearch" class="tm-catalog-feature tm-panel">
      <img src="/token-market/art/model-core.png" alt="" />
      <div><small>{{ heading }}</small><h2>{{ category === 'delivery' ? '发现附近的好味道' : '发现适合你的商品与服务' }}</h2><p>探索当前已上架的商家与商品。</p></div>
    </div>
    <div class="tm-catalog-controls">
      <div class="tm-tabs" aria-label="排序"><button v-for="option in sortOptions" :key="option.value" type="button" class="tm-tab" :class="{ 'is-active': sort === option.value }" @click="sort=option.value">{{ option.label }}</button></div>
      <select v-model="merchantFilter" class="tm-select" aria-label="筛选商家"><option value="">全部商家</option><option v-for="merchant in merchants" :key="merchant.id" :value="merchant.id">{{ merchant.name }}</option></select>
    </div>
    <div v-if="loading" class="tm-feedback" role="status">正在加载商品…</div>
    <div v-else-if="error" class="tm-empty" role="alert"><h2>加载失败</h2><p>{{ error }}</p><button class="tm-button" type="button" @click="load">重新加载</button></div>
    <template v-else>
      <section v-if="activeTab !== '商家'">
        <div v-if="visibleProducts.length" class="tm-catalog-products"><MarketProductCard v-for="product in visibleProducts" :key="product.id" :product="product" :merchant-name="merchantName(product.merchantId)" @added="showToast" /></div>
        <div v-else-if="activeTab === '商品' || !merchants.length" class="tm-empty"><img src="/token-market/icons/search.svg" alt="" /><h2>没有搜索结果</h2><p>试试其他关键词或清除筛选条件。</p><button class="tm-button" type="button" @click="clearFilters">清除筛选</button></div>
      </section>
      <section v-if="activeTab !== '商品' && merchants.length" class="tm-section"><div class="tm-section-head"><h2>相关商家</h2></div><div class="tm-catalog-merchants"><RouterLink v-for="merchant in merchants" :key="merchant.id" class="tm-panel" :to="`/token-market/merchants/${merchant.id}`"><img src="/token-market/icons/store.svg" alt="" /><span><strong>{{ merchant.name }}</strong><small>{{ categoryLabels[merchant.categoryId] }} · {{ merchant.rating.toFixed(1) }}</small></span><span>→</span></RouterLink></div></section>
      <div v-else-if="activeTab === '商家'" class="tm-empty"><img src="/token-market/icons/store.svg" alt="" /><h2>没有相关商家</h2><p>试试其他关键词或分类。</p><button class="tm-button" type="button" @click="clearFilters">清除筛选</button></div>
    </template>
    <div v-if="toast" class="tm-toast" role="status">{{ toast }}已加入本地购物车草稿</div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import MarketProductCard from '../components/MarketProductCard.vue'
import { categoryLabels } from '../presentation'
import { useTokenMarketStore } from '../store'
import type { TokenMarketCategoryId, TokenMarketMerchant, TokenMarketProduct } from '../types'
const route = useRoute()
const router = useRouter()
const market = useTokenMarketStore()
const isSearch = computed(() => route.path.endsWith('/search'))
const query = computed(() => String(route.query.q ?? '').trim())
const category = computed(() => categoryLabels[String(route.query.category)] ? String(route.query.category) as TokenMarketCategoryId : 'mall')
const heading = computed(() => categoryLabels[category.value] || '品质商城')
const subtitle = computed(() => ({ mall: '精选好物，按需浏览。', delivery: '查看可用的外卖与本地服务。', digital: '数字商品与权益，交付方式以详情为准。', ai_credit: '按需选择 AI 模型额度与创作套餐。', services: '寻找专业服务与解决方案。' })[category.value])
const tabs = ['全部', '商品', '商家']
const activeTab = ref('全部')
const sort = ref('default')
const merchantFilter = ref('')
const sortOptions = [{ value: 'default', label: '综合推荐' }, { value: 'low', label: '价格 ↑' }, { value: 'high', label: '价格 ↓' }]
const products = ref<TokenMarketProduct[]>([])
const merchants = ref<TokenMarketMerchant[]>([])
const loading = ref(false)
const error = ref('')
const toast = ref('')
let timer: number | undefined
let requestId = 0
async function load(): Promise<void> {
  const id = ++requestId
  loading.value = true; error.value = ''
  try {
    if (isSearch.value) {
      if (!query.value) { products.value = []; merchants.value = []; return }
      await market.search(query.value)
      if (id !== requestId) return
      products.value = [...market.searchProducts]
      merchants.value = [...market.searchMerchants]
    } else {
      const loaded = await market.loadProducts({ categoryId: category.value, pageSize: 100 })
      if (id !== requestId) return
      products.value = loaded
      merchants.value = market.merchants.filter(item => item.categoryId === category.value || loaded.some(product => product.merchantId === item.id))
    }
  } catch (failure) { if (id === requestId) error.value = failure instanceof Error ? failure.message : '请稍后重试' }
  finally { if (id === requestId) loading.value = false }
}
const visibleProducts = computed(() => {
  let items = products.value.filter(item => !merchantFilter.value || item.merchantId === merchantFilter.value)
  if (sort.value === 'low') items = [...items].sort((a, b) => a.priceToken - b.priceToken)
  if (sort.value === 'high') items = [...items].sort((a, b) => b.priceToken - a.priceToken)
  return items
})
function merchantName(id: string): string { return merchants.value.find(item => item.id === id)?.name || market.merchants.find(item => item.id === id)?.name || 'Token Market 商家' }
function clearFilters(): void { sort.value = 'default'; merchantFilter.value = ''; activeTab.value = '全部'; if (isSearch.value) void router.push('/token-market/channel?category=mall') }
function showToast(name: string): void { toast.value = name; window.clearTimeout(timer); timer = window.setTimeout(() => { toast.value = '' }, 3000) }
watch(() => route.fullPath, () => { activeTab.value = '全部'; merchantFilter.value = ''; void load() }, { immediate: true })
onBeforeUnmount(() => { requestId++; window.clearTimeout(timer) })
</script>
<style scoped>
.tm-catalog-feature{display:flex;align-items:center;gap:24px;margin:24px 0;padding:24px;min-height:178px}.tm-catalog-feature img{width:270px;height:130px;object-fit:contain;border-radius:10px;background:#0a1430}.tm-catalog-feature div{min-width:0}.tm-catalog-feature small{color:var(--tm-cyan)}.tm-catalog-feature h2{margin:8px 0}.tm-catalog-feature p{margin:0;color:var(--tm-muted)}.tm-catalog-controls{display:flex;align-items:center;justify-content:space-between;gap:12px}.tm-catalog-controls .tm-tabs{margin:12px 0 22px}.tm-catalog-controls .tm-tab{min-width:110px}.tm-catalog-controls .tm-tab.is-active{background:none;color:var(--tm-cyan);border:0}.tm-catalog-controls .tm-select{width:190px}.tm-catalog-products{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px}.tm-catalog-merchants{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.tm-catalog-merchants a{display:flex;align-items:center;gap:12px;padding:18px}.tm-catalog-merchants img{width:29px}.tm-catalog-merchants span:nth-child(2){flex:1;min-width:0}.tm-catalog-merchants small{display:block;color:var(--tm-muted)}
@media(max-width:1000px){.tm-catalog-products{grid-template-columns:repeat(2,1fr)}.tm-catalog-merchants{grid-template-columns:repeat(2,1fr)}}
@media(max-width:700px){.tm-catalog-feature{display:none}.tm-catalog-controls{display:block}.tm-catalog-controls .tm-tabs{overflow-x:auto;flex-wrap:nowrap}.tm-catalog-controls .tm-tab{flex:none}.tm-catalog-controls .tm-select{width:100%;margin-bottom:16px}.tm-catalog-products,.tm-catalog-merchants{grid-template-columns:1fr}.tm-catalog .tm-tabs{flex-wrap:nowrap;overflow-x:auto}.tm-catalog .tm-tabs .tm-tab{white-space:nowrap}}
</style>
