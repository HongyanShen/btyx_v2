-- Freeze the catalog used by the last version that stored only individual choices.
-- Each group is backfilled only when its mode column is first introduced.
DO $migration$
BEGIN
    IF to_regclass('%[1]s.operators') IS NULL THEN
        RETURN;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_schema = '%[1]s'
        AND table_name = 'operators' AND column_name = 'all_pages'
    ) THEN
        ALTER TABLE %[1]s.operators ADD COLUMN all_pages boolean NOT NULL DEFAULT false;
        WITH legacy(keys) AS (VALUES (%[2]s::jsonb -> 'pages'))
        UPDATE %[1]s.operators SET all_pages = true FROM legacy
        WHERE NOT super_admin AND pages @> keys
          AND jsonb_array_length(pages) = jsonb_array_length(keys);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_schema = '%[1]s'
        AND table_name = 'operators' AND column_name = 'all_endpoints'
    ) THEN
        ALTER TABLE %[1]s.operators ADD COLUMN all_endpoints boolean NOT NULL DEFAULT false;
        WITH legacy(keys) AS (VALUES (%[2]s::jsonb -> 'endpoints'))
        UPDATE %[1]s.operators SET all_endpoints = true FROM legacy
        WHERE NOT super_admin AND endpoints @> keys
          AND jsonb_array_length(endpoints) = jsonb_array_length(keys);
    END IF;
END
$migration$;
