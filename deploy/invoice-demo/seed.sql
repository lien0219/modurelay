\set ON_ERROR_STOP on
\pset pager off

-- LOCAL DEMO ONLY. Run after the application has applied its migrations.
-- These are synthetic local ledger fixtures, never real gateway transactions.
BEGIN;
DO $$
BEGIN
    IF current_database() <> 'invoice_demo' OR current_user <> 'invoice_demo' THEN
        RAISE EXCEPTION 'Refusing to seed: database AND role must both be invoice_demo (database=%, role=%)', current_database(), current_user;
    END IF;
    IF to_regclass('invoice_applications') IS NULL THEN
        RAISE EXCEPTION 'Invoice schema is missing. Start the invoice demo backend and wait for migrations first.';
    END IF;
END $$;
SELECT pg_advisory_xact_lock(hashtextextended('invoice-demo-seed-v1', 0));

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Keep existing passwords, balances, application history and account state on
-- subsequent runs. The startup bootstrap may already have created the admin.
INSERT INTO users (email, password_hash, role, username, notes, signup_source)
SELECT email, crypt('InvoiceDemo!2026', gen_salt('bf', 10)), role, username,
       'LOCAL invoice demo fixture; no real payment or invoice', 'email'
FROM (VALUES
    ('invoice-admin@local.test', 'admin', '发票测试管理员'),
    ('invoice-user@local.test', 'user', '发票流程测试用户'),
    ('invoice-low@local.test', 'user', '不足300元测试用户')
) AS demo(email, role, username)
WHERE true
ON CONFLICT (email) WHERE deleted_at IS NULL DO NOTHING;

DO $$
BEGIN
    IF (SELECT COUNT(*) FROM users
        WHERE deleted_at IS NULL AND status = 'active'
          AND ((email = 'invoice-admin@local.test' AND role = 'admin')
            OR (email IN ('invoice-user@local.test', 'invoice-low@local.test') AND role = 'user'))) <> 3 THEN
        RAISE EXCEPTION 'Demo accounts already exist with unexpected roles/status. Existing accounts were not overwritten.';
    END IF;
END $$;

INSERT INTO auth_identities (user_id, provider_type, provider_key, provider_subject, verified_at, metadata)
SELECT id, 'email', 'email', LOWER(email), NOW(), '{"source":"invoice_demo_local_seed"}'::jsonb
FROM users
WHERE deleted_at IS NULL
  AND email IN ('invoice-admin@local.test', 'invoice-user@local.test', 'invoice-low@local.test')
ON CONFLICT (provider_type, provider_key, provider_subject) DO NOTHING;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM users u
        JOIN auth_identities ai ON ai.provider_type = 'email'
             AND ai.provider_key = 'email' AND ai.provider_subject = LOWER(u.email)
        WHERE u.deleted_at IS NULL
          AND u.email IN ('invoice-admin@local.test', 'invoice-user@local.test', 'invoice-low@local.test')
          AND ai.user_id <> u.id
    ) THEN
        RAISE EXCEPTION 'An existing email identity belongs to another account. No identity was reassigned.';
    END IF;
END $$;

-- Only the first seed enables invoices. Re-running seed preserves the toggle
-- so the admin can test disabling it. The marker is confined to this demo DB.
WITH first_seed AS (
    INSERT INTO settings (key, value, updated_at)
    VALUES ('invoice_demo_seed_version', '1', NOW())
    ON CONFLICT (key) DO NOTHING
    RETURNING id
)
INSERT INTO settings (key, value, updated_at)
SELECT 'invoice_enabled', 'true', NOW() FROM first_seed
WHERE true
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();

-- Admin compliance acknowledgement is deliberately left to the administrator
-- in the existing first-login UI. Seeding data does not accept that commitment.

-- amount/balance/total_recharged are USD wallet units. pay_amount is the
-- original cash currency. Invoice eligibility uses verified CNY cash payments.
-- Stable out_trade_no values make the insert and wallet credits idempotent.
WITH fixtures(email, trade_no, wallet_amount, cash_amount, currency, order_type, status, label) AS (
    VALUES
      ('invoice-user@local.test', 'LOCAL-INVOICE-DEMO-USER-CNY-600', 60.00::numeric, 600.00::numeric, 'CNY', 'balance', 'COMPLETED', '合格：人民币实付600元'),
      ('invoice-low@local.test', 'LOCAL-INVOICE-DEMO-LOW-CNY-29999', 29.99, 299.99, 'CNY', 'balance', 'COMPLETED', '不足：人民币实付299.99元'),
      ('invoice-low@local.test', 'LOCAL-INVOICE-DEMO-LOW-USD', 10.00, 10.00, 'USD', 'balance', 'COMPLETED', '不计额度：美元充值'),
      ('invoice-low@local.test', 'LOCAL-INVOICE-DEMO-LOW-UNPAID', 100.00, 1000.00, 'CNY', 'balance', 'EXPIRED', '不计额度：未付款已过期'),
      ('invoice-low@local.test', 'LOCAL-INVOICE-DEMO-LOW-SUB', 100.00, 1000.00, 'CNY', 'subscription', 'COMPLETED', '不计额度：订阅订单，仅发票排除对照，不创建订阅权益')
), new_orders AS (
    INSERT INTO payment_orders (
        user_id, user_email, user_name, user_notes,
        amount, pay_amount, fee_rate, bonus_amount, recharge_code,
        out_trade_no, payment_type, payment_trade_no,
        order_type, provider_key, provider_snapshot, status,
        expires_at, paid_at, completed_at, client_ip, src_host
    )
    SELECT u.id, u.email, u.username, f.label,
           f.wallet_amount, f.cash_amount, 0, 0, '',
           f.trade_no, 'demo-local', f.trade_no,
           f.order_type, 'demo-local',
           jsonb_build_object('currency', f.currency, 'provider_key', 'demo-local', 'local_fixture', true),
           f.status, NOW() - INTERVAL '1 day',
           CASE WHEN f.status = 'COMPLETED' THEN NOW() - INTERVAL '2 days' END,
           CASE WHEN f.status = 'COMPLETED' THEN NOW() - INTERVAL '2 days' END,
           '127.0.0.1', 'localhost'
    FROM fixtures f JOIN users u ON u.email = f.email AND u.deleted_at IS NULL
    WHERE true
    ON CONFLICT (out_trade_no) WHERE out_trade_no <> '' DO NOTHING
    RETURNING user_id, amount, order_type, status
), wallet_credits AS (
    SELECT user_id, SUM(amount) AS amount FROM new_orders
    WHERE order_type = 'balance' AND status = 'COMPLETED'
    GROUP BY user_id
)
UPDATE users u
SET balance = u.balance + c.amount,
    total_recharged = u.total_recharged + c.amount,
    updated_at = NOW()
FROM wallet_credits c WHERE u.id = c.user_id
  AND u.email IN ('invoice-user@local.test', 'invoice-low@local.test');

COMMIT;
\echo 'Local demo data ready. Re-running this file preserves invoice history, existing payments, balances and the invoice toggle.'
\echo 'The demo administrator must read and acknowledge the existing compliance screen on first login.'
\echo 'Use inspect.sql (or db.sh) to inspect the CNY eligibility and recorded applications.'
