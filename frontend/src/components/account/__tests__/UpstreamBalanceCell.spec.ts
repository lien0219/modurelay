import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UpstreamBalanceCell from '../UpstreamBalanceCell.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { Account, UpstreamBalanceData } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${Object.values(params).join(',')}` : key,
      te: (key: string) => key.endsWith('.http_error')
    })
  }
})

const makeAccount = (balance: UpstreamBalanceData, overrides: Partial<Account> = {}): Account => ({
  id: 1,
  name: 'upstream',
  platform: 'openai',
  type: 'apikey',
  proxy_id: null,
  concurrency: 1,
  priority: 1,
  status: 'active',
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-07-13T00:00:00Z',
  updated_at: '2026-07-13T00:00:00Z',
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null,
  extra: {
    upstream_billing_probe_enabled: true,
    upstream_billing_probe: {
      status: 'ok',
      received_at: '2026-07-13T00:00:00Z',
      fresh_until: '2026-07-13T01:00:00Z',
      last_attempt_at: '2026-07-13T00:00:00Z',
      next_probe_at: '2026-07-13T00:30:00Z',
      balance: {
        status: 'ok',
        data: balance,
        source: 'billing',
        received_at: '2026-07-13T00:00:00Z',
        fresh_until: '2026-07-13T01:00:00Z',
        last_attempt_at: '2026-07-13T00:00:00Z',
        next_probe_at: '2026-07-13T00:30:00Z'
      }
    }
  },
  ...overrides
})

describe('UpstreamBalanceCell', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-13T00:30:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders a fresh wallet balance and emits a refresh command', async () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: {
        account: makeAccount({ mode: 'wallet', unit: 'USD', balance: 18.5, remaining: 18.5 }),
        now: Date.now()
      }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toContain('18.50')
    expect(wrapper.get('[data-testid="upstream-balance-value"]').element.tagName).toBe('BUTTON')
    expect(wrapper.getComponent(HelpTooltip).props('trigger')).toBe('click')
    expect(wrapper.find('[data-testid="upstream-balance-status"]').exists()).toBe(false)
    await wrapper.get('[data-testid="upstream-balance-probe"]').trigger('click')
    expect(wrapper.emitted('probe')).toHaveLength(1)
  })

  it('renders balances for legacy Antigravity upstream accounts', () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: {
        account: makeAccount(
          { mode: 'wallet', unit: 'USD', balance: 7.25, remaining: 7.25 },
          { platform: 'antigravity', type: 'upstream' }
        ),
        now: Date.now()
      }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toContain('7.25')
    expect(wrapper.get('[data-testid="upstream-balance-probe"]').exists()).toBe(true)
  })

  it('keeps zero as a real exhausted quota value', () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: {
        account: makeAccount({ mode: 'key_quota', unit: 'USD', remaining: 0, limit: 100, used: 100 }),
        now: Date.now()
      }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toContain('0.00')
    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.exhausted'
    )
    expect(wrapper.get('[data-testid="upstream-balance-status"]').classes()).toContain('text-red-600')
  })

  it('does not round a small negative balance to negative zero', () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: {
        account: makeAccount({ mode: 'wallet', unit: 'USD', balance: -0.005, remaining: -0.005 }),
        now: Date.now()
      }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toContain('0.005000')
    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.exhausted'
    )
  })

  it('explains when exhaustion was used to pause scheduling automatically', () => {
    const account = makeAccount({ mode: 'wallet', unit: 'USD', balance: 0, remaining: 0 })
    account.extra!.upstream_billing_auto_unschedulable = true
    const wrapper = mount(UpstreamBalanceCell, {
      props: { account, now: Date.now() }
    })

    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.exhaustedAutoPaused'
    )
  })

  it('keeps the durable scheduling pause visible after the upstream balance recovers', () => {
    const account = makeAccount({ mode: 'wallet', unit: 'USD', balance: 12, remaining: 12 })
    account.schedulable = false
    account.extra!.upstream_billing_auto_unschedulable = true
    account.extra!.upstream_billing_probe!.auto_unschedulable = true
    const wrapper = mount(UpstreamBalanceCell, {
      props: { account, now: Date.now() }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toContain('12.00')
    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.autoPaused'
    )
    expect(wrapper.get('[data-testid="upstream-balance-status"]').classes()).toContain('text-red-600')
  })

  it('renders unlimited subscription quota without inventing a number', () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: {
        account: makeAccount({ mode: 'subscription', unit: 'USD', unlimited: true }),
        now: Date.now()
      }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toBe(
      'admin.accounts.upstreamBalance.unlimited'
    )
  })

  it('labels New API token and wallet balance sources', async () => {
    const tokenAccount = makeAccount({ mode: 'key_quota', unit: 'USD', unlimited: true })
    tokenAccount.extra!.upstream_billing_probe!.balance!.source = 'new_api_token'
    const wrapper = mount(UpstreamBalanceCell, {
      attachTo: document.body,
      props: { account: tokenAccount, now: Date.now() }
    })

    await wrapper.get('[data-testid="upstream-balance-value"]').trigger('click')
    await flushPromises()
    let tooltips = document.body.querySelectorAll('[role="tooltip"]')
    expect(tooltips[tooltips.length - 1]?.textContent).toContain(
      'admin.accounts.upstreamBalance.sources.newAPIToken'
    )
    wrapper.unmount()

    const walletAccount = makeAccount({ mode: 'wallet', unit: 'USD', balance: 12, remaining: 12 })
    walletAccount.extra!.upstream_billing_probe!.balance!.source = 'new_api_wallet'
    const walletWrapper = mount(UpstreamBalanceCell, {
      attachTo: document.body,
      props: { account: walletAccount, now: Date.now() }
    })
    await walletWrapper.get('[data-testid="upstream-balance-value"]').trigger('click')
    await flushPromises()
    tooltips = document.body.querySelectorAll('[role="tooltip"]')
    expect(tooltips[tooltips.length - 1]?.textContent).toContain(
      'admin.accounts.upstreamBalance.sources.newAPIWallet'
    )
    walletWrapper.unmount()
  })

  it('distinguishes an unlimited New API token from an unconfigured wallet probe', async () => {
    const account = makeAccount({
      mode: 'key_quota',
      unit: 'CNY',
      unlimited: true,
      wallet_probe_status: 'not_configured'
    })
    account.extra!.upstream_billing_probe!.balance!.source = 'new_api_token'
    const wrapper = mount(UpstreamBalanceCell, {
      attachTo: document.body,
      props: { account, now: Date.now() }
    })

    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toBe(
      'admin.accounts.upstreamBalance.tokenUnlimited'
    )
    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.walletNotConfigured'
    )
    await wrapper.get('[data-testid="upstream-balance-value"]').trigger('click')
    await flushPromises()
    const tooltips = document.body.querySelectorAll('[role="tooltip"]')
    expect(tooltips[tooltips.length - 1]?.textContent).toContain(
      'admin.accounts.upstreamBalance.walletNotConfiguredDetail'
    )
    wrapper.unmount()
  })

  it('shows a PAT wallet query warning without treating the relay token as failed', async () => {
    const account = makeAccount({
      mode: 'key_quota',
      unit: 'CNY',
      unlimited: true,
      wallet_probe_status: 'failed',
      wallet_probe_error: 'http_error',
      wallet_probe_http_status: 502
    })
    account.extra!.upstream_billing_probe!.balance!.source = 'new_api_token'
    const wrapper = mount(UpstreamBalanceCell, {
      attachTo: document.body,
      props: { account, now: Date.now() }
    })

    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.walletProbeFailed'
    )
    expect(wrapper.get('[data-testid="upstream-balance-status"]').classes()).toContain('text-amber-600')
    await wrapper.get('[data-testid="upstream-balance-value"]').trigger('click')
    await flushPromises()
    const tooltips = document.body.querySelectorAll('[role="tooltip"]')
    expect(tooltips[tooltips.length - 1]?.textContent).toContain(
      'admin.accounts.upstreamBalance.walletProbeError:admin.accounts.upstreamBalance.errors.http_error (HTTP 502)'
    )
    wrapper.unmount()
  })

  it('distinguishes stale, failed and unsupported snapshots', async () => {
    const account = makeAccount({ mode: 'wallet', unit: 'USD', balance: 9, remaining: 9 })
    const wrapper = mount(UpstreamBalanceCell, {
      props: { account, now: Date.parse('2026-07-13T01:00:00.001Z') }
    })
    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toBe(
      'admin.accounts.upstreamBalance.stale'
    )

    const failed = makeAccount({ mode: 'wallet', unit: 'USD', balance: 9, remaining: 9 })
    failed.extra!.upstream_billing_probe!.balance!.status = 'failed'
    failed.extra!.upstream_billing_probe!.balance!.last_error = 'http_error'
    failed.extra!.upstream_billing_probe!.balance!.http_status = 502
    await wrapper.setProps({ account: failed, now: Date.now() })
    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toContain('9.00')
    expect(wrapper.get('[data-testid="upstream-balance-status"]').text()).toBe(
      'admin.accounts.upstreamBalance.failed'
    )

    await wrapper.setProps({
      account: {
        ...makeAccount({ mode: 'wallet', unit: 'USD', balance: 9, remaining: 9 }),
        extra: {
          upstream_billing_probe: {
            status: 'unsupported',
            last_attempt_at: '2026-07-13T00:00:00Z',
            next_probe_at: '2026-07-13T04:00:00Z'
          }
        }
      }
    })
    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toBe(
      'admin.accounts.upstreamBalance.unsupported'
    )

    await wrapper.setProps({
      account: {
        ...makeAccount({ mode: 'wallet', unit: 'USD', balance: 9, remaining: 9 }),
        extra: {
          upstream_billing_probe: {
            status: 'failed',
            last_attempt_at: '2026-07-13T00:00:00Z',
            next_probe_at: '2026-07-13T01:00:00Z',
            last_error: 'request_failed'
          }
        }
      }
    })
    expect(wrapper.get('[data-testid="upstream-balance-value"]').text()).toBe(
      'admin.accounts.upstreamBalance.failed'
    )
  })

  it('disables refresh while probing and hides it for non API-key accounts', async () => {
    const wrapper = mount(UpstreamBalanceCell, {
      props: {
        account: makeAccount({ mode: 'wallet', unit: 'USD', balance: 1, remaining: 1 }),
        now: Date.now(),
        probing: true
      }
    })
    expect(wrapper.get('[data-testid="upstream-balance-probe"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="upstream-balance-probe"]').attributes('aria-busy')).toBe('true')
    expect(wrapper.get('[data-testid="upstream-balance-probe"]').attributes('aria-label')).toBe(
      'admin.accounts.upstreamBalance.refreshing'
    )

    await wrapper.setProps({
      account: makeAccount(
        { mode: 'wallet', unit: 'USD', balance: 1, remaining: 1 },
        { type: 'oauth' }
      )
    })
    expect(wrapper.find('[data-testid="upstream-balance-probe"]').exists()).toBe(false)
    expect(wrapper.text()).toBe('-')

    await wrapper.setProps({
      account: makeAccount(
        { mode: 'wallet', unit: 'USD', balance: 1, remaining: 1 },
        { platform: 'future-platform' as any, type: 'apikey' }
      )
    })
    expect(wrapper.find('[data-testid="upstream-balance-probe"]').exists()).toBe(false)
    expect(wrapper.text()).toBe('-')
  })
})
