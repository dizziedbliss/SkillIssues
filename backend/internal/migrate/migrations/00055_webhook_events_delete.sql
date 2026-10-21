-- +goose Up
-- A webhook whose write failed forgets its delivery id, so GitHub's
-- redelivery is applied instead of skipped as already seen.
GRANT DELETE ON webhook_events TO skillissues_api;

-- +goose Down
REVOKE DELETE ON webhook_events FROM skillissues_api;
