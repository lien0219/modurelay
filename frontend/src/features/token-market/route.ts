import type { RouteRecordRaw } from 'vue-router'

const marketMeta = { requiresAuth: true, requiresAdmin: false }
const market = { ...marketMeta, marketPermission: 'market.read' }

export const tokenMarketRoutes: RouteRecordRaw[] = [{
  path: '/token-market',
  component: () => import('./TokenMarketRouterView.vue'),
  meta: marketMeta,
  children: [
    { path: '', name: 'TokenMarket', component: () => import('./TokenMarketPage.vue'), meta: { ...market, title: 'Token Market' } },
    { path: 'channel', name: 'TokenMarketChannel', component: () => import('./pages/ChannelPage.vue'), meta: { ...market, title: 'Token Market · Channel' } },
    { path: 'search', name: 'TokenMarketSearch', component: () => import('./pages/ChannelPage.vue'), meta: { ...market, title: 'Token Market · Search' } },
    { path: 'products/:id', name: 'TokenMarketProductDetail', component: () => import('./pages/ProductDetailPage.vue'), meta: { ...marketMeta, title: 'Token Market · Product', marketPermission: 'products.read' } },
    { path: 'merchants/:id', name: 'TokenMarketMerchantStore', component: () => import('./pages/MerchantStorePage.vue'), meta: { ...marketMeta, title: 'Token Market · Merchant', marketPermission: 'merchants.read' } },
    { path: 'cart', name: 'TokenMarketCart', component: () => import('./pages/CartPage.vue'), meta: { ...marketMeta, title: 'Token Market · Cart', marketPermission: 'cart.read' } },
    { path: 'checkout', name: 'TokenMarketCheckout', component: () => import('./pages/CheckoutPage.vue'), meta: { ...marketMeta, title: 'Token Market · Checkout', marketPermission: 'cart.read' } },
    { path: 'payment-result', name: 'TokenMarketPaymentResult', component: () => import('./pages/CheckoutPage.vue'), meta: { ...marketMeta, title: 'Token Market · Payment Result', marketPermission: 'orders.read' } },
    { path: 'orders', name: 'TokenMarketOrders', component: () => import('./pages/OrdersPage.vue'), meta: { ...marketMeta, title: 'Token Market · Orders', marketPermission: 'orders.read' } },
    { path: 'orders/:id', name: 'TokenMarketOrderDetail', component: () => import('./pages/OrderDetailPage.vue'), meta: { ...marketMeta, title: 'Token Market · Order Detail', marketPermission: 'orders.read' } },
    { path: 'orders/:id/after-sale', name: 'TokenMarketAfterSale', component: () => import('./pages/OrderDetailPage.vue'), meta: { ...marketMeta, title: 'Token Market · After Sale', marketPermission: 'orders.read' } },
    { path: 'wallet', name: 'TokenMarketWallet', component: () => import('./pages/WalletPage.vue'), meta: { ...marketMeta, title: 'Token Market · Wallet', marketPermission: 'wallet.read' } },
    { path: 'exchange', name: 'TokenMarketExchange', component: () => import('./pages/ExchangePage.vue'), meta: { ...marketMeta, title: 'Token Market · Exchange', marketPermission: 'wallet.exchange.read' } },
    { path: 'recharge', name: 'TokenMarketRecharge', component: () => import('./pages/ExchangePage.vue'), meta: { ...marketMeta, title: 'Token Market · Recharge', marketPermission: 'wallet.read' } },
    { path: 'exchange-history', name: 'TokenMarketExchangeHistory', component: () => import('./pages/WalletPage.vue'), meta: { ...marketMeta, title: 'Token Market · Exchange History', marketPermission: 'wallet.exchange.read' } },
    { path: 'favorites', name: 'TokenMarketFavorites', component: () => import('./pages/AccountPage.vue'), meta: { ...market, title: 'Token Market · Favorites' } },
    { path: 'messages', name: 'TokenMarketMessages', component: () => import('./pages/AccountPage.vue'), meta: { ...market, title: 'Token Market · Messages' } },
    { path: 'account', name: 'TokenMarketAccount', component: () => import('./pages/AccountPage.vue'), meta: { ...market, title: 'Token Market · Account' } },
    { path: 'addresses', name: 'TokenMarketAddresses', component: () => import('./pages/AccountPage.vue'), meta: { ...market, title: 'Token Market · Addresses' } },
    { path: 'security', name: 'TokenMarketSecurity', component: () => import('./pages/AccountPage.vue'), meta: { ...market, title: 'Token Market · Security' } },
    { path: 'merchant-apply', name: 'TokenMarketMerchantApply', component: () => import('./pages/MerchantWorkspacePage.vue'), meta: { ...market, title: 'Token Market · Merchant Apply' } },
    { path: 'merchant-center', name: 'TokenMarketMerchantCenter', component: () => import('./pages/MerchantWorkspacePage.vue'), meta: { ...marketMeta, title: 'Token Market · Merchant Center', marketPermission: 'merchant.center' } },
    { path: 'merchant-products', name: 'TokenMarketMerchantProducts', component: () => import('./pages/MerchantWorkspacePage.vue'), meta: { ...marketMeta, title: 'Token Market · Merchant Products', marketPermission: 'merchant.center' } },
    { path: 'merchant-products/new', name: 'TokenMarketMerchantPublish', component: () => import('./pages/MerchantWorkspacePage.vue'), meta: { ...marketMeta, title: 'Token Market · Publish Product', marketPermission: 'merchant.center' } },
    { path: 'merchant-orders', name: 'TokenMarketMerchantOrders', component: () => import('./pages/MerchantWorkspacePage.vue'), meta: { ...marketMeta, title: 'Token Market · Merchant Orders', marketPermission: 'merchant.center' } },
    { path: 'merchant-settlement', name: 'TokenMarketMerchantSettlement', component: () => import('./pages/MerchantWorkspacePage.vue'), meta: { ...marketMeta, title: 'Token Market · Settlement', marketPermission: 'merchant.center' } },
  ],
}]
