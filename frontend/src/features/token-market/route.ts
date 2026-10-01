import type { RouteRecordRaw } from 'vue-router'

export const tokenMarketRoute: RouteRecordRaw = {
  path: '/token-market',
  name: 'TokenMarket',
  component: () => import('@/views/user/TokenMarketView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: false,
    title: 'Token Market',
  },
}
