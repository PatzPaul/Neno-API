-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TYPE content_status AS ENUM ('draft','in_review','approved','published','rejected');
CREATE TYPE feed_kind AS ENUM ('verse','egw_quote','sabbath_school','prophecy','hymn','health','youth','devotional','sermon','audio_bible');
CREATE TYPE media_kind AS ENUM ('video','audio','image');
CREATE TYPE target_kind AS ENUM ('verse','egw_paragraph','belief','hymn','feed_item','ss_day','course_lesson');
CREATE TYPE user_role AS ENUM ('member','editor','reviewer','admin');

CREATE TABLE languages (
  code text PRIMARY KEY,           -- BCP 47: sw, sw-TZ, en, fr
  name text NOT NULL
);

-- ── Bible ───────────────────────────────────────────────
CREATE TABLE bible_translations (
  id serial PRIMARY KEY,
  code text UNIQUE NOT NULL,       -- SUV, KJV, LSG
  lang text NOT NULL REFERENCES languages(code),
  name text NOT NULL,
  license text NOT NULL,
  versification text NOT NULL DEFAULT 'eng',
  version int NOT NULL DEFAULT 1
);
CREATE TABLE bible_books (
  osis text PRIMARY KEY,           -- GEN … REV (USFM codes)
  ord int UNIQUE NOT NULL,
  testament char(2) NOT NULL CHECK (testament IN ('OT','NT'))
);
CREATE TABLE bible_book_names (
  book text REFERENCES bible_books(osis),
  lang text REFERENCES languages(code),
  name text NOT NULL, abbr text NOT NULL,
  PRIMARY KEY (book, lang)
);
CREATE TABLE verses (
  translation_id int REFERENCES bible_translations(id),
  osis_ref text NOT NULL,          -- JHN.3.16
  book text NOT NULL REFERENCES bible_books(osis),
  chapter int NOT NULL, verse int NOT NULL,
  text text NOT NULL,
  tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', text)) STORED,
  PRIMARY KEY (translation_id, osis_ref)
);
CREATE INDEX verses_chapter_idx ON verses (translation_id, book, chapter, verse);
CREATE INDEX verses_tsv_idx ON verses USING gin (tsv);
CREATE INDEX verses_trgm_idx ON verses USING gin (text gin_trgm_ops);

-- ── Ellen G. White ──────────────────────────────────────
CREATE TABLE egw_books (code text PRIMARY KEY, original_title text NOT NULL); -- SC, DA, GC, PP, MH
CREATE TABLE egw_editions (
  id serial PRIMARY KEY,
  book_code text NOT NULL REFERENCES egw_books(code),
  lang text NOT NULL REFERENCES languages(code),
  title text NOT NULL, license text NOT NULL, version int NOT NULL DEFAULT 1,
  UNIQUE (book_code, lang)
);
CREATE TABLE egw_paragraphs (
  edition_id int REFERENCES egw_editions(id),
  refcode text NOT NULL,           -- "SC 9.1"
  chapter int NOT NULL, chapter_title text,
  ord int NOT NULL, page int,
  text text NOT NULL,
  tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', text)) STORED,
  PRIMARY KEY (edition_id, refcode)
);
CREATE INDEX egw_tsv_idx ON egw_paragraphs USING gin (tsv);

-- ── Fundamental Beliefs ─────────────────────────────────
CREATE TABLE beliefs (n int PRIMARY KEY CHECK (n BETWEEN 1 AND 28), group_key text NOT NULL);
CREATE TABLE belief_texts (
  n int REFERENCES beliefs(n), lang text REFERENCES languages(code),
  title text NOT NULL, body text NOT NULL, status content_status NOT NULL DEFAULT 'draft',
  PRIMARY KEY (n, lang)
);

-- ── Hymnals ─────────────────────────────────────────────
CREATE TABLE hymnals (id serial PRIMARY KEY, code text UNIQUE NOT NULL, lang text NOT NULL REFERENCES languages(code), name text NOT NULL, license text NOT NULL);
CREATE TABLE media (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind media_kind NOT NULL, url text NOT NULL, hls_url text,
  bytes bigint, duration_s int, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE hymns (
  id serial PRIMARY KEY,
  hymnal_id int NOT NULL REFERENCES hymnals(id),
  number int NOT NULL, title text NOT NULL, original_title text, category text,
  audio_choir uuid REFERENCES media(id), audio_piano uuid REFERENCES media(id),
  UNIQUE (hymnal_id, number)
);
CREATE TABLE hymn_stanzas (
  hymn_id int REFERENCES hymns(id), idx int NOT NULL,
  kind text NOT NULL CHECK (kind IN ('verse','refrain')), text text NOT NULL,
  PRIMARY KEY (hymn_id, idx)
);

-- ── Sabbath School ──────────────────────────────────────
CREATE TABLE ss_quarters (id serial PRIMARY KEY, year int NOT NULL, quarter int NOT NULL CHECK (quarter BETWEEN 1 AND 4), lang text NOT NULL REFERENCES languages(code), title text NOT NULL, status content_status NOT NULL DEFAULT 'draft', UNIQUE (year, quarter, lang));
CREATE TABLE ss_lessons (id serial PRIMARY KEY, quarter_id int NOT NULL REFERENCES ss_quarters(id), n int NOT NULL, title text NOT NULL, week_start date NOT NULL, memory_ref text, memory_text text, UNIQUE (quarter_id, n));
CREATE TABLE ss_days (id serial PRIMARY KEY, lesson_id int NOT NULL REFERENCES ss_lessons(id), day_idx int NOT NULL CHECK (day_idx BETWEEN 0 AND 6), title text NOT NULL, body text NOT NULL, question text, audio uuid REFERENCES media(id), UNIQUE (lesson_id, day_idx));

-- ── Courses ─────────────────────────────────────────────
CREATE TABLE courses (id serial PRIMARY KEY, slug text UNIQUE NOT NULL, lang text NOT NULL REFERENCES languages(code), title text NOT NULL, audience text[] NOT NULL DEFAULT '{}', status content_status NOT NULL DEFAULT 'draft');
CREATE TABLE course_lessons (id serial PRIMARY KEY, course_id int NOT NULL REFERENCES courses(id), n int NOT NULL, title text NOT NULL, body text NOT NULL, image uuid REFERENCES media(id), UNIQUE (course_id, n));
CREATE TABLE quiz_questions (id serial PRIMARY KEY, lesson_id int NOT NULL REFERENCES course_lessons(id), prompt text NOT NULL, explain_ref text, explain_text text);
CREATE TABLE quiz_options (id serial PRIMARY KEY, question_id int NOT NULL REFERENCES quiz_questions(id), label text NOT NULL, is_correct boolean NOT NULL DEFAULT false, ord int NOT NULL);

-- ── Feed ────────────────────────────────────────────────
CREATE TABLE feed_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind feed_kind NOT NULL,
  lang text NOT NULL REFERENCES languages(code),
  kicker text NOT NULL, source text, body text NOT NULL, ref_label text,
  alt_lang text REFERENCES languages(code), alt_body text,   -- parallel text
  media_id uuid REFERENCES media(id), cta_label text, cta_target text,
  audience text[] NOT NULL DEFAULT '{}',
  status content_status NOT NULL DEFAULT 'draft',
  publish_at timestamptz, created_by uuid, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX feed_pub_idx ON feed_items (lang, status, publish_at DESC);
CREATE TABLE feed_links (
  item_id uuid REFERENCES feed_items(id) ON DELETE CASCADE, ord int NOT NULL,
  target target_kind NOT NULL, target_ref text NOT NULL, label text NOT NULL,
  PRIMARY KEY (item_id, ord)
);

-- ── Users & review ──────────────────────────────────────
CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone text UNIQUE, display_name text, church text,
  role user_role NOT NULL DEFAULT 'member', conference text,
  ui_lang text NOT NULL DEFAULT 'sw' REFERENCES languages(code),
  parallel_lang text REFERENCES languages(code),
  bible_translation text NOT NULL DEFAULT 'SUV',
  text_scale numeric(3,2) NOT NULL DEFAULT 1.0,
  data_saver boolean NOT NULL DEFAULT false,
  sunset_city text, sunset_lat numeric(8,5), sunset_lng numeric(8,5),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  subject_table text NOT NULL, subject_id text NOT NULL,
  reviewer_id uuid NOT NULL REFERENCES users(id),
  decision content_status NOT NULL, note text,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- ── Synced user data (soft delete + updated_at for delta sync) ─
CREATE TABLE user_marks (
  id uuid PRIMARY KEY,                       -- client-generated
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('highlight','note','save','like')),
  target target_kind NOT NULL, target_ref text NOT NULL,
  color text, note text,
  created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz
);
CREATE INDEX user_marks_sync_idx ON user_marks (user_id, updated_at);
CREATE TABLE user_answers (
  id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  target target_kind NOT NULL, target_ref text NOT NULL, answer text, option_id int,
  updated_at timestamptz NOT NULL, deleted_at timestamptz
);
CREATE TABLE user_progress (
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  target target_kind NOT NULL, target_ref text NOT NULL, position text, percent int,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, target, target_ref)
);

-- ── Offline packs ───────────────────────────────────────
CREATE TABLE packs (
  id serial PRIMARY KEY, slug text NOT NULL, lang text NOT NULL REFERENCES languages(code),
  version int NOT NULL, url text NOT NULL, bytes bigint NOT NULL, sha256 text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (slug, version)
);

INSERT INTO languages VALUES ('sw','Kiswahili'),('en','English'),('fr','Français');

-- +goose Down
DROP SCHEMA public CASCADE; CREATE SCHEMA public;
