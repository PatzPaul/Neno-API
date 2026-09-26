-- name: UpsertUserFromToken :exec
-- Called on authenticated requests (cached in-process). Email and role follow Keycloak; a display name
-- the user set in the app is never overwritten.
INSERT INTO users (id, email, display_name, role)
VALUES (@id, sqlc.narg(email), sqlc.narg(display_name), @role)
ON CONFLICT (id) DO UPDATE
SET email        = COALESCE(EXCLUDED.email, users.email),
    display_name = COALESCE(users.display_name, EXCLUDED.display_name),
    role         = EXCLUDED.role,
    updated_at   = CASE WHEN users.role IS DISTINCT FROM EXCLUDED.role OR users.email IS DISTINCT FROM EXCLUDED.email
                        THEN now() ELSE users.updated_at END;

-- name: GetUser :one
SELECT id, email, phone, display_name, church, role, ui_lang, parallel_lang, bible_translation,
       text_scale, data_saver, sunset_city, sunset_lat, sunset_lng
FROM users WHERE id = @id;

-- name: UpdateUserSettings :exec
UPDATE users SET
  display_name = sqlc.narg(display_name), church = sqlc.narg(church), ui_lang = @ui_lang,
  parallel_lang = sqlc.narg(parallel_lang), bible_translation = @bible_translation,
  text_scale = @text_scale::float8, data_saver = @data_saver, sunset_city = sqlc.narg(sunset_city),
  sunset_lat = sqlc.narg(sunset_lat)::float8, sunset_lng = sqlc.narg(sunset_lng)::float8,
  updated_at = now()
WHERE id = @id;

-- name: LanguageExists :one
SELECT EXISTS (SELECT 1 FROM languages WHERE code = @code);

-- name: TranslationExists :one
SELECT EXISTS (SELECT 1 FROM bible_translations WHERE code = @code);
