-- Rename the saved-definition table to "assessments" (GitHub-Actions "workflow").
-- The per-case "scenario" vocabulary (scenario_results, YAML scenarios:) is unchanged.
ALTER TABLE saved_scenarios RENAME TO assessments;
ALTER INDEX idx_saved_scenarios_updated_at RENAME TO idx_assessments_updated_at;

-- name becomes a human-addressable unique slug (each assessment serializes to
-- <name>.yaml). Disambiguate any pre-existing duplicate names before adding the
-- UNIQUE constraint: keep the oldest row's name, suffix the rest, and log each
-- rename so the operation is not silent.
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT id, name,
               row_number() OVER (PARTITION BY name ORDER BY created_at, id) AS rn
          FROM assessments
    LOOP
        IF r.rn > 1 THEN
            UPDATE assessments
               SET name = r.name || '-' || substr(r.id::text, 1, 8)
             WHERE id = r.id;
            RAISE NOTICE 'assessment-vocabulary-refactor: renamed duplicate assessment "%" to "%-%"',
                r.name, r.name, substr(r.id::text, 1, 8);
        END IF;
    END LOOP;
END $$;

ALTER TABLE assessments ADD CONSTRAINT assessments_name_key UNIQUE (name);
