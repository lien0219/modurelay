import type { RouteRecordRaw } from 'vue-router'

const marketMeta = {
  requiresAuth: true,
  requiresAdmin: false,
}

export const tokenMarketRoutes: RouteRecordRaw[] = [
  {
    path: '/token-market',
    name: 'TokenMarket',
    component: () => import('@/views/user/TokenMarketView.vue'),
    meta: { ...marketMeta, title: 'Token Market' },
  },
  {
    path: '/token-market/products/:id',
    name: 'TokenMarketProductDetail',
    component: () => import('./pages/ProductDetailPage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Product' },
  },
  {
    path: '/token-market/merchants/:id',
    name: 'TokenMarketMerchantStore',
    component: () => import('./pages/MerchantStorePage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Merchant' },
  },
  {
    path: '/token-market/cart',
    name: 'TokenMarketCart',
    component: () => import('./pages/CartPage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Cart' },
  },
  {
    path: '/token-market/orders',
    name: 'TokenMarketOrders',
    component: () => import('./pages/OrdersPage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Orders' },
  },
  {
    path: '/token-market/wallet',
    name: 'TokenMarketWallet',
    component: () => import('./pages/WalletPage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Wallet' },
  },
  {
    path: '/token-market/exchange-history',
    name: 'TokenMarketExchangeHistory',
    component: () => import('./pages/ExchangeHistoryPage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Exchange History' },
  },
  {
    path: '/token-market/merchant-center',
    name: 'TokenMarketMerchantCenter',
    component: () => import('./pages/MerchantCenterPage.vue'),
    meta: { ...marketMeta, title: 'Token Market · Merchant Center' },
  },
]
