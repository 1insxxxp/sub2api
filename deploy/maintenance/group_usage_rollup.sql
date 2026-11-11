\set ON_ERROR_STOP on
BEGIN;
SET LOCAL application_name = 'sub2api-bounded-group-rollup';
SET LOCAL statement_timeout = '30s';
SET LOCAL lock_timeout = '2s';
SET LOCAL idle_in_transaction_session_timeout = '15s';
SET LOCAL work_mem = '8MB';
SET LOCAL temp_file_limit = '64MB';
SET LOCAL max_parallel_workers_per_gather = 0;
SET LOCAL jit = off;
SET LOCAL TIME ZONE 'Asia/Shanghai';

CREATE TEMP TABLE rollup_bounds ON COMMIT DROP AS
SELECT :'rollup_date'::date AS bucket_date,
       (:'rollup_date'::date::timestamp AT TIME ZONE 'Asia/Shanghai') AS starts_at,
       ((:'rollup_date'::date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai') AS ends_at;

DO $guard$
BEGIN
    IF (SELECT bucket_date FROM rollup_bounds) >= (clock_timestamp() AT TIME ZONE 'Asia/Shanghai')::date THEN
        RAISE EXCEPTION 'Only complete historical days may be published';
    END IF;
    IF NOT pg_try_advisory_xact_lock(hashtextextended('sub2api:bounded-group-daily-rollup', 0)) THEN
        RAISE EXCEPTION 'Another bounded group rollup is running';
    END IF;
END
$guard$;

-- Aggregate one indexed day before acquiring the publisher fence.
CREATE TEMP TABLE rollup_source ON COMMIT DROP AS
SELECT ul.group_id,
       COUNT(*) AS source_rows,
       BOOL_OR(ul.actual_cost IS NULL OR ul.actual_cost < 0
           OR ul.actual_cost::text IN ('NaN', 'Infinity', '-Infinity')) AS invalid_amount,
       COALESCE(SUM(ul.actual_cost), 0) AS actual_cost,
       COALESCE(SUM(ul.actual_cost) FILTER (WHERE EXISTS (
           SELECT 1 FROM sub_admin_commission_exclusions ce
           WHERE ce.user_id = ul.user_id
             AND ce.started_at <= ul.created_at
             AND (ce.ended_at IS NULL OR ul.created_at < ce.ended_at)
       )), 0) AS commission_excluded_cost
FROM usage_logs ul
WHERE ul.created_at >= (:'rollup_date'::date::timestamp AT TIME ZONE 'Asia/Shanghai')
  AND ul.created_at < ((:'rollup_date'::date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')
  AND ul.group_id IS NOT NULL
GROUP BY ul.group_id;

-- Native publishers use FOR UPDATE. KEY SHARE fences their publication while
-- remaining compatible with the normal usage INSERT trigger's KEY SHARE.
DO $fence$
BEGIN
    PERFORM id FROM usage_group_rollup_state
    WHERE id = 1 AND timezone_name = 'Asia/Shanghai' FOR KEY SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Group rollup publisher state is missing or has an unexpected timezone';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM rollup_source) AND EXISTS (
        SELECT 1 FROM usage_group_daily_rollups r, rollup_bounds b
        WHERE r.bucket_date = b.bucket_date
    ) THEN
        RAISE EXCEPTION 'Source day is empty but published history exists';
    END IF;
    IF EXISTS (
        SELECT 1 FROM rollup_source
        WHERE invalid_amount OR actual_cost::text IN ('NaN', 'Infinity', '-Infinity')
           OR commission_excluded_cost::text IN ('NaN', 'Infinity', '-Infinity')
           OR actual_cost < 0 OR commission_excluded_cost < 0
           OR commission_excluded_cost > actual_cost
    ) THEN
        RAISE EXCEPTION 'Invalid source monetary totals';
    END IF;
END
$fence$;

CREATE TEMP TABLE rollup_before ON COMMIT DROP AS
SELECT r.* FROM usage_group_daily_rollups r, rollup_bounds b
WHERE r.bucket_date = b.bucket_date;

DELETE FROM usage_group_daily_rollups r
USING rollup_bounds b
WHERE r.bucket_date = b.bucket_date
  AND NOT EXISTS (SELECT 1 FROM rollup_source s WHERE s.group_id = r.group_id);

INSERT INTO usage_group_daily_rollups (bucket_date, group_id, actual_cost, commission_excluded_cost, computed_at)
SELECT b.bucket_date, s.group_id, s.actual_cost, s.commission_excluded_cost, clock_timestamp()
FROM rollup_source s CROSS JOIN rollup_bounds b
ON CONFLICT (bucket_date, group_id) DO UPDATE SET
    actual_cost = EXCLUDED.actual_cost,
    commission_excluded_cost = EXCLUDED.commission_excluded_cost,
    computed_at = EXCLUDED.computed_at;

DO $verify$
BEGIN
    IF EXISTS (
        SELECT s.group_id FROM rollup_source s
        FULL JOIN (
            SELECT r.* FROM usage_group_daily_rollups r, rollup_bounds b
            WHERE r.bucket_date = b.bucket_date
        ) r ON r.group_id = s.group_id
        WHERE s.group_id IS NULL OR r.group_id IS NULL
           OR r.actual_cost IS DISTINCT FROM s.actual_cost
           OR r.commission_excluded_cost IS DISTINCT FROM s.commission_excluded_cost
    ) THEN
        RAISE EXCEPTION 'Published totals differ from the source snapshot';
    END IF;
END
$verify$;

SELECT jsonb_build_object(
    'day', b.bucket_date,
    'source_rows', (SELECT COALESCE(SUM(source_rows), 0) FROM rollup_source),
    'group_count', (SELECT COUNT(*) FROM rollup_source),
    'actual_cost', (SELECT COALESCE(SUM(actual_cost), 0) FROM rollup_source),
    'commission_excluded_cost', (SELECT COALESCE(SUM(commission_excluded_cost), 0) FROM rollup_source),
    'before', COALESCE((SELECT jsonb_agg(to_jsonb(r) ORDER BY group_id) FROM rollup_before r), '[]'::jsonb),
    'after', COALESCE((SELECT jsonb_agg(to_jsonb(r) ORDER BY group_id)
        FROM usage_group_daily_rollups r WHERE r.bucket_date = b.bucket_date), '[]'::jsonb)
)
FROM rollup_bounds b;
COMMIT;
