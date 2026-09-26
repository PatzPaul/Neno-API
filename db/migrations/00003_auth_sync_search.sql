-- +goose Up
-- Identity comes from Keycloak (users.id = token sub).
ALTER TABLE users ADD COLUMN email text, ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

-- Server receive time for delta sync. LWW compares the client's updated_at, but pulls page on this column so
-- skewed device clocks can't hide rows from other devices.
ALTER TABLE user_marks    ADD COLUMN server_updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE user_answers  ADD COLUMN server_updated_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE user_progress ADD COLUMN server_updated_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX user_marks_pull_idx    ON user_marks (user_id, server_updated_at);
CREATE INDEX user_answers_pull_idx  ON user_answers (user_id, server_updated_at);
CREATE INDEX user_progress_pull_idx ON user_progress (user_id, server_updated_at);
CREATE INDEX user_answers_target_idx ON user_answers (user_id, target, target_ref);

-- Like counts. Counted as DISTINCT user_id, so a racing double-like can never inflate the number.
CREATE INDEX user_marks_like_idx ON user_marks (target, target_ref) WHERE kind = 'like' AND deleted_at IS NULL;

-- unaccent() is only STABLE; this wrapper lets search use trigram indexes on accent-folded text.
-- +goose StatementBegin
CREATE FUNCTION immutable_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
  AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;
-- +goose StatementEnd

CREATE INDEX belief_texts_title_trgm_idx ON belief_texts USING gin (immutable_unaccent(title) gin_trgm_ops);
CREATE INDEX belief_texts_body_trgm_idx  ON belief_texts USING gin (immutable_unaccent(body) gin_trgm_ops);
CREATE INDEX hymns_title_trgm_idx        ON hymns USING gin (immutable_unaccent(title) gin_trgm_ops);
CREATE INDEX egw_paragraphs_trgm_idx     ON egw_paragraphs USING gin (immutable_unaccent(text) gin_trgm_ops);
CREATE INDEX verses_unaccent_trgm_idx    ON verses USING gin (immutable_unaccent(text) gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS verses_unaccent_trgm_idx, egw_paragraphs_trgm_idx, hymns_title_trgm_idx,
  belief_texts_body_trgm_idx, belief_texts_title_trgm_idx, user_marks_like_idx, user_answers_target_idx,
  user_progress_pull_idx, user_answers_pull_idx, user_marks_pull_idx;
DROP FUNCTION IF EXISTS immutable_unaccent(text);
ALTER TABLE user_progress DROP COLUMN IF EXISTS server_updated_at;
ALTER TABLE user_answers  DROP COLUMN IF EXISTS server_updated_at;
ALTER TABLE user_marks    DROP COLUMN IF EXISTS server_updated_at;
ALTER TABLE users DROP COLUMN IF EXISTS updated_at, DROP COLUMN IF EXISTS email;
