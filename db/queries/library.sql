-- ── EGW ─────────────────────────────────────────────────

-- name: ListEgwBooks :many
SELECT e.id, e.book_code, e.lang, e.title, b.original_title,
       (SELECT count(DISTINCT p.chapter) FROM egw_paragraphs p WHERE p.edition_id = e.id)::int AS chapters
FROM egw_editions e
JOIN egw_books b ON b.code = e.book_code
WHERE e.lang = @lang
ORDER BY e.title;

-- name: GetEgwEdition :one
SELECT e.id, e.book_code, e.lang, e.title,
       (SELECT count(DISTINCT p.chapter) FROM egw_paragraphs p WHERE p.edition_id = e.id)::int AS chapters
FROM egw_editions e
WHERE e.id = @id;

-- name: GetEgwEditionByBookLang :one
SELECT id FROM egw_editions WHERE book_code = @book_code AND lang = @lang;

-- name: ListEgwChapterParagraphs :many
SELECT refcode, ord, page, text, chapter_title
FROM egw_paragraphs
WHERE edition_id = @edition_id AND chapter = @chapter
ORDER BY ord;

-- ── Fundamental Beliefs ─────────────────────────────────

-- name: ListBeliefs :many
SELECT b.n, b.group_key, t.title
FROM beliefs b
JOIN belief_texts t ON t.n = b.n
WHERE t.lang = @lang AND t.status = 'published'
ORDER BY b.n;

-- name: GetBelief :one
SELECT b.n, b.group_key, t.title, t.body
FROM beliefs b
JOIN belief_texts t ON t.n = b.n
WHERE b.n = @n AND t.lang = @lang AND t.status = 'published';

-- ── Hymnals ─────────────────────────────────────────────

-- name: ListHymnals :many
SELECT code, lang, name FROM hymnals ORDER BY lang, code;

-- name: GetHymnal :one
SELECT id, code, lang, name FROM hymnals WHERE code = @code;

-- name: ListHymns :many
-- q_number filters by exact number; q_title by accent-folded substring or trigram similarity.
SELECT number, title, original_title, category,
       (audio_choir IS NOT NULL OR audio_piano IS NOT NULL)::bool AS has_audio
FROM hymns
WHERE hymnal_id = @hymnal_id
  AND (sqlc.narg(q_number)::int IS NULL OR number = sqlc.narg(q_number)::int)
  AND (sqlc.narg(q_title)::text IS NULL
       OR immutable_unaccent(title) ILIKE '%' || immutable_unaccent(sqlc.narg(q_title)::text) || '%'
       OR similarity(immutable_unaccent(title), immutable_unaccent(sqlc.narg(q_title)::text)) > 0.3)
ORDER BY number
LIMIT 500;

-- name: GetHymn :one
SELECT h.id, h.number, h.title, h.original_title, h.category,
       c.id AS choir_id, c.kind AS choir_kind, c.url AS choir_url, c.hls_url AS choir_hls_url, c.bytes AS choir_bytes, c.duration_s AS choir_duration_s,
       p.id AS piano_id, p.kind AS piano_kind, p.url AS piano_url, p.hls_url AS piano_hls_url, p.bytes AS piano_bytes, p.duration_s AS piano_duration_s
FROM hymns h
LEFT JOIN media c ON c.id = h.audio_choir
LEFT JOIN media p ON p.id = h.audio_piano
WHERE h.hymnal_id = @hymnal_id AND h.number = @number;

-- name: ListHymnStanzas :many
SELECT idx, kind, text FROM hymn_stanzas WHERE hymn_id = @hymn_id ORDER BY idx;

-- ── Sabbath School ──────────────────────────────────────

-- name: GetSabbathSchoolLesson :one
SELECT q.year, q.quarter, q.title AS quarter_title,
       l.id AS lesson_id, l.n, l.title, l.week_start, l.memory_ref, l.memory_text
FROM ss_lessons l
JOIN ss_quarters q ON q.id = l.quarter_id
WHERE q.lang = @lang AND q.status = 'published'
  AND l.week_start <= @day::date AND @day::date < l.week_start + 7
ORDER BY l.week_start DESC
LIMIT 1;

-- name: ListSabbathSchoolDays :many
SELECT d.day_idx, d.title, d.body, d.question,
       m.id AS media_id, m.kind AS media_kind, m.url AS media_url, m.hls_url AS media_hls_url,
       m.bytes AS media_bytes, m.duration_s AS media_duration_s
FROM ss_days d
LEFT JOIN media m ON m.id = d.audio
WHERE d.lesson_id = @lesson_id
ORDER BY d.day_idx;

-- ── Courses ─────────────────────────────────────────────

-- name: ListCourses :many
SELECT c.id, c.slug, c.title, c.audience,
       (SELECT count(*) FROM course_lessons cl WHERE cl.course_id = c.id)::int AS lessons
FROM courses c
WHERE c.lang = @lang AND c.status = 'published'
ORDER BY c.title;

-- name: GetCourseLesson :one
SELECT c.id AS course_id, c.title AS course_title,
       (SELECT count(*) FROM course_lessons x WHERE x.course_id = c.id)::int AS total,
       l.id AS lesson_id, l.n, l.title, l.body,
       m.id AS media_id, m.kind AS media_kind, m.url AS media_url, m.hls_url AS media_hls_url,
       m.bytes AS media_bytes, m.duration_s AS media_duration_s
FROM courses c
JOIN course_lessons l ON l.course_id = c.id
LEFT JOIN media m ON m.id = l.image
WHERE c.id = @course_id AND l.n = @n AND c.status = 'published';

-- name: ListLessonQuestions :many
SELECT id, prompt, explain_ref, explain_text FROM quiz_questions WHERE lesson_id = @lesson_id ORDER BY id;

-- name: ListLessonOptions :many
SELECT o.id, o.question_id, o.label, o.is_correct
FROM quiz_options o
JOIN quiz_questions q ON q.id = o.question_id
WHERE q.lesson_id = @lesson_id
ORDER BY o.question_id, o.ord;
