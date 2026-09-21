-- Reconcile interrupted/partial installs of the additive SMS rental tables.
--
-- Migrations 258-261 intentionally use IF NOT EXISTS so a deployment can be
-- resumed without replacing an existing table. A pre-existing partial table
-- can nevertheless be missing a foreign key or a uniqueness constraint. This
-- forward-only repair adds constraints only when both sides of the relation
-- exist; foreign keys are NOT VALID so historical orphan rows remain readable
-- while all future writes are protected. Unique indexes are created only when
-- the existing data is already unique. No rows are deleted or rewritten.

-- All relations in 258-261 are single-column foreign keys. Keep their
-- definitions in one table so the guards and equivalence test cannot drift.
DO $$
DECLARE
    fk RECORD;
    local_table_exists BOOLEAN;
    local_column_exists BOOLEAN;
    ref_table_exists BOOLEAN;
    ref_column_exists BOOLEAN;
    equivalent_fk BOOLEAN;
    constraint_name TEXT;
    suffix INTEGER;
BEGIN
    FOR fk IN
        SELECT *
        FROM (VALUES
            ('sms_order_services', 'order_id', 'sms_orders', 'id', 'c', 'sms_order_services_order_fk'),
            ('sms_order_services', 'service_id', 'sms_services', 'id', 'r', 'sms_order_services_service_fk'),
            ('sms_rental_service_quotes', 'user_id', 'users', 'id', 'c', 'sms_rental_service_quotes_user_fk'),
            ('sms_rental_service_quotes', 'order_id', 'sms_orders', 'id', 'c', 'sms_rental_service_quotes_order_fk'),
            ('sms_rental_service_quotes', 'service_id', 'sms_services', 'id', 'r', 'sms_rental_service_quotes_service_fk'),
            ('sms_rental_restore_quotes', 'user_id', 'users', 'id', 'c', 'sms_rental_restore_quotes_user_fk'),
            ('sms_rental_restore_quotes', 'source_order_id', 'sms_orders', 'id', 'c', 'sms_rental_restore_quotes_source_order_fk'),
            ('sms_rental_service_charges', 'user_id', 'users', 'id', 'r', 'sms_rental_service_charges_user_fk_restrict'),
            ('sms_rental_service_charges', 'order_id', 'sms_orders', 'id', 'r', 'sms_rental_service_charges_order_fk_restrict'),
            ('sms_rental_service_charges', 'service_id', 'sms_services', 'id', 'r', 'sms_rental_service_charges_service_fk'),
            ('sms_rental_service_charges', 'quote_id', 'sms_rental_service_quotes', 'id', 'r', 'sms_rental_service_charges_quote_fk'),
            ('sms_rental_multi_service_quotes', 'user_id', 'users', 'id', 'c', 'sms_rental_multi_quotes_user_fk'),
            ('sms_rental_multi_service_quotes', 'channel_id', 'sms_channels', 'id', 'r', 'sms_rental_multi_quotes_channel_fk'),
            ('sms_rental_multi_service_quotes', 'provider_id', 'sms_providers', 'id', 'r', 'sms_rental_multi_quotes_provider_fk'),
            ('sms_rental_multi_service_quotes', 'country_id', 'sms_countries', 'id', 'r', 'sms_rental_multi_quotes_country_fk'),
            ('sms_rental_multi_service_quotes', 'consumed_order_id', 'sms_orders', 'id', 'r', 'sms_rental_multi_quotes_consumed_order_fk'),
            ('sms_rental_recovery_operations', 'user_id', 'users', 'id', 'r', 'sms_rental_recovery_operations_user_fk'),
            ('sms_rental_recovery_operations', 'source_order_id', 'sms_orders', 'id', 'r', 'sms_rental_recovery_operations_source_order_fk'),
            ('sms_rental_recovery_operations', 'result_order_id', 'sms_orders', 'id', 'r', 'sms_rental_recovery_operations_result_order_fk'),
            ('sms_rental_recovery_operations', 'restore_quote_id', 'sms_rental_restore_quotes', 'id', 'r', 'sms_rental_recovery_operations_restore_quote_fk'),
            ('sms_rental_recovery_operations', 'service_quote_id', 'sms_rental_service_quotes', 'id', 'r', 'sms_rental_recovery_operations_service_quote_fk')
        ) AS definitions(local_table, local_column, ref_table, ref_column, delete_action, constraint_name)
    LOOP
        SELECT to_regclass(format('public.%I', fk.local_table)) IS NOT NULL INTO local_table_exists;
        SELECT to_regclass(format('public.%I', fk.ref_table)) IS NOT NULL INTO ref_table_exists;
        IF NOT local_table_exists OR NOT ref_table_exists THEN
            CONTINUE;
        END IF;

        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = fk.local_table
              AND column_name = fk.local_column
        ) INTO local_column_exists;
        SELECT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = fk.ref_table
              AND column_name = fk.ref_column
        ) INTO ref_column_exists;
        IF NOT local_column_exists OR NOT ref_column_exists THEN
            CONTINUE;
        END IF;

        -- Compare the actual relation definition, not only its name. This
        -- recognizes constraints left by 258-261 under a different name.
        SELECT EXISTS (
            SELECT 1
            FROM pg_constraint c
            WHERE c.contype = 'f'
              AND c.conrelid = to_regclass(format('public.%I', fk.local_table))
              AND c.confrelid = to_regclass(format('public.%I', fk.ref_table))
              AND c.confdeltype = fk.delete_action::"char"
            AND (SELECT array_agg(a.attname::text ORDER BY u.ord)
                   FROM unnest(c.conkey) WITH ORDINALITY AS u(attnum, ord)
                   JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = u.attnum)
                    = ARRAY[fk.local_column]::text[]
              AND (SELECT array_agg(a.attname::text ORDER BY u.ord)
                   FROM unnest(c.confkey) WITH ORDINALITY AS u(attnum, ord)
                   JOIN pg_attribute a ON a.attrelid = c.confrelid AND a.attnum = u.attnum)
                    = ARRAY[fk.ref_column]::text[]
        ) INTO equivalent_fk;
        IF equivalent_fk THEN
            CONTINUE;
        END IF;

        -- Avoid a name collision with a non-equivalent legacy constraint.
        constraint_name := fk.constraint_name;
        suffix := 0;
        WHILE EXISTS (
            SELECT 1 FROM pg_constraint
            WHERE conrelid = to_regclass(format('public.%I', fk.local_table))
              AND conname = constraint_name
        ) LOOP
            suffix := suffix + 1;
            constraint_name := fk.constraint_name || '_repair_' || suffix::text;
        END LOOP;

        EXECUTE format(
            'ALTER TABLE public.%I ADD CONSTRAINT %I FOREIGN KEY (%I) REFERENCES public.%I(%I) ON DELETE %s NOT VALID',
            fk.local_table, constraint_name, fk.local_column, fk.ref_table,
            fk.ref_column, CASE WHEN fk.delete_action = 'c' THEN 'CASCADE' ELSE 'RESTRICT' END
        );
    END LOOP;
END $$;

-- Repair the unique identities used for idempotency and one-time quote
-- consumption. Existing unique indexes/constraints are compared by table,
-- ordered key columns, and (when present) predicate; index names are ignored.
DO $$
DECLARE
    idx RECORD;
    table_exists BOOLEAN;
    all_columns_exist BOOLEAN;
    equivalent_index BOOLEAN;
    duplicate_free BOOLEAN;
    null_guard TEXT;
    where_sql TEXT;
    index_name TEXT;
    suffix INTEGER;
BEGIN
    FOR idx IN
        SELECT *
        FROM (VALUES
            ('sms_order_services', 'order_id,service_id', NULL::TEXT, 'uq_sms_order_services_order_service', TRUE),
            ('sms_rental_service_charges', 'order_id,idempotency_key', NULL::TEXT, 'uq_sms_rental_service_charges_order_idempotency', TRUE),
            ('sms_rental_service_charges', 'quote_id', NULL::TEXT, 'uq_sms_rental_service_charges_quote', TRUE),
            ('sms_rental_multi_service_quotes', 'consumed_order_id', 'consumed_order_id IS NOT NULL', 'uq_sms_rental_multi_quotes_consumed_order', FALSE),
            ('sms_rental_recovery_operations', 'user_id,operation_type,idempotency_key', NULL::TEXT, 'uq_sms_rental_recovery_user_operation_key', TRUE),
            ('sms_rental_recovery_operations', 'restore_quote_id', 'restore_quote_id IS NOT NULL', 'uq_sms_rental_recovery_restore_quote', FALSE),
            ('sms_rental_recovery_operations', 'service_quote_id', 'service_quote_id IS NOT NULL', 'uq_sms_rental_recovery_service_quote', FALSE)
        ) AS definitions(table_name, columns_sql, predicate_sql, index_name, reject_nulls)
    LOOP
        SELECT to_regclass(format('public.%I', idx.table_name)) IS NOT NULL INTO table_exists;
        IF NOT table_exists THEN
            CONTINUE;
        END IF;

        SELECT NOT EXISTS (
            SELECT 1
            FROM unnest(string_to_array(idx.columns_sql, ',')) AS wanted(column_name)
            WHERE NOT EXISTS (
                SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'public'
                  AND table_name = idx.table_name
                  AND column_name = btrim(wanted.column_name)
            )
        ) INTO all_columns_exist;
        IF NOT all_columns_exist THEN
            CONTINUE;
        END IF;

        -- A non-partial unique index on a nullable key is equivalent to the
        -- IS NOT NULL form below: PostgreSQL permits repeated NULLs in both.
        SELECT EXISTS (
            SELECT 1
            FROM pg_index i
            WHERE i.indrelid = to_regclass(format('public.%I', idx.table_name))
              AND i.indisunique
              AND (SELECT array_agg(a.attname::text ORDER BY k.ord)
                   FROM unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord)
                   JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
                   WHERE k.ord <= i.indnkeyatts)
                    = string_to_array(idx.columns_sql, ',')::text[]
              AND (
                    (idx.predicate_sql IS NULL AND i.indpred IS NULL)
                 OR (idx.predicate_sql IS NOT NULL AND i.indpred IS NULL)
                 OR (idx.predicate_sql IS NOT NULL AND regexp_replace(lower(pg_get_expr(i.indpred, i.indrelid)), '[\s()]+', '', 'g') = regexp_replace(lower(idx.predicate_sql), '[\s()]+', '', 'g'))
              )
        ) INTO equivalent_index;
        IF equivalent_index THEN
            CONTINUE;
        END IF;

        where_sql := CASE WHEN idx.predicate_sql IS NULL THEN '' ELSE ' WHERE ' || idx.predicate_sql END;
        null_guard := '';
        IF idx.reject_nulls THEN
            SELECT string_agg(format('%I IS NULL', btrim(column_name)), ' OR ')
            INTO null_guard
            FROM unnest(string_to_array(idx.columns_sql, ',')) AS wanted(column_name);
            null_guard := ' OR ' || null_guard;
        END IF;
        EXECUTE format(
            'SELECT NOT EXISTS (SELECT 1 FROM (SELECT %s FROM public.%I%s GROUP BY %s HAVING COUNT(*) > 1%s) duplicates)',
            idx.columns_sql, idx.table_name, where_sql, idx.columns_sql, null_guard
        ) INTO duplicate_free;
        IF NOT duplicate_free THEN
            CONTINUE;
        END IF;

        -- Avoid a name collision with a non-equivalent legacy index while
        -- keeping the stable name for the normal first repair run.
        index_name := idx.index_name;
        suffix := 0;
        WHILE to_regclass(format('public.%I', index_name)) IS NOT NULL LOOP
            suffix := suffix + 1;
            index_name := idx.index_name || '_repair_' || suffix::text;
        END LOOP;
        EXECUTE format('CREATE UNIQUE INDEX %I ON public.%I(%s)%s', index_name, idx.table_name, idx.columns_sql, where_sql);
    END LOOP;
END $$;
