SELECT json_build_object(
 'postgres_version',version(),
 'current_code_used',(SELECT COUNT(*) FROM pg_stat_activity),
 'current_code_limit',current_setting('max_connections')::bigint,
 'client_backends',(SELECT COUNT(*) FROM pg_stat_activity WHERE backend_type='client backend'),
 'other_backends',(SELECT COUNT(*) FROM pg_stat_activity WHERE backend_type<>'client backend'),
 'backend_types',(SELECT json_agg(x ORDER BY x.backend_type) FROM (SELECT backend_type,COUNT(*) AS processes FROM pg_stat_activity GROUP BY backend_type) x));
