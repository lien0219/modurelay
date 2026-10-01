export type TokenMarketExchangeMode = 'balance' | 'token'
export type TokenMarketPanelKind = 'wallet' | 'orders' | 'activity' | 'cart'

export type TokenMarketDialogState =
  | { type: 'exchange' }
  | { type: 'product'; title: string; price: string }
  | { type: 'info'; title: string; message: string }

export interface TokenMarketNavItem {
  label: string
  section: 'home' | string
}
