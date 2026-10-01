import type { RouteRecordRaw } from 'vue-router'

const marketMeta = {
  requiresAuth: true,
  requiresAdmin: false,
}

export const tokenMarketRoutes: RouteRecordRaw[] = [
  {
    path: '/token-market',
    component: () => import('./TokenMarketRouterView.vue'),
    meta: { ...marketMeta },
    children: [
      {
        path: '',
        name: 'TokenMarket',
        component: () => import('@/views/user/TokenMarketView.vue'),
        meta: { ...marketMeta, title: 'Token Market', marketPermission: 'market.read' },
      },
      {
        path: 'products/:id',
        name: 'TokenMarketProductDetail',
        component: () => import('./pages/ProductDetailPage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Product', marketPermission: 'products.read' },
      },
      {
        path: 'merchants/:id',
        name: 'TokenMarketMerchantStore',
        component: () => import('./pages/MerchantStorePage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Merchant', marketPermission: 'merchants.read' },
      },
      {
        path: 'cart',
        name: 'TokenMarketCart',
        component: () => import('./pages/CartPage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Cart', marketPermission: 'cart.read' },
      },
      {
        path: 'orders',
        name: 'TokenMarketOrders',
        component: () => import('./pages/OrdersPage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Orders', marketPermission: 'orders.read' },
      },
      {
        path: 'wallet',
        name: 'TokenMarketWallet',
        component: () => import('./pages/WalletPage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Wallet', marketPermission: 'wallet.read' },
      },
      {
        path: 'exchange-history',
        name: 'TokenMarketExchangeHistory',
        component: () => import('./pages/ExchangeHistoryPage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Exchange History', marketPermission: 'wallet.exchange.read' },
      },
      {
        path: 'merchant-center',
        name: 'TokenMarketMerchantCenter',
        component: () => import('./pages/MerchantCenterPage.vue'),
        meta: { ...marketMeta, title: 'Token Market · Merchant Center', marketPermission: 'merchant.center' },
      },
    ],
  },
]
