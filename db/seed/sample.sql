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
  ('JHN', 'sw', 'Yohana',  'Yn'),  ('JHN', 'en', 'John',    'Jn')
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

COMMIT;
