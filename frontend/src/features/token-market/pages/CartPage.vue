<template>
  <div class="tm-cart-page">
    <div class="tm-heading"><h1>购物车{{ draft.cartCount ? `（${draft.cartCount}）` : '' }}</h1><p>商品保存在当前浏览器的草稿中，结算价格由服务端确认。</p></div>
    <div v-if="!lines.length" class="tm-empty"><img src="/token-market/icons/cart.svg" alt="" /><h2>购物车还是空的</h2><p>去逛逛，找到适合你的商品。</p><RouterLink class="tm-button primary" to="/token-market/channel?category=ai_credit">浏览商品</RouterLink></div>
    <div v-else class="tm-cart-grid">
      <div class="tm-cart-lines">
        <article v-for="line in lines" :key="line.productId" class="tm-cart-line tm-panel">
          <label class="tm-cart-check"><input v-model="line.selected" type="checkbox" :aria-label="`选择${productName(line.productId)}`" /></label>
          <RouterLink class="tm-cart-art" :to="`/token-market/products/${line.productId}`"><img :src="productArtFor(line.productId)" :alt="productName(line.productId)" /></RouterLink>
          <div class="tm-cart-copy"><RouterLink :to="`/token-market/products/${line.productId}`"><h2>{{ productName(line.productId) }}</h2></RouterLink><p>{{ line.variant }}</p><strong class="tm-price">{{ productPrice(line.productId) == null ? '价格待更新' : formatToken(productPrice(line.productId)!) }}</strong></div>
          <div class="tm-cart-controls"><button type="button" :disabled="line.quantity <= 1" :aria-label="`减少${productName(line.productId)}数量`" @click="draft.setQuantity(line.productId,line.quantity-1)">−</button><output>{{ line.quantity }}</output><button type="button" :disabled="line.quantity >= 99" :aria-label="`增加${productName(line.productId)}数量`" @click="draft.setQuantity(line.productId,line.quantity+1)">+</button><button type="button" class="tm-icon-button" :aria-label="`移除${productName(line.productId)}`" title="移除商品" @click="draft.removeFromCart(line.productId)"><img src="/token-market/icons/trash.svg" alt="" /></button></div>
        </article>
      </div>
      <aside class="tm-cart-summary tm-panel"><h2>订单结算</h2><div class="tm-row"><span>已选商品（{{ selectedCount }}）</span><strong>{{ formatToken(subtotal) }}</strong></div><div class="tm-row"><span>运费与优惠</span><span>待服务端确认</span></div><hr /><span class="tm-muted">商品小计</span><strong class="tm-price">{{ formatToken(subtotal) }}</strong><RouterLink class="tm-button primary" :class="{ disabled: !selectedCount || missingPrices }" :aria-disabled="!selectedCount || missingPrices" :to="selectedCount && !missingPrices ? '/token-market/checkout' : '/token-market/cart'">去结算（{{ selectedCount }}）</RouterLink><small>下单、支付与扣款接口尚未接入。</small></aside>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useMarketDraftStore } from '../experience'
import { formatToken, productArt } from '../presentation'
import { useTokenMarketStore } from '../store'
const draft = useMarketDraftStore()
const market = useTokenMarketStore()
const lines = computed(() => draft.data.cart)
const selectedCount = computed(() => draft.selectedCart.reduce((total, line) => total + line.quantity, 0))
const missingPrices = computed(() => draft.selectedCart.some(line => productPrice(line.productId) == null))
const subtotal = computed(() => draft.selectedCart.reduce((total, line) => total + (productPrice(line.productId) ?? 0) * line.quantity, 0))
function product(id: string) { return market.products.find(item => item.id === id) }
function productName(id: string): string { return product(id)?.name || '商品已下架' }
function productPrice(id: string): number | null { return product(id)?.priceToken ?? null }
function productArtFor(id: string): string { const item = product(id); return item ? productArt(item) : '/token-market/icons/shop.svg' }
</script>
<style scoped>
.tm-cart-grid{display:grid;grid-template-columns:minmax(0,1fr) 336px;gap:24px}.tm-cart-lines{display:grid;align-content:start;gap:16px}.tm-cart-line{display:grid;grid-template-columns:24px 144px minmax(0,1fr) auto;align-items:center;gap:16px;min-height:166px;padding:20px}.tm-cart-check input{width:18px;height:18px;accent-color:var(--tm-cyan)}.tm-cart-art{display:grid;place-items:center;width:144px;height:112px;border-radius:10px;background:#0a1530}.tm-cart-art img{width:100%;height:100%;object-fit:contain}.tm-cart-copy{min-width:0}.tm-cart-copy h2{font-size:16px;margin:0 0 4px}.tm-cart-copy p{margin:0 0 7px;color:var(--tm-muted);font-size:12px}.tm-cart-controls{display:flex;align-items:center;gap:8px}.tm-cart-controls>button:not(.tm-icon-button){width:32px;height:32px;border:1px solid var(--tm-border);border-radius:7px;background:transparent;color:var(--tm-text)}.tm-cart-controls output{min-width:20px;text-align:center}.tm-cart-summary{align-self:start;padding:24px}.tm-cart-summary h2{margin-bottom:20px}.tm-cart-summary .tm-row{margin:14px 0;color:var(--tm-muted)}.tm-cart-summary .tm-row strong{color:var(--tm-text)}.tm-cart-summary hr{margin:20px 0;border:0;border-top:1px solid var(--tm-border)}.tm-cart-summary>.tm-price{display:block;margin:8px 0 16px;font-size:32px}.tm-cart-summary .tm-button{width:100%}.tm-cart-summary .tm-button.disabled{pointer-events:none;opacity:.5}.tm-cart-summary small{display:block;margin-top:12px;color:var(--tm-muted)}
@media(max-width:1100px){.tm-cart-grid{grid-template-columns:1fr}.tm-cart-summary{order:1}}
@media(max-width:700px){.tm-cart-line{grid-template-columns:20px 100px minmax(0,1fr);gap:10px;min-height:0;padding:12px}.tm-cart-art{width:100px;height:96px}.tm-cart-copy h2{font-size:14px}.tm-cart-copy .tm-price{font-size:19px}.tm-cart-controls{grid-column:2/4;justify-content:flex-end}.tm-cart-summary{padding:20px}}
</style>
