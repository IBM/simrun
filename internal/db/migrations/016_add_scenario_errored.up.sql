-- Distinguish execution errors (warmup/detonation/matching infrastructure
-- failures) from clean expectation mismatches. A scenario is "errored" when it
-- failed without producing per-expectation results; the value is computed at
-- write time. Existing rows default to false.
ALTER TABLE scenario_results ADD COLUMN errored boolean NOT NULL DEFAULT false;
