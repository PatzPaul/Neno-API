-- ── Likes ───────────────────────────────────────────────

-- name: HasActiveLike :one
SELECT EXISTS (
  SELECT 1 FROM user_marks
  WHERE user_id = @user_id AND kind = 'like' AND target = 'feed_item' AND target_ref = @target_ref AND deleted_at IS NULL
);

-- name: InsertLike :exec
INSERT INTO user_marks (id, user_id, kind, target, target_ref, created_at, updated_at)
VALUES (gen_random_uuid(), @user_id, 'like', 'feed_item', @target_ref, now(), now());

-- name: DeleteLikes :exec
UPDATE user_marks SET deleted_at = now(), updated_at = now(), server_updated_at = now()
WHERE user_id = @user_id AND kind = 'like' AND target = 'feed_item' AND target_ref = @target_ref AND deleted_at IS NULL;

-- name: CountLikes :one
SELECT count(DISTINCT user_id)::int FROM user_marks
WHERE kind = 'like' AND target = 'feed_item' AND target_ref = @target_ref AND deleted_at IS NULL;

-- name: FeedItemPublished :one
SELECT EXISTS (SELECT 1 FROM feed_items WHERE id = @id AND status = 'published' AND publish_at <= now());

-- ── Sync push (last-write-wins on the client's updated_at) ─

-- name: UpsertMark :exec
-- A row owned by another user is never touched (WHERE on the conflict update).
INSERT INTO user_marks (id, user_id, kind, target, target_ref, color, note, created_at, updated_at, deleted_at, server_updated_at)
VALUES (@id, @user_id, @kind, @target, @target_ref, sqlc.narg(color), sqlc.narg(note), @created_at, @updated_at, sqlc.narg(deleted_at), now())
ON CONFLICT (id) DO UPDATE
SET kind = EXCLUDED.kind, target = EXCLUDED.target, target_ref = EXCLUDED.target_ref, color = EXCLUDED.color,
    note = EXCLUDED.note, updated_at = EXCLUDED.updated_at, deleted_at = EXCLUDED.deleted_at, server_updated_at = now()
WHERE user_marks.user_id = EXCLUDED.user_id AND EXCLUDED.updated_at > user_marks.updated_at;

-- name: UpsertAnswer :exec
INSERT INTO user_answers (id, user_id, target, target_ref, answer, option_id, updated_at, deleted_at, server_updated_at)
VALUES (@id, @user_id, @target, @target_ref, sqlc.narg(answer), sqlc.narg(option_id), @updated_at, sqlc.narg(deleted_at), now())
ON CONFLICT (id) DO UPDATE
SET target = EXCLUDED.target, target_ref = EXCLUDED.target_ref, answer = EXCLUDED.answer, option_id = EXCLUDED.option_id,
    updated_at = EXCLUDED.updated_at, deleted_at = EXCLUDED.deleted_at, server_updated_at = now()
WHERE user_answers.user_id = EXCLUDED.user_id AND EXCLUDED.updated_at > user_answers.updated_at;

-- name: UpsertProgress :exec
INSERT INTO user_progress (user_id, target, target_ref, position, percent, updated_at, server_updated_at)
VALUES (@user_id, @target, @target_ref, sqlc.narg(position), sqlc.narg(percent), @updated_at, now())
ON CONFLICT (user_id, target, target_ref) DO UPDATE
SET position = EXCLUDED.position, percent = EXCLUDED.percent, updated_at = EXCLUDED.updated_at, server_updated_at = now()
WHERE EXCLUDED.updated_at > user_progress.updated_at;

-- ── Sync pull ───────────────────────────────────────────

-- name: SyncNow :one
SELECT now()::timestamptz AS now;

-- name: PullMarks :many
SELECT id, kind, target, target_ref, color, note, created_at, updated_at, deleted_at
FROM user_marks
WHERE user_id = @user_id AND (sqlc.narg(since)::timestamptz IS NULL OR server_updated_at > sqlc.narg(since)::timestamptz)
ORDER BY server_updated_at;

-- name: PullAnswers :many
SELECT id, target, target_ref, answer, option_id, updated_at, deleted_at
FROM user_answers
WHERE user_id = @user_id AND (sqlc.narg(since)::timestamptz IS NULL OR server_updated_at > sqlc.narg(since)::timestamptz)
ORDER BY server_updated_at;

-- name: PullProgress :many
SELECT target, target_ref, position, percent, updated_at
FROM user_progress
WHERE user_id = @user_id AND (sqlc.narg(since)::timestamptz IS NULL OR server_updated_at > sqlc.narg(since)::timestamptz)
ORDER BY server_updated_at;

-- ── Quiz answers ────────────────────────────────────────

-- name: GetQuizOptionForLesson :one
-- Confirms option ∈ question ∈ lesson ∈ published course, and returns the correct option of that question.
SELECT o.is_correct,
       (SELECT c.id FROM quiz_options c WHERE c.question_id = q.id AND c.is_correct ORDER BY c.ord LIMIT 1)::int AS correct_option_id
FROM quiz_options o
JOIN quiz_questions q ON q.id = o.question_id
JOIN course_lessons l ON l.id = q.lesson_id
JOIN courses c ON c.id = l.course_id
WHERE o.id = @option_id AND q.id = @question_id AND l.course_id = @course_id AND l.n = @n AND c.status = 'published';

-- name: UpdateQuizAnswer :execrows
UPDATE user_answers SET option_id = @option_id, answer = NULL, deleted_at = NULL, updated_at = now(), server_updated_at = now()
WHERE user_id = @user_id AND target = 'course_lesson' AND target_ref = @target_ref;

-- name: InsertQuizAnswer :exec
INSERT INTO user_answers (id, user_id, target, target_ref, option_id, updated_at)
VALUES (gen_random_uuid(), @user_id, 'course_lesson', @target_ref, @option_id, now());
