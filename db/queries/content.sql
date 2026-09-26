-- name: Ping :one
SELECT 1::int AS ok;

-- name: ListLatestPacks :many
SELECT DISTINCT ON (slug) slug, lang, version, url, bytes, sha256
FROM packs
WHERE lang = @lang
ORDER BY slug, version DESC;

-- name: ListFeed :many
-- Keyset pagination on (publish_at DESC, id DESC). Pass NULL cursor fields for the first page.
SELECT f.id, f.kind, f.lang, f.kicker, f.source, f.body, f.ref_label,
       f.alt_lang, f.alt_body, f.cta_label, f.cta_target, f.audience, f.publish_at,
       m.id AS media_id, m.kind AS media_kind, m.url AS media_url, m.hls_url AS media_hls_url,
       m.bytes AS media_bytes, m.duration_s AS media_duration_s,
       (SELECT count(DISTINCT um.user_id) FROM user_marks um
        WHERE um.kind = 'like' AND um.target = 'feed_item' AND um.target_ref = f.id::text AND um.deleted_at IS NULL)::int AS like_count
FROM feed_items f
LEFT JOIN media m ON m.id = f.media_id
WHERE f.status = 'published'
  AND f.publish_at <= now()
  AND f.lang = @lang
  AND (cardinality(@kinds::text[]) = 0 OR f.kind::text = ANY(@kinds::text[]))
  AND (sqlc.narg(cursor_at)::timestamptz IS NULL
       OR (f.publish_at, f.id) < (sqlc.narg(cursor_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY f.publish_at DESC, f.id DESC
LIMIT @lim;

-- name: GetFeedItem :one
SELECT f.id, f.kind, f.lang, f.kicker, f.source, f.body, f.ref_label,
       f.alt_lang, f.alt_body, f.cta_label, f.cta_target, f.audience, f.publish_at,
       m.id AS media_id, m.kind AS media_kind, m.url AS media_url, m.hls_url AS media_hls_url,
       m.bytes AS media_bytes, m.duration_s AS media_duration_s,
       (SELECT count(DISTINCT um.user_id) FROM user_marks um
        WHERE um.kind = 'like' AND um.target = 'feed_item' AND um.target_ref = f.id::text AND um.deleted_at IS NULL)::int AS like_count
FROM feed_items f
LEFT JOIN media m ON m.id = f.media_id
WHERE f.id = @id AND f.status = 'published' AND f.publish_at <= now();

-- name: ListFeedLinks :many
SELECT target, target_ref, label
FROM feed_links
WHERE item_id = @item_id
ORDER BY ord;

-- name: GetTranslation :one
SELECT id, code, lang FROM bible_translations WHERE code = @code;

-- name: GetBookName :one
SELECT name FROM bible_book_names WHERE book = @book AND lang = @lang;

-- name: ListChapterVerses :many
SELECT osis_ref, verse, text
FROM verses
WHERE translation_id = @translation_id AND book = @book AND chapter = @chapter
ORDER BY verse;

-- name: ListBibleBooks :many
-- Names in the translation's language, falling back to English, then the OSIS code.
SELECT b.osis, b.ord, b.testament::text AS testament,
       COALESCE(n.name, e.name, b.osis)::text AS name, COALESCE(n.abbr, e.abbr, '')::text AS abbr,
       COALESCE((SELECT max(v.chapter) FROM verses v WHERE v.translation_id = @translation_id AND v.book = b.osis), 0)::int AS chapters
FROM bible_books b
LEFT JOIN bible_book_names n ON n.book = b.osis AND n.lang = @lang
LEFT JOIN bible_book_names e ON e.book = b.osis AND e.lang = 'en'
ORDER BY b.ord;
