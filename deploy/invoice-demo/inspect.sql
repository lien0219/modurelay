\set ON_ERROR_STOP on
\pset pager off
\pset null '(null)'
\timing off

BEGIN READ ONLY;
DO $$
BEGIN
    IF current_database() <> 'invoice_demo' OR current_user <> 'invoice_demo' THEN
        RAISE EXCEPTION 'Refusing to inspect: database AND role must both be invoice_demo (database=%, role=%)', current_database(), current_user;
    END IF;
END $$;

\echo '\n=== Database / migration status ==='
SELECT current_database() AS database, current_user AS role,
       (SELECT COUNT(*) FROM schema_migrations) AS applied_migrations;
-- Migration identity is the full filename, so both 271 migrations are distinct.
SELECT filename, applied_at FROM schema_migrations
WHERE filename LIKE '271_%' OR filename = '281_invoice_documents.sql' ORDER BY filename;
SELECT to_regclass('public.invoice_applications') AS invoice_table,
       to_regclass('public.payment_orders') AS payment_table,
       to_regclass('public.invoice_documents') AS invoice_document_table;

\echo '\n=== Invoice setting / administrator first-login acknowledgement ==='
SELECT key, value, updated_at FROM settings
WHERE key IN ('invoice_enabled', 'invoice_demo_seed_version');
SELECT u.email AS admin_email,
       COALESCE((s.value::jsonb)->>'version', '(missing)') AS acknowledged_version,
       (s.value::jsonb)->>'accepted_at' AS accepted_at
FROM users u LEFT JOIN settings s ON s.key = 'admin_compliance_acknowledgement:' || u.id::text
WHERE u.email = 'invoice-admin@local.test' AND u.deleted_at IS NULL;

\echo '\n=== Demo users: wallet fields are USD, not CNY invoice eligibility ==='
SELECT id, email, username, role, status,
       balance AS wallet_balance_usd, total_recharged AS wallet_total_recharged_usd
FROM users WHERE deleted_at IS NULL
  AND email IN ('invoice-admin@local.test', 'invoice-user@local.test', 'invoice-low@local.test')
ORDER BY id;

\echo '\n=== Payment fixtures: only qualifying net paid CNY balance orders count ==='
SELECT p.id, u.email, p.out_trade_no, p.status, p.order_type,
       p.provider_snapshot->>'currency' AS currency,
       p.amount AS wallet_amount_usd, p.pay_amount AS original_cash_amount,
       p.refund_amount AS wallet_refund_usd, p.paid_at, p.user_notes AS fixture
FROM payment_orders p JOIN users u ON u.id = p.user_id
WHERE u.deleted_at IS NULL
  AND u.email IN ('invoice-admin@local.test', 'invoice-user@local.test', 'invoice-low@local.test')
ORDER BY u.id, p.id;

\echo '\n=== Eligibility: mirrors invoice_repo.go; pending/processing/issued reserve quota ==='
WITH order_funds AS (
    SELECT user_id, SUM(CASE
      WHEN order_type = 'balance'
       AND (UPPER(TRIM(provider_snapshot->>'currency')) = 'CNY'
         OR (COALESCE(TRIM(provider_snapshot->>'currency'), '') = ''
             AND COALESCE(NULLIF(TRIM(provider_snapshot->>'provider_key'), ''), provider_key) IN ('alipay','wxpay','easypay')))
       AND paid_at IS NOT NULL AND amount > 0 AND pay_amount > 0 THEN
        CASE
          WHEN status IN ('COMPLETED', 'REFUND_FAILED') THEN ROUND(pay_amount * 100)
          WHEN status IN ('PARTIALLY_REFUNDED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING') THEN
            CASE WHEN refund_amount <= 0 OR amount - refund_amount <= 0.01 THEN 0
                 ELSE GREATEST(0, ROUND(pay_amount * 100) - ROUND(pay_amount * refund_amount / amount * 100)) END
          ELSE 0
        END
      ELSE 0 END)::bigint AS total_cents
    FROM payment_orders GROUP BY user_id
), reserved AS (
    SELECT user_id, SUM(amount_cents) AS reserved_cents FROM invoice_applications
    WHERE status IN ('pending', 'processing', 'issued') GROUP BY user_id
), eligibility AS (
    SELECT u.email, COALESCE(f.total_cents, 0) AS total_cents,
           COALESCE(r.reserved_cents, 0) AS reserved_cents,
           GREATEST(0, COALESCE(f.total_cents, 0) - COALESCE(r.reserved_cents, 0)) AS available_cents
    FROM users u LEFT JOIN order_funds f ON f.user_id = u.id
    LEFT JOIN reserved r ON r.user_id = u.id
    WHERE u.deleted_at IS NULL
      AND u.email IN ('invoice-admin@local.test', 'invoice-user@local.test', 'invoice-low@local.test')
)
SELECT email, (total_cents / 100.00)::numeric(20,2) AS net_paid_cny,
       (reserved_cents / 100.00)::numeric(20,2) AS reserved_cny,
       (available_cents / 100.00)::numeric(20,2) AS available_cny,
       300.00 AS minimum_cny,
       CASE
         WHEN NOT COALESCE((SELECT value = 'true' FROM settings WHERE key = 'invoice_enabled'), false) THEN 'blocked: invoice disabled'
         WHEN total_cents < 30000 THEN 'blocked: net paid CNY < 300'
         WHEN available_cents < 30000 THEN 'blocked: available CNY < 300'
         ELSE 'can apply (each application >= CNY 300)'
       END AS application_gate
FROM eligibility ORDER BY email;

\echo '\n=== Invoice application history / full financial fields ==='
SELECT i.id, i.application_no, i.user_email,
       (i.amount_cents / 100.00)::numeric(20,2) AS amount_cny, i.status,
       i.invoice_type, i.title_type, i.title, i.tax_number,
       i.email AS recipient_email, i.contact, i.requested_issue_date,
       i.address, i.phone, i.bank_name, i.bank_account,
       i.remark, i.admin_remark, i.invoice_number, i.created_at, i.updated_at, i.processed_at,
       d.filename AS pdf_filename, d.size_bytes AS pdf_size_bytes,
       d.sha256 AS pdf_sha256, d.uploaded_at AS pdf_uploaded_at
FROM invoice_applications i JOIN users u ON u.id = i.user_id
LEFT JOIN invoice_documents d ON d.invoice_id = i.id
WHERE u.email IN ('invoice-admin@local.test', 'invoice-user@local.test', 'invoice-low@local.test')
ORDER BY i.created_at DESC, i.id DESC;
COMMIT;
