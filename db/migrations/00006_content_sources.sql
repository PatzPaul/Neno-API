-- +goose Up
-- Provenance for every imported text: where it came from, the exact bytes we used, and the licence terms.
CREATE TABLE content_sources (
  id serial PRIMARY KEY,
  kind text NOT NULL CHECK (kind IN ('bible', 'egw', 'hymnal', 'sabbath_school', 'beliefs')),
  ref text NOT NULL,                 -- e.g. translation code NENO, edition SC-sw
  url text NOT NULL,                 -- download / API location
  sha256 text NOT NULL,              -- of the downloaded archive or export
  license text NOT NULL,             -- e.g. "CC BY-SA 4.0", "public domain", "licensed: <agreement>"
  attribution text NOT NULL,         -- exact text the app must display
  license_url text,
  fetched_at timestamptz NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (kind, ref, sha256)
);

ALTER TABLE bible_translations
  ADD COLUMN attribution text,
  ADD COLUMN license_url text,
  ADD COLUMN source_id int REFERENCES content_sources(id);

-- +goose Down
ALTER TABLE bible_translations DROP COLUMN source_id, DROP COLUMN license_url, DROP COLUMN attribution;
DROP TABLE IF EXISTS content_sources;
