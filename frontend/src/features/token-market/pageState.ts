export type TokenMarketPageState = 'ready' | 'loading' | 'empty' | 'error' | 'forbidden'

export type TokenMarketSkeletonVariant = 'detail' | 'list' | 'dashboard' | 'wallet'

export interface TokenMarketPermissionContext {
  permission?: string
  routeName?: string
}

export type TokenMarketPermissionResult = 'allowed' | 'denied' | 'pending'
