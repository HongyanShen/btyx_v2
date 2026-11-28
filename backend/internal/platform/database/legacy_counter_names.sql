-- Give the original rows business keys before Ent enforces NOT NULL and uniqueness.
DO $migration$
BEGIN
    IF to_regclass('task_number_counters') IS NOT NULL AND NOT EXISTS (
        SELECT FROM pg_attribute WHERE attrelid = to_regclass('task_number_counters')
          AND attname = 'name' AND NOT attisdropped
    ) THEN
        ALTER TABLE task_number_counters ADD COLUMN name varchar;
        UPDATE task_number_counters SET name = CASE id
            WHEN '000000000000000000000000001' THEN 'task'
            WHEN '000000000000000000000000002' THEN 'nickname'
            ELSE id
        END;
    END IF;
END
$migration$;
