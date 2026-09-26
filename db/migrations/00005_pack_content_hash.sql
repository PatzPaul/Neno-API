-- +goose Up
-- cmd/packs only publishes a new pack version when the content hash changes.
ALTER TABLE packs ADD COLUMN content_hash text;
-- Drop the [SAMPLE] manifest rows that pointed at a non-existent CDN; real rows come from cmd/packs.
DELETE FROM packs WHERE url LIKE 'https://cdn.example.invalid/%';

-- +goose Down
-- The deleted [SAMPLE] rows are not restored (re-run `make seed` on a dev DB if needed).
ALTER TABLE packs DROP COLUMN IF EXISTS content_hash;
