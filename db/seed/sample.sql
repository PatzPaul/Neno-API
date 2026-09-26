-- Dev-only sample data. Every text body is a clearly marked [SAMPLE] placeholder:
-- never seed real Scripture or EGW text — licensed content arrives via cmd/ingest.
-- Idempotent: safe to re-run (`make seed`).

BEGIN;

INSERT INTO bible_translations (code, lang, name, license) VALUES
  ('SUV', 'sw', 'Swahili Union Version', '[SAMPLE] unlicensed placeholder'),
  ('KJV', 'en', 'King James Version',    '[SAMPLE] unlicensed placeholder')
ON CONFLICT (code) DO NOTHING;

INSERT INTO bible_book_names (book, lang, name, abbr) VALUES
  ('GEN', 'sw', 'Mwanzo',  'Mwa'), ('GEN', 'en', 'Genesis', 'Gen'),
  ('DAN', 'sw', 'Danieli', 'Dan'), ('DAN', 'en', 'Daniel',  'Dan'),
  ('JHN', 'sw', 'Yohana',  'Yn'),  ('JHN', 'en', 'John',    'John')
ON CONFLICT (book, lang) DO NOTHING;

-- John 3:14–18 in both translations (placeholder text), plus 3:19 only in SUV to exercise parallel alignment.
INSERT INTO verses (translation_id, osis_ref, book, chapter, verse, text)
SELECT t.id, 'JHN.3.' || v, 'JHN', 3, v, format('[SAMPLE] %s Yohana 3:%s', t.code, v)
FROM bible_translations t, generate_series(14, 19) v
WHERE t.code = 'SUV' OR (t.code = 'KJV' AND v <= 18)
ON CONFLICT DO NOTHING;

INSERT INTO media (id, kind, url, hls_url, bytes, duration_s) VALUES
  ('00000000-0000-4000-8000-00000000a001', 'video',
   'https://cdn.example.invalid/sample/daniel-7.mp4', 'https://cdn.example.invalid/sample/daniel-7/index.m3u8', 4800000, 94)
ON CONFLICT (id) DO NOTHING;

INSERT INTO feed_items (id, kind, lang, kicker, source, body, ref_label, alt_lang, alt_body, media_id, cta_label, cta_target, audience, status, publish_at) VALUES
  ('00000000-0000-4000-8000-000000000001', 'verse', 'sw', 'Aya ya Siku', 'Biblia · SUV',
   '[SAMPLE] Aya ya siku', 'YOHANA 3:16', 'en', '[SAMPLE] Verse of the day', NULL, 'Soma sura nzima', 'verse:JHN.3.16', '{}', 'published', now() - interval '1 hour'),
  ('00000000-0000-4000-8000-000000000002', 'egw_quote', 'sw', 'Roho ya Unabii', 'Njia Salama · SC 9.1',
   '[SAMPLE] Nukuu ya EGW', 'SC 9.1', 'en', '[SAMPLE] EGW quote', NULL, NULL, NULL, '{}', 'published', now() - interval '2 hours'),
  ('00000000-0000-4000-8000-000000000003', 'prophecy', 'sw', 'Unabii', 'Danieli 7',
   '[SAMPLE] Wanyama wanne wa Danieli 7', 'DANIELI 7:17', NULL, NULL, '00000000-0000-4000-8000-00000000a001', 'Jifunze zaidi', 'course_lesson:daniel/3', '{youth,seeker}', 'published', now() - interval '3 hours'),
  ('00000000-0000-4000-8000-000000000004', 'health', 'sw', 'Afya', 'Kanuni 8 za Afya',
   '[SAMPLE] Kidokezo cha afya', NULL, NULL, NULL, NULL, NULL, NULL, '{}', 'published', now() - interval '4 hours'),
  ('00000000-0000-4000-8000-000000000005', 'hymn', 'sw', 'Nyimbo', 'Nyimbo za Kristo · 1',
   '[SAMPLE] Ubeti wa wimbo', NULL, NULL, NULL, NULL, 'Sikiliza', 'hymn:NZK/1', '{}', 'published', now() - interval '5 hours'),
  ('00000000-0000-4000-8000-000000000006', 'verse', 'en', 'Verse of the Day', 'Bible · KJV',
   '[SAMPLE] Verse of the day', 'JOHN 3:16', NULL, NULL, NULL, NULL, NULL, '{}', 'published', now() - interval '1 hour'),
  -- Must never be served: unpublished, and scheduled for the future.
  ('00000000-0000-4000-8000-000000000007', 'devotional', 'sw', 'Rasimu', NULL,
   '[SAMPLE] draft — should not appear', NULL, NULL, NULL, NULL, NULL, NULL, '{}', 'in_review', now() - interval '1 hour'),
  ('00000000-0000-4000-8000-000000000008', 'devotional', 'sw', 'Kesho', NULL,
   '[SAMPLE] future — should not appear', NULL, NULL, NULL, NULL, NULL, NULL, '{}', 'published', now() + interval '1 day')
ON CONFLICT (id) DO NOTHING;

INSERT INTO feed_links (item_id, ord, target, target_ref, label) VALUES
  ('00000000-0000-4000-8000-000000000001', 1, 'verse',         'JHN.3.17', '[SAMPLE] Yohana 3:17'),
  ('00000000-0000-4000-8000-000000000001', 2, 'egw_paragraph', 'SC 9.1',   '[SAMPLE] Njia Salama 9.1'),
  ('00000000-0000-4000-8000-000000000001', 3, 'belief',        '10',       '[SAMPLE] Imani 10')
ON CONFLICT DO NOTHING;

INSERT INTO packs (slug, lang, version, url, bytes, sha256) VALUES
  ('bible-SUV', 'sw', 1, 'https://cdn.example.invalid/packs/bible-SUV-v1.sqlite', 4200000, repeat('0', 64)),
  ('bible-SUV', 'sw', 2, 'https://cdn.example.invalid/packs/bible-SUV-v2.sqlite', 4300000, repeat('0', 64)),
  ('hymnal-NZK', 'sw', 1, 'https://cdn.example.invalid/packs/hymnal-NZK-v1.sqlite', 900000, repeat('0', 64))
ON CONFLICT (slug, version) DO NOTHING;

-- Reader sample: John 1–3 and Daniel 7 in both translations (placeholder text). KJV deliberately lacks
-- John 3:19 so parallel alignment with a missing verse stays exercised.
INSERT INTO verses (translation_id, osis_ref, book, chapter, verse, text)
SELECT t.id, format('%s.%s.%s', c.book, c.chapter, v), c.book, c.chapter, v,
       format('[SAMPLE] %s %s %s:%s', t.code, CASE c.book WHEN 'JHN' THEN 'Yohana' ELSE 'Danieli' END, c.chapter, v)
FROM bible_translations t
CROSS JOIN (VALUES ('JHN', 1, 51), ('JHN', 2, 25), ('JHN', 3, 36), ('DAN', 7, 28)) AS c(book, chapter, verses)
CROSS JOIN LATERAL generate_series(1, c.verses) v
WHERE t.code IN ('SUV', 'KJV') AND NOT (t.code = 'KJV' AND c.book = 'JHN' AND c.chapter = 3 AND v = 19)
ON CONFLICT DO NOTHING;

-- Daniel 7:17 placeholder (course lesson 3 explains with it).
INSERT INTO verses (translation_id, osis_ref, book, chapter, verse, text)
SELECT t.id, 'DAN.7.17', 'DAN', 7, 17, format('[SAMPLE] %s Danieli 7:17', t.code)
FROM bible_translations t WHERE t.code IN ('SUV', 'KJV')
ON CONFLICT DO NOTHING;

-- Today's Sabbath School item, anchored second on the first feed page.
INSERT INTO feed_items (id, kind, lang, kicker, source, body, ref_label, status, publish_at) VALUES
  ('00000000-0000-4000-8000-000000000009', 'sabbath_school', 'sw', 'Shule ya Sabato', 'Somo la wiki',
   '[SAMPLE] Swali la leo la Shule ya Sabato', NULL, 'published', now() - interval '30 minutes')
ON CONFLICT (id) DO NOTHING;

-- ── EGW: Steps to Christ, sw + en, two chapters aligned by refcode ─
INSERT INTO egw_books (code, original_title) VALUES ('SC', 'Steps to Christ') ON CONFLICT DO NOTHING;
INSERT INTO egw_editions (book_code, lang, title, license) VALUES
  ('SC', 'sw', 'Njia Salama', '[SAMPLE] unlicensed placeholder'),
  ('SC', 'en', 'Steps to Christ', '[SAMPLE] unlicensed placeholder')
ON CONFLICT (book_code, lang) DO NOTHING;

INSERT INTO egw_paragraphs (edition_id, refcode, chapter, chapter_title, ord, page, text)
SELECT e.id, format('SC %s.%s', pg.page, pg.para), pg.chapter,
       CASE e.lang WHEN 'sw' THEN format('[SAMPLE] Sura ya %s', pg.chapter) ELSE format('[SAMPLE] Chapter %s', pg.chapter) END,
       pg.ord, pg.page,
       CASE e.lang WHEN 'sw' THEN format('[SAMPLE] Njia Salama SC %s.%s — maandishi ya EGW yatatoka kwa White Estate.', pg.page, pg.para)
                   ELSE format('[SAMPLE] Steps to Christ SC %s.%s — text comes from the White Estate.', pg.page, pg.para) END
FROM egw_editions e
CROSS JOIN (VALUES (1, 9, 1, 1), (1, 9, 2, 2), (1, 10, 1, 3), (2, 17, 1, 1), (2, 17, 2, 2)) AS pg(chapter, page, para, ord)
WHERE e.book_code = 'SC'
ON CONFLICT DO NOTHING;

-- ── Fundamental Beliefs: all 28 numbers; a few published texts ─
INSERT INTO beliefs (n, group_key)
SELECT n, CASE WHEN n <= 5 THEN 'god' WHEN n <= 7 THEN 'humanity' WHEN n <= 11 THEN 'salvation'
               WHEN n <= 18 THEN 'church' WHEN n <= 23 THEN 'christian_life' ELSE 'last_things' END
FROM generate_series(1, 28) n
ON CONFLICT DO NOTHING;

INSERT INTO belief_texts (n, lang, title, body, status)
SELECT b.n, l.lang,
       CASE l.lang WHEN 'sw' THEN format('[SAMPLE] Imani ya %s', b.n) ELSE format('[SAMPLE] Belief %s', b.n) END,
       CASE l.lang WHEN 'sw' THEN format('[SAMPLE] Maelezo ya imani ya %s. Maandishi rasmi yatatoka kwa Konferensi Kuu.', b.n)
                   ELSE format('[SAMPLE] Text of belief %s. Official wording comes from the General Conference.', b.n) END,
       CASE WHEN b.n IN (1, 10, 19, 20, 28) THEN 'published' ELSE 'draft' END::content_status
FROM beliefs b CROSS JOIN (VALUES ('sw'), ('en')) AS l(lang)
ON CONFLICT DO NOTHING;

-- ── Hymnals ─────────────────────────────────────────────
INSERT INTO hymnals (code, lang, name, license) VALUES
  ('NZK', 'sw', 'Nyimbo za Kristo', '[SAMPLE] unlicensed placeholder'),
  ('SDAH', 'en', 'SDA Hymnal', '[SAMPLE] unlicensed placeholder')
ON CONFLICT (code) DO NOTHING;

INSERT INTO media (id, kind, url, bytes, duration_s) VALUES
  ('00000000-0000-4000-8000-00000000a002', 'audio', 'https://cdn.example.invalid/sample/nzk-1-choir.m4a', 1400000, 182),
  ('00000000-0000-4000-8000-00000000a003', 'audio', 'https://cdn.example.invalid/sample/nzk-1-piano.m4a', 1300000, 176)
ON CONFLICT (id) DO NOTHING;

INSERT INTO hymns (hymnal_id, number, title, original_title, category, audio_choir, audio_piano)
SELECT y.id, h.number, format('[SAMPLE] %s %s', CASE y.lang WHEN 'sw' THEN 'Wimbo' ELSE 'Hymn' END, h.number),
       CASE y.lang WHEN 'sw' THEN format('[SAMPLE] Hymn %s', h.number) END,
       CASE y.lang WHEN 'sw' THEN h.cat_sw ELSE h.cat_en END,
       CASE WHEN y.code = 'NZK' AND h.number = 1 THEN '00000000-0000-4000-8000-00000000a002'::uuid END,
       CASE WHEN y.code = 'NZK' AND h.number = 1 THEN '00000000-0000-4000-8000-00000000a003'::uuid END
FROM hymnals y
CROSS JOIN (VALUES (1, 'Sifa', 'Praise'), (12, 'Maombi', 'Prayer'), (27, 'Imani', 'Faith'),
                   (44, 'Sabato', 'Sabbath'), (58, 'Kuja kwa Yesu', 'Second Coming'), (73, 'Sifa', 'Praise')) AS h(number, cat_sw, cat_en)
WHERE y.code IN ('NZK', 'SDAH')
ON CONFLICT (hymnal_id, number) DO NOTHING;

INSERT INTO hymn_stanzas (hymn_id, idx, kind, text)
SELECT h.id, st.idx, st.kind,
       format('[SAMPLE] %s %s — %s', CASE st.kind WHEN 'refrain' THEN 'Kiitikio' ELSE 'Ubeti' END, st.idx, h.title)
FROM hymns h
CROSS JOIN (VALUES (1, 'verse'), (2, 'refrain'), (3, 'verse'), (4, 'verse')) AS st(idx, kind)
ON CONFLICT DO NOTHING;

-- ── Sabbath School: current quarter, lessons for last/this/next week (weeks start on Sabbath) ─
INSERT INTO ss_quarters (year, quarter, lang, title, status)
SELECT extract(year FROM current_date)::int, extract(quarter FROM current_date)::int, l.lang,
       CASE l.lang WHEN 'sw' THEN '[SAMPLE] Robo ya mwaka' ELSE '[SAMPLE] Quarterly' END, 'published'
FROM (VALUES ('sw'), ('en')) AS l(lang)
ON CONFLICT (year, quarter, lang) DO NOTHING;

WITH weeks AS (
  SELECT (current_date - ((extract(dow FROM current_date)::int + 1) % 7)) + 7 * w AS week_start
  FROM generate_series(-1, 1) w
)
INSERT INTO ss_lessons (quarter_id, n, title, week_start, memory_ref, memory_text)
SELECT q.id,
       1 + ((wk.week_start - date_trunc('quarter', wk.week_start)::date) / 7),
       CASE q.lang WHEN 'sw' THEN format('[SAMPLE] Somo la wiki ya %s', to_char(wk.week_start, 'DD Mon'))
                   ELSE format('[SAMPLE] Lesson for the week of %s', to_char(wk.week_start, 'DD Mon')) END,
       wk.week_start, 'JHN.3.16',
       CASE q.lang WHEN 'sw' THEN '[SAMPLE] Fungu la kukariri' ELSE '[SAMPLE] Memory text' END
FROM weeks wk
JOIN ss_quarters q ON q.year = extract(year FROM wk.week_start)::int AND q.quarter = extract(quarter FROM wk.week_start)::int
ON CONFLICT (quarter_id, n) DO NOTHING;

INSERT INTO ss_days (lesson_id, day_idx, title, body, question)
SELECT l.id, d,
       CASE q.lang WHEN 'sw' THEN format('[SAMPLE] Siku ya %s', d + 1) ELSE format('[SAMPLE] Day %s', d + 1) END,
       CASE q.lang WHEN 'sw' THEN '[SAMPLE] Maandishi ya somo la siku. Yatatoka kwenye robo rasmi ya Divisheni.'
                   ELSE '[SAMPLE] Daily lesson text. Comes from the official Division quarterly.' END,
       CASE WHEN d IN (1, 3, 5) THEN CASE q.lang WHEN 'sw' THEN '[SAMPLE] Swali la kutafakari?' ELSE '[SAMPLE] Question to consider?' END END
FROM ss_lessons l
JOIN ss_quarters q ON q.id = l.quarter_id
CROSS JOIN generate_series(0, 6) d
ON CONFLICT (lesson_id, day_idx) DO NOTHING;

-- ── Course: Unabii wa Danieli, 12 lessons; lesson 3 has a quiz ─
INSERT INTO courses (slug, lang, title, audience, status) VALUES
  ('unabii-wa-danieli', 'sw', '[SAMPLE] Unabii wa Danieli', '{youth,seeker}', 'published')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO course_lessons (course_id, n, title, body)
SELECT c.id, n, format('[SAMPLE] Somo la %s', n), format('[SAMPLE] Maandishi ya somo la %s.', n)
FROM courses c CROSS JOIN generate_series(1, 12) n
WHERE c.slug = 'unabii-wa-danieli'
ON CONFLICT (course_id, n) DO NOTHING;

INSERT INTO quiz_questions (lesson_id, prompt, explain_ref, explain_text)
SELECT l.id, '[SAMPLE] Wanyama wanne wa Danieli 7 wanawakilisha nini?', 'DAN.7.17', '[SAMPLE] Maelezo ya jibu.'
FROM course_lessons l JOIN courses c ON c.id = l.course_id
WHERE c.slug = 'unabii-wa-danieli' AND l.n = 3
  AND NOT EXISTS (SELECT 1 FROM quiz_questions q WHERE q.lesson_id = l.id);

INSERT INTO quiz_options (question_id, label, is_correct, ord)
SELECT q.id, o.label, o.correct, o.ord
FROM quiz_questions q
JOIN course_lessons l ON l.id = q.lesson_id
JOIN courses c ON c.id = l.course_id
CROSS JOIN (VALUES ('[SAMPLE] Falme nne', true, 1), ('[SAMPLE] Misimu minne', false, 2), ('[SAMPLE] Mito minne', false, 3)) AS o(label, correct, ord)
WHERE c.slug = 'unabii-wa-danieli' AND l.n = 3
  AND NOT EXISTS (SELECT 1 FROM quiz_options x WHERE x.question_id = q.id);

COMMIT;
