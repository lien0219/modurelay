-- Align legacy SMSPVA temporary orders with delivery-time settlement.
--
-- Real-account staging verification confirmed that SMSPVA can allocate a
-- temporary number before charging for an SMS. New code therefore keeps the
-- platform amount held until the first delivered SMS. This migration repairs
-- only legacy rows with positive evidence that no SMS was ever delivered:
-- first_sms_received_at IS NULL and no sms_messages row exists.
--
-- It is intentionally narrow and idempotent:
--   1) active/provider_unknown legacy captured rows become held reservations;
--   2) cancelled/expired legacy "refund pending" rows are returned to balance;
--   3) rows with any delivery evidence are untouched.

WITH moved_to_hold AS (
    UPDATE sms_orders o
    SET captured_amount = 0,
        settlement_status = 'held',
        refund_status = 'not_requested',
        provider_refund_status = 'not_requested',
        refund_reason = 'SMSPVA temporary order corrected to delivery-time settlement',
        updated_at = NOW()
    FROM sms_providers p
    WHERE o.provider_id = p.id
      AND p.code = 'smspva'
      AND o.product_type = 'temporary'
      AND o.status IN ('active','provider_unknown')
      AND o.provider_order_id <> ''
      AND o.settlement_status = 'captured'
      AND o.first_sms_received_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM sms_messages m WHERE m.order_id = o.id)
    RETURNING o.user_id, o.reserved_amount
),
hold_totals AS (
    SELECT user_id, SUM(reserved_amount) AS amount
    FROM moved_to_hold
    GROUP BY user_id
)
UPDATE users u
SET frozen_balance = COALESCE(u.frozen_balance,0) + h.amount,
    updated_at = NOW()
FROM hold_totals h
WHERE u.id = h.user_id;

WITH returned_terminal AS (
    UPDATE sms_orders o
    SET refunded_amount = reserved_amount,
        settlement_status = 'refunded',
        refund_status = 'approved',
        provider_refund_status = 'not_required',
        refund_reason = 'SMSPVA order cancelled/expired before SMS delivery; no provider refund was required',
        reconciliation_action = '',
        reconciliation_attempts = 0,
        reconcile_after = NULL,
        updated_at = NOW()
    FROM sms_providers p
    WHERE o.provider_id = p.id
      AND p.code = 'smspva'
      AND o.product_type = 'temporary'
      AND o.status IN ('cancelled','expired')
      AND o.settlement_status = 'captured'
      AND o.refund_status = 'pending'
      AND o.first_sms_received_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM sms_messages m WHERE m.order_id = o.id)
    RETURNING o.user_id, o.reserved_amount
),
refund_totals AS (
    SELECT user_id, SUM(reserved_amount) AS amount
    FROM returned_terminal
    GROUP BY user_id
)
UPDATE users u
SET balance = u.balance + r.amount,
    updated_at = NOW()
FROM refund_totals r
WHERE u.id = r.user_id;
