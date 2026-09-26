-- Offline pack builder (cmd/packs). Reads published sources, records built packs.

-- name: ListTranslationsForPacks :many
SELECT id, code, lang FROM bible_translations ORDER BY code;

-- name: ListVersesForPack :many
SELECT osis_ref, book, chapter, verse, text
FROM verses
WHERE translation_id = @translation_id
ORDER BY book, chapter, verse;

-- name: ListEgwEditionsForPacks :many
SELECT e.id, e.book_code, e.lang, e.title, b.original_title
FROM egw_editions e
JOIN egw_books b ON b.code = e.book_code
ORDER BY e.lang, e.book_code;

-- name: ListEgwParagraphsForPack :many
SELECT refcode, chapter, chapter_title, ord, page, text
FROM egw_paragraphs
WHERE edition_id = @edition_id
ORDER BY chapter, ord;

-- name: ListHymnalsForPacks :many
SELECT id, code, lang, name FROM hymnals ORDER BY code;

-- name: ListHymnsForPack :many
SELECT number, title, original_title, category
FROM hymns
WHERE hymnal_id = @hymnal_id
ORDER BY number;

-- name: ListStanzasForPack :many
SELECT h.number, s.idx, s.kind, s.text
FROM hymn_stanzas s
JOIN hymns h ON h.id = s.hymn_id
WHERE h.hymnal_id = @hymnal_id
ORDER BY h.number, s.idx;

-- name: LatestPack :one
SELECT id, version, url, bytes, sha256, content_hash
FROM packs
WHERE slug = @slug
ORDER BY version DESC
LIMIT 1;

-- name: InsertPack :exec
INSERT INTO packs (slug, lang, version, url, bytes, sha256, content_hash)
VALUES (@slug, @lang, @version, @url, @bytes, @sha256, @content_hash);

-- name: UpdatePackFile :exec
-- Same content rebuilt (e.g. file lost on disk): refresh the file facts for the existing version.
UPDATE packs SET url = @url, bytes = @bytes, sha256 = @sha256 WHERE id = @id;
