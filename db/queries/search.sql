-- Grouped search. `q` is the raw query; matching is accent-folded (immutable_unaccent) substring, full-text
-- (`simple` config — Postgres has no Swahili stemmer) and trigram similarity.

-- name: SearchVerses :many
SELECT v.osis_ref, v.book, v.chapter, v.verse, v.text, n.name AS book_name
FROM verses v
JOIN bible_translations t ON t.id = v.translation_id
LEFT JOIN bible_book_names n ON n.book = v.book AND n.lang = t.lang
WHERE t.lang = @lang
  AND (v.tsv @@ plainto_tsquery('simple', @q)
       OR immutable_unaccent(v.text) ILIKE '%' || immutable_unaccent(@q) || '%')
ORDER BY ts_rank(v.tsv, plainto_tsquery('simple', @q)) DESC, t.id, v.book, v.chapter, v.verse
LIMIT @lim;

-- name: FindBookByName :one
SELECT book FROM bible_book_names
WHERE lower(immutable_unaccent(name)) = lower(immutable_unaccent(@name)) OR lower(abbr) = lower(@name)
ORDER BY (lang = @lang) DESC
LIMIT 1;

-- name: GetVerseInLang :one
SELECT v.osis_ref, v.text, n.name AS book_name
FROM verses v
JOIN bible_translations t ON t.id = v.translation_id
LEFT JOIN bible_book_names n ON n.book = v.book AND n.lang = t.lang
WHERE t.lang = @lang AND v.book = @book AND v.chapter = @chapter AND v.verse = @verse
ORDER BY t.id
LIMIT 1;

-- name: SearchEgw :many
SELECT p.refcode, p.text, p.chapter, e.id AS edition_id, e.title
FROM egw_paragraphs p
JOIN egw_editions e ON e.id = p.edition_id
WHERE e.lang = @lang
  AND (p.tsv @@ plainto_tsquery('simple', @q)
       OR immutable_unaccent(p.text) ILIKE '%' || immutable_unaccent(@q) || '%')
ORDER BY ts_rank(p.tsv, plainto_tsquery('simple', @q)) DESC, e.id, p.chapter, p.ord
LIMIT @lim;

-- name: SearchBeliefs :many
SELECT t.n, t.title, t.body
FROM belief_texts t
WHERE t.lang = @lang AND t.status = 'published'
  AND (immutable_unaccent(t.title) ILIKE '%' || immutable_unaccent(@q) || '%'
       OR immutable_unaccent(t.body) ILIKE '%' || immutable_unaccent(@q) || '%'
       OR similarity(immutable_unaccent(t.title), immutable_unaccent(@q)) > 0.3)
ORDER BY similarity(immutable_unaccent(t.title), immutable_unaccent(@q)) DESC, t.n
LIMIT @lim;

-- name: SearchHymns :many
SELECT h.number, h.title, h.category, y.code
FROM hymns h
JOIN hymnals y ON y.id = h.hymnal_id
WHERE y.lang = @lang
  AND (h.number::text = @q::text
       OR immutable_unaccent(h.title) ILIKE '%' || immutable_unaccent(@q::text) || '%'
       OR similarity(immutable_unaccent(h.title), immutable_unaccent(@q::text)) > 0.3)
ORDER BY (h.number::text = @q::text) DESC, similarity(immutable_unaccent(h.title), immutable_unaccent(@q::text)) DESC, h.number
LIMIT @lim;

-- name: SearchVideos :many
SELECT f.id, f.kicker, f.body, f.ref_label
FROM feed_items f
JOIN media m ON m.id = f.media_id AND m.kind = 'video'
WHERE f.lang = @lang AND f.status = 'published' AND f.publish_at <= now()
  AND f.kind IN ('prophecy', 'sermon')
  AND (immutable_unaccent(f.body) ILIKE '%' || immutable_unaccent(@q) || '%'
       OR immutable_unaccent(f.kicker) ILIKE '%' || immutable_unaccent(@q) || '%'
       OR immutable_unaccent(coalesce(f.ref_label, '')) ILIKE '%' || immutable_unaccent(@q) || '%')
ORDER BY f.publish_at DESC
LIMIT @lim;
