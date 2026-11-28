-- Runs on the application's schema-scoped connection during independent migration.
DO $migration$
DECLARE
    task_tables regclass[] := ARRAY[to_regclass('tasks'), to_regclass('creative_tasks')];
    task_table regclass;
    highest bigint;
    largest_in_table bigint;
    added bigint;
BEGIN
    -- Fresh installations and already upgraded schemas need no backfill.
    IF to_regclass('task_display_number_seq') IS NULL AND NOT EXISTS (
        SELECT FROM unnest(task_tables) AS t(oid)
        WHERE t.oid IS NOT NULL AND NOT EXISTS (
            SELECT FROM pg_attribute WHERE attrelid = t.oid AND attname = 'display_number' AND NOT attisdropped
        )
    ) THEN
        RETURN;
    END IF;

    INSERT INTO task_number_counters (id, name, created_at, last_number)
        VALUES ('000000000000000000000000001', 'task', now(), 0) ON CONFLICT (name) DO NOTHING;
    SELECT last_number INTO STRICT highest FROM task_number_counters WHERE name = 'task' FOR UPDATE;

    FOREACH task_table IN ARRAY task_tables LOOP
        IF task_table IS NOT NULL THEN
            EXECUTE 'LOCK TABLE ' || task_table || ' IN ACCESS EXCLUSIVE MODE';
            EXECUTE 'ALTER TABLE ' || task_table || ' ADD COLUMN IF NOT EXISTS display_number bigint';
            EXECUTE 'ALTER TABLE ' || task_table || ' ALTER COLUMN display_number DROP DEFAULT';
            EXECUTE 'SELECT max(display_number) FROM ' || task_table INTO largest_in_table;
            highest := greatest(highest, largest_in_table);
        END IF;
    END LOOP;

    IF to_regclass('task_display_number_seq') IS NOT NULL THEN
        SELECT greatest(highest, CASE WHEN is_called THEN last_value ELSE 0 END)
            INTO highest FROM task_display_number_seq;
    END IF;

    FOREACH task_table IN ARRAY task_tables LOOP
        IF task_table IS NOT NULL THEN
            EXECUTE 'WITH missing AS (
                SELECT id, row_number() OVER (ORDER BY created_at, id) AS n
                FROM ' || task_table || ' WHERE display_number IS NULL
            ) UPDATE ' || task_table || ' AS task SET display_number = $1 + missing.n
              FROM missing WHERE task.id = missing.id' USING highest;
            GET DIAGNOSTICS added = ROW_COUNT;
            highest := highest + added;
        END IF;
    END LOOP;

    UPDATE task_number_counters SET last_number = highest WHERE name = 'task';
    DROP FUNCTION IF EXISTS next_task_display_number();
    DROP SEQUENCE IF EXISTS task_display_number_seq;
END
$migration$;
