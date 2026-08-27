-- +goose Up
ALTER TABLE alert_policies
    ALTER COLUMN duration TYPE TEXT USING duration::TEXT,
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';
ALTER TABLE metrics_cache
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';
ALTER TABLE alerts
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';
ALTER TABLE alert_instances
    ALTER COLUMN start_time TYPE TIMESTAMPTZ USING start_time AT TIME ZONE 'UTC',
    ALTER COLUMN end_time TYPE TIMESTAMPTZ USING end_time AT TIME ZONE 'UTC';
ALTER TABLE timeline_events
    ALTER COLUMN timestamp TYPE TIMESTAMPTZ USING timestamp AT TIME ZONE 'UTC';

ALTER TABLE incidents
    ADD COLUMN IF NOT EXISTS title TEXT,
    ADD COLUMN IF NOT EXISTS start_time TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS end_time TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS namespace TEXT,
    ADD COLUMN IF NOT EXISTS policy_name TEXT,
    ADD COLUMN IF NOT EXISTS alert_name TEXT,
    ADD COLUMN IF NOT EXISTS severity TEXT,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resource_kind TEXT,
    ADD COLUMN IF NOT EXISTS resource_name TEXT,
    ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS suppressed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS reopen_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_event_at TIMESTAMPTZ;

-- +goose StatementBegin
DO $$
DECLARE
    target_column TEXT;
BEGIN
    FOREACH target_column IN ARRAY ARRAY['start_time', 'end_time', 'started_at', 'resolved_at', 'updated_at', 'acknowledged_at', 'suppressed_at', 'last_event_at']
    LOOP
        IF EXISTS (
            SELECT 1 FROM information_schema.columns AS c
            WHERE table_schema = 'public' AND table_name = 'incidents'
              AND c.column_name = target_column
              AND data_type = 'timestamp without time zone'
        ) THEN
            EXECUTE format(
                'ALTER TABLE incidents ALTER COLUMN %I TYPE TIMESTAMPTZ USING %I AT TIME ZONE ''UTC''',
                target_column, target_column
            );
        END IF;
    END LOOP;
END $$;
-- +goose StatementEnd

UPDATE incidents
SET namespace = COALESCE(namespace, ''),
    policy_name = COALESCE(policy_name, title, ''),
    started_at = COALESCE(started_at, start_time, now()),
    resolved_at = COALESCE(resolved_at, end_time),
    updated_at = COALESCE(updated_at, start_time, now()),
    last_event_at = COALESCE(last_event_at, updated_at, start_time, now());

ALTER TABLE incidents
    ALTER COLUMN namespace SET DEFAULT '',
    ALTER COLUMN namespace SET NOT NULL,
    ALTER COLUMN policy_name SET DEFAULT '',
    ALTER COLUMN policy_name SET NOT NULL,
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN started_at SET DEFAULT now(),
    ALTER COLUMN started_at SET NOT NULL,
    ALTER COLUMN updated_at SET DEFAULT now(),
    ALTER COLUMN updated_at SET NOT NULL,
    ALTER COLUMN last_event_at SET DEFAULT now(),
    ALTER COLUMN last_event_at SET NOT NULL;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_alert_instances_policy') THEN
        ALTER TABLE alert_instances
            ADD CONSTRAINT fk_alert_instances_policy FOREIGN KEY (policy_id)
            REFERENCES alert_policies(id) ON DELETE CASCADE NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_timeline_events_incident') THEN
        ALTER TABLE timeline_events
            ADD CONSTRAINT fk_timeline_events_incident FOREIGN KEY (incident_id)
            REFERENCES incidents(id) ON DELETE CASCADE NOT VALID;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF to_regclass('public.runtime_timeline') IS NOT NULL THEN
        EXECUTE 'ALTER TABLE runtime_timeline ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE ''UTC''';
    END IF;
    IF to_regclass('public.runtime_graph_nodes') IS NOT NULL THEN
        EXECUTE 'ALTER TABLE runtime_graph_nodes ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE ''UTC''';
    END IF;
    IF to_regclass('public.runtime_graph_edges') IS NOT NULL THEN
        EXECUTE 'ALTER TABLE runtime_graph_edges ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE ''UTC''';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- This compatibility migration intentionally retains normalized columns and data.
SELECT 1;
