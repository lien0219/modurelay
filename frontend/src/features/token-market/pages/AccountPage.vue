<template>
  <div class="tm-account">
    <template v-if="section === 'favorites'"><div class="tm-heading"><h1>把心动留在这里</h1><p>你收藏的 {{ favoriteProducts.length }} 件商品与 {{ favoriteMerchants.length }} 家商店保存在当前浏览器。</p></div><div class="tm-tabs"><button v-for="tab in ['收藏商品','关注商家']" :key="tab" class="tm-tab" :class="{ 'is-active': favoriteTab === tab }" @click="favoriteTab=tab">{{ tab }}</button></div><div v-if="favoriteTab === '收藏商品'"><div v-if="favoriteProducts.length" class="tm-account-products"><MarketProductCard v-for="product in favoriteProducts" :key="product.id" :product="product" :merchant-name="merchantName(product.merchantId)" /></div><div v-else class="tm-empty"><img src="/token-market/icons/heart.svg" alt="" /><h2>暂无收藏商品</h2><p>商品详情页可以收藏喜欢的商品。</p><RouterLink class="tm-button primary" to="/token-market">去逛逛</RouterLink></div></div><div v-else><div v-if="favoriteMerchants.length" class="tm-account-merchants"><RouterLink v-for="merchant in favoriteMerchants" :key="merchant.id" class="tm-panel" :to="`/token-market/merchants/${merchant.id}`">{{ merchant.name }} <span>→</span></RouterLink></div><div v-else class="tm-empty"><h2>暂无关注商家</h2><p>可以在店铺页面关注商家。</p></div></div></template>
    <template v-else-if="section === 'messages'"><div class="tm-heading"><h1>消息中心</h1><p>订单通知与商家沟通会在消息接口接入后显示。</p></div><div class="tm-messages"><aside class="tm-panel"><div class="tm-tabs" role="tablist" aria-label="消息筛选"><button class="tm-tab" :class="{ 'is-active': messageFilter === 'all' }" type="button" role="tab" :aria-selected="messageFilter === 'all'" @click="messageFilter='all'">全部</button><button class="tm-tab" :class="{ 'is-active': messageFilter === 'unread' }" type="button" role="tab" :aria-selected="messageFilter === 'unread'" @click="messageFilter='unread'">未读</button></div><div class="tm-empty"><img src="/token-market/icons/message.svg" alt="" /><h2>{{ messageFilter === 'unread' ? '暂无未读消息' : '暂无消息' }}</h2><p>消息列表接口尚未接入。</p></div></aside><section class="tm-panel"><h2>商家沟通</h2><p v-if="route.query.merchant" class="tm-muted">目标商家：{{ merchantName(String(route.query.merchant)) }}</p><div class="tm-empty"><p>当前无法读取会话或发送消息。请通过已开通的客服渠道联系平台。</p></div><label class="tm-field"><span>消息草稿</span><textarea v-model="messageDraft" class="tm-textarea" placeholder="输入消息…" /></label><button class="tm-button" type="button" :disabled="!messageDraft.trim()" @click="toast='消息接口尚未接入，草稿未发送'">发送</button></section></div></template>
    <template v-else><div class="tm-heading"><h1>{{ section === 'addresses' ? '收货地址' : section === 'security' ? '安全设置' : '个人中心' }}</h1><p>管理账户资料、收货地址与安全设置。</p></div><div class="tm-tabs"><RouterLink v-for="tab in accountTabs" :key="tab.to" class="tm-tab" :class="{ 'is-active': route.path === tab.to }" :to="tab.to">{{ tab.label }}</RouterLink></div>
      <div v-if="section === 'account'" class="tm-account-grid"><form class="tm-panel tm-account-form" @submit.prevent="saveProfile"><h2>{{ auth.user?.username || '我的账户' }}</h2><label class="tm-field"><span>平台用户名</span><input v-model.trim="username" class="tm-input" required minlength="2" maxlength="32" /></label><label class="tm-field"><span>电子邮箱</span><input class="tm-input" :value="auth.user?.email || ''" readonly /><small>更换邮箱请使用平台账户设置。</small></label><label class="tm-field"><span>个人简介（仅本机草稿）</span><textarea v-model="draft.data.bio" class="tm-textarea" maxlength="300" /></label><button class="tm-button primary" type="submit" :disabled="saving">{{ saving ? '保存中…' : '保存平台用户名' }}</button></form><aside class="tm-panel tm-account-assets"><h2>我的资产</h2><strong>{{ market.wallet ? formatToken(market.wallet.tokenBalance) : '—' }}</strong><RouterLink class="tm-text-link" to="/token-market/wallet">查看钱包 →</RouterLink><RouterLink class="tm-text-link" to="/token-market/favorites">查看收藏 →</RouterLink></aside></div>
      <div v-else-if="section === 'addresses'"><button class="tm-button primary" type="button" @click="startAddress()">+ 新增收货地址</button><p class="tm-muted">地址保存在本次浏览器会话中，尚未同步到平台账户或结算接口。</p><div v-if="draft.data.addresses.length" class="tm-address-grid"><article v-for="address in draft.data.addresses" :key="address.id" class="tm-panel"><div class="tm-row"><h2>{{ address.label }}</h2><span v-if="address.isDefault">默认草稿</span></div><strong>{{ address.name }} · {{ address.phone }}</strong><p>{{ address.detail }}</p><div><button class="tm-text-link" type="button" @click="startAddress(address)">编辑</button><button class="tm-text-link" type="button" @click="draft.removeAddress(address.id)">删除</button><button v-if="!address.isDefault" class="tm-text-link" type="button" @click="draft.saveAddress({ ...address, isDefault: true })">设为默认</button></div></article></div><div v-else class="tm-empty"><h2>暂无地址</h2><p>实物商品结算前请填写收货地址。</p></div></div>
      <div v-else class="tm-panel tm-security"><div v-for="item in securityItems" :key="item.title" class="tm-row"><span><strong>{{ item.title }}</strong><small>{{ item.detail }}</small></span><RouterLink class="tm-button" to="/profile">{{ item.action }}</RouterLink></div></div>
    </template>
    <div v-if="addressOpen" class="tm-dialog-backdrop" @click.self="addressOpen=false"><form class="tm-dialog" role="dialog" aria-modal="true" aria-labelledby="address-dialog-title" @submit.prevent="saveAddress"><h2 id="address-dialog-title">{{ editingAddress ? '编辑收货地址' : '新增收货地址' }}</h2><label class="tm-field"><span>地址标签</span><input v-model.trim="addressForm.label" class="tm-input" required placeholder="家 / 公司" /></label><label class="tm-field"><span>收货人</span><input v-model.trim="addressForm.name" class="tm-input" required /></label><label class="tm-field"><span>联系电话</span><input v-model.trim="addressForm.phone" class="tm-input" type="tel" required /></label><label class="tm-field"><span>详细地址</span><textarea v-model.trim="addressForm.detail" class="tm-textarea" required /></label><label class="tm-address-default"><input v-model="addressForm.isDefault" type="checkbox" />设为默认草稿</label><div class="tm-dialog-actions"><button class="tm-button" type="button" @click="addressOpen=false">取消</button><button class="tm-button primary" type="submit">保存草稿</button></div></form></div>
    <div v-if="toast" class="tm-toast" role="status">{{ toast }}</div>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import MarketProductCard from '../components/MarketProductCard.vue'
import { useMarketDraftStore, type MarketAddress } from '../experience'
import { formatToken } from '../presentation'
import { useTokenMarketStore } from '../store'
const route = useRoute()
const auth = useAuthStore()
const market = useTokenMarketStore()
const draft = useMarketDraftStore()
const section = computed(() => route.path.split('/').at(-1) || 'account')
const accountTabs = [{ label: '个人资料', to: '/token-market/account' }, { label: '收货地址', to: '/token-market/addresses' }, { label: '安全设置', to: '/token-market/security' }]
const securityItems = [
  { title: '登录密码', detail: '通过平台账户设置更新密码', action: '修改密码' },
  { title: '电子邮箱', detail: auth.user?.email || '未提供', action: '管理邮箱' },
  { title: '双重验证', detail: '使用平台现有验证设置', action: '管理验证' },
  { title: '登录设备', detail: '前往平台账户安全中心查看', action: '查看设备' },
]
const username = ref(auth.user?.username || '')
const saving = ref(false)
const toast = ref('')
const messageDraft = ref('')
const messageFilter = ref('all')
const favoriteTab = ref('收藏商品')
const favoriteProducts = computed(() => market.products.filter(item => draft.data.favoriteProductIds.includes(item.id)))
const favoriteMerchants = computed(() => market.merchants.filter(item => draft.data.favoriteMerchantIds.includes(item.id)))
const addressOpen = ref(false)
const editingAddress = ref(false)
const addressForm = ref<MarketAddress>({ id: '', label: '', name: '', phone: '', detail: '', isDefault: false })
function merchantName(id: string): string { return market.merchants.find(item => item.id === id)?.name || '未知商家' }
async function saveProfile(): Promise<void> {
  saving.value = true
  try { await market.savePlatformUsername(username.value); toast.value = '平台用户名已保存' }
  catch (failure) { toast.value = failure instanceof Error ? failure.message : '保存失败，请稍后重试' }
  finally { saving.value = false }
}
function startAddress(address?: MarketAddress): void {
  editingAddress.value = !!address
  addressForm.value = address ? { ...address } : { id: crypto.randomUUID(), label: '', name: '', phone: '', detail: '', isDefault: draft.data.addresses.length === 0 }
  addressOpen.value = true
}
function saveAddress(): void { draft.saveAddress({ ...addressForm.value }); addressOpen.value = false; toast.value = '地址已保存到本次会话草稿，尚未同步平台' }
watch(() => auth.user?.username, value => { if (value) username.value = value })
</script>
<style scoped>
.tm-account .tm-tabs{align-items:center}.tm-account .tm-tab{display:inline-flex;align-items:center;justify-content:center}.tm-account-grid{display:grid;grid-template-columns:minmax(0,1.7fr) minmax(280px,1fr);gap:24px}.tm-account-form,.tm-account-assets{padding:24px}.tm-account-form h2{font-size:28px}.tm-account-assets strong{display:block;margin:15px 0;font-size:32px}.tm-account-assets .tm-text-link{display:block;margin:10px 0}.tm-account-products{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:15px}.tm-account-merchants{display:grid;grid-template-columns:repeat(3,1fr);gap:15px}.tm-account-merchants a{display:flex;justify-content:space-between;padding:20px}.tm-messages{display:grid;grid-template-columns:320px minmax(0,1fr);gap:24px}.tm-messages>aside,.tm-messages>section{min-height:530px;padding:24px}.tm-messages .tm-empty{padding:45px 10px;background:none;border:0}.tm-messages>section .tm-empty{min-height:260px}.tm-address-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:20px;margin-top:20px}.tm-address-grid article{padding:24px}.tm-address-grid h2{margin:0 0 10px}.tm-address-grid .tm-row span{color:var(--tm-cyan);font-size:12px}.tm-address-grid p{color:var(--tm-muted)}.tm-address-grid article>div:last-child{display:flex;gap:12px}.tm-address-default{display:flex;align-items:center;gap:8px}.tm-security{padding:18px 24px}.tm-security>.tm-row{padding:22px 0;border-bottom:1px solid var(--tm-border)}.tm-security>.tm-row:last-child{border:0}.tm-security small{display:block;color:var(--tm-muted);font-size:12px}
@media(max-width:950px){.tm-account-grid,.tm-messages{grid-template-columns:1fr}.tm-account-products{grid-template-columns:repeat(2,1fr)}}@media(max-width:700px){.tm-account .tm-tabs{flex-wrap:nowrap;overflow-x:auto}.tm-account .tm-tab{white-space:nowrap}.tm-account-form,.tm-account-assets,.tm-messages>aside,.tm-messages>section,.tm-address-grid article{padding:18px}.tm-account-products,.tm-account-merchants,.tm-address-grid{grid-template-columns:1fr}.tm-messages>aside,.tm-messages>section{min-height:0}.tm-security{padding:12px}.tm-security>.tm-row{gap:10px}.tm-security .tm-button{padding:0 10px;min-width:104px}}
</style>
