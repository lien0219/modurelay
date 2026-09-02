export default {
  distribution: {
    title: 'Distribution Management',
    description: 'Product and technical architecture for agent-exclusive multiplier distribution',
    eyebrow: 'Agent distribution architecture',
    headline: 'Distribution powered by agent-exclusive multipliers, not commissions',
    intro: 'The platform assigns each qualified agent an exclusive settlement multiplier. Agents set downstream distribution multipliers within policy bounds, while usage billing calculates agent cost, customer charges, and the multiplier spread in a separate, auditable ledger.',
    status: 'To be developed',
    principlesLabel: 'Agent distribution design principles',
    principles: {
      exclusiveMultiplier: 'One exclusive multiplier per agent',
      usagePricing: 'Priced from actual usage',
      independentLedger: 'Separated from affiliate rebates',
    },
    model: {
      eyebrow: 'Core pricing model',
      title: 'Define agent value through the multiplier spread',
      constraint: 'Constraint: R ≥ A',
      base: {
        title: 'Base usage amount',
        description: 'Raw usage value calculated from model pricing',
        formula: 'B = base usage amount',
      },
      agent: {
        title: 'Agent settlement cost',
        description: 'The platform assigns the exclusive agent multiplier',
        formula: 'Agent cost = B × A',
      },
      retail: {
        title: 'Downstream customer charge',
        description: 'The agent sets a distribution multiplier within policy',
        formula: 'Customer charge = B × R',
      },
      spread: {
        title: 'Multiplier spread',
        description: 'The difference between customer charge and agent cost',
        formula: 'Spread = B × (R - A)',
      },
      note: 'Each usage event snapshots B, A, R, and the rule version. Refunds reverse the original snapshot, and affiliate or commission ledger semantics are not reused.',
    },
    flow: {
      eyebrow: 'Business lifecycle',
      title: 'From agent approval to spread posting',
      qualification: {
        title: 'Agent qualification',
        description: 'Approve eligibility, multiplier, business scope, and validity.',
      },
      offer: {
        title: 'Distribution offer',
        description: 'Set downstream multiplier, channel identity, and products.',
      },
      attribution: {
        title: 'Customer attribution',
        description: 'Bind activated customers with locking and transfer rules.',
      },
      billing: {
        title: 'Usage pricing',
        description: 'Create agent cost and customer charge snapshots from usage.',
      },
      ledger: {
        title: 'Spread posting',
        description: 'Record debits, spread, reversals, and the full audit trail.',
      },
    },
    architecture: {
      eyebrow: 'System design',
      title: 'Six-layer distribution architecture',
      scope: 'Independent domain model',
      coreObjects: 'Core objects',
      responsibility: 'Responsibility',
      qualification: {
        title: 'Agent qualification',
        description: 'Defines who can distribute and the cost boundary available to them.',
        objects: 'Agent profile, status, exclusive multiplier, validity',
        responsibility: 'Review, activation, tier changes, scope grants',
      },
      offer: {
        title: 'Distribution offer',
        description: 'Holds multiplier strategies for the agent\'s channels and products.',
        objects: 'Distribution multiplier, products, channel code, limits',
        responsibility: 'Floor validation, allowed range, offer versions',
      },
      attribution: {
        title: 'Customer attribution',
        description: 'Creates one auditable business relationship between customer and agent.',
        objects: 'Agent-customer binding, source, activation, lock period',
        responsibility: 'Bind, transfer, release, resolve conflicts',
      },
      pricing: {
        title: 'Usage pricing',
        description: 'Resolves multipliers during billing and creates immutable price snapshots.',
        objects: 'Base amount, agent multiplier, retail multiplier, rule version',
        responsibility: 'Resolve, quantize, snapshot prices',
      },
      ledger: {
        title: 'Distribution ledger',
        description: 'Records agent cost and multiplier spread as reconcilable entries.',
        objects: 'Cost debit, spread credit, refund reversal, idempotency key',
        responsibility: 'Post, reconcile, export, repair exceptions',
      },
      governance: {
        title: 'Governance and risk',
        description: 'Limits abnormal pricing and attribution abuse with full evidence.',
        objects: 'Multiplier bounds, risk events, freezes, audit logs',
        responsibility: 'Validate, alert, freeze, review',
      },
    },
    boundaries: {
      eyebrow: 'Implementation constraints',
      title: 'Three boundaries the implementation must preserve',
      pricing: {
        title: 'The platform controls multiplier bounds',
        description: 'Retail multiplier cannot be lower than the agent multiplier and remains subject to product floors, maximum spread, and scope limits.',
      },
      snapshot: {
        title: 'Usage snapshots are the accounting source',
        description: 'Multiplier changes never rewrite history. A usage event posts once, and refunds reverse the original event.',
      },
      risk: {
        title: 'Agent relationships stay auditable',
        description: 'Binding, transfer, multiplier changes, freezes, and releases record actor, reason, before and after values, and timestamp.',
      },
    },
  },
}
