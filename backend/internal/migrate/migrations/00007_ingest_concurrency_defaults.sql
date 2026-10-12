-- D20: default total in-flight GraphQL requests was sized for a 20-vCPU
-- dev box (4 routine + 2 priority + 8 on-demand = 14). The target
-- deployment is 2 vCPU; bring the default down to ~4 total. Still a live
-- setting — change it at runtime via PUT /admin/config if a bigger box
-- warrants more.
--
-- ingest.routine_workers now backs the Initial/Freshness pool and
-- ingest.priority_workers the Deep Diver pool (Phase 5) — same keys,
-- repurposed meaning; ingest.ondemand_workers stays reserved for the
-- on-demand issue-body read path (Phase 6, not wired up yet).
--
-- +goose Up

UPDATE settings SET value = '2', updated_by = 'migration_00007' WHERE key = 'ingest.routine_workers' AND value = '4';
UPDATE settings SET value = '1', updated_by = 'migration_00007' WHERE key = 'ingest.priority_workers' AND value = '2';
UPDATE settings SET value = '1', updated_by = 'migration_00007' WHERE key = 'ingest.ondemand_workers' AND value = '8';

-- +goose Down

UPDATE settings SET value = '4', updated_by = 'migration_00007_down' WHERE key = 'ingest.routine_workers' AND value = '2';
UPDATE settings SET value = '2', updated_by = 'migration_00007_down' WHERE key = 'ingest.priority_workers' AND value = '1';
UPDATE settings SET value = '8', updated_by = 'migration_00007_down' WHERE key = 'ingest.ondemand_workers' AND value = '1';
