package server

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/store"
)

// ── Likes ───────────────────────────────────────────────

func (s *Server) LikeFeedItem(ctx context.Context, req api.LikeFeedItemRequestObject) (api.LikeFeedItemResponseObject, error) {
	st, found, err := s.setLike(ctx, req.Id, true)
	if err != nil {
		return nil, err
	}
	if !found {
		return api.LikeFeedItem404JSONResponse{NotFoundJSONResponse: notFound("feed item not found")}, nil
	}
	return api.LikeFeedItem200JSONResponse(st), nil
}

func (s *Server) UnlikeFeedItem(ctx context.Context, req api.UnlikeFeedItemRequestObject) (api.UnlikeFeedItemResponseObject, error) {
	st, found, err := s.setLike(ctx, req.Id, false)
	if err != nil {
		return nil, err
	}
	if !found {
		return api.UnlikeFeedItem404JSONResponse{NotFoundJSONResponse: notFound("feed item not found")}, nil
	}
	return api.UnlikeFeedItem200JSONResponse(st), nil
}

// setLike is idempotent: liking twice keeps one active like, unliking soft-deletes (so sync sees it).
func (s *Server) setLike(ctx context.Context, item uuid.UUID, like bool) (api.LikeState, bool, error) {
	p, err := principal(ctx)
	if err != nil {
		return api.LikeState{}, false, err
	}
	ok, err := s.q.FeedItemPublished(ctx, item)
	if err != nil || !ok {
		return api.LikeState{}, false, err
	}
	ref := item.String()
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Serialise concurrent like/unlike for the same user + item.
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", p.UserID.String()+ref); err != nil {
			return err
		}
		if !like {
			return q.DeleteLikes(ctx, store.DeleteLikesParams{UserID: p.UserID, TargetRef: ref})
		}
		has, err := q.HasActiveLike(ctx, store.HasActiveLikeParams{UserID: p.UserID, TargetRef: ref})
		if err != nil || has {
			return err
		}
		return q.InsertLike(ctx, store.InsertLikeParams{UserID: p.UserID, TargetRef: ref})
	})
	if err != nil {
		return api.LikeState{}, true, err
	}
	n, err := s.q.CountLikes(ctx, ref)
	return api.LikeState{Liked: like, LikeCount: int(n)}, true, err
}

// ── Me ──────────────────────────────────────────────────

func (s *Server) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.loadMe(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return api.GetMe200JSONResponse(me), nil
}

func (s *Server) loadMe(ctx context.Context, id uuid.UUID) (api.Me, error) {
	u, err := s.q.GetUser(ctx, id)
	if err != nil {
		return api.Me{}, err
	}
	me := api.Me{
		Id: u.ID, Email: textPtr(u.Email), Phone: textPtr(u.Phone), DisplayName: textPtr(u.DisplayName), Church: textPtr(u.Church),
		Role: api.MeRole(u.Role), UiLang: u.UiLang, ParallelLang: textPtr(u.ParallelLang), BibleTranslation: u.BibleTranslation,
		TextScale: float32(numFloat(u.TextScale)), DataSaver: u.DataSaver, SunsetCity: textPtr(u.SunsetCity),
	}
	if u.SunsetLat.Valid {
		v := float32(numFloat(u.SunsetLat))
		me.SunsetLat = &v
	}
	if u.SunsetLng.Valid {
		v := float32(numFloat(u.SunsetLng))
		me.SunsetLng = &v
	}
	return me, nil
}

func numFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func (s *Server) UpdateMe(ctx context.Context, req api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	bad := func(msg string) (api.UpdateMeResponseObject, error) {
		return api.UpdateMe400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}
	if req.Body == nil {
		return bad("body required")
	}
	b := req.Body
	cur, err := s.q.GetUser(ctx, p.UserID)
	if err != nil {
		return nil, err
	}

	upd := store.UpdateUserSettingsParams{
		ID: p.UserID, DisplayName: cur.DisplayName, Church: cur.Church, UiLang: cur.UiLang, ParallelLang: cur.ParallelLang,
		BibleTranslation: cur.BibleTranslation, TextScale: numFloat(cur.TextScale), DataSaver: cur.DataSaver, SunsetCity: cur.SunsetCity,
	}
	if cur.SunsetLat.Valid {
		upd.SunsetLat = pgtype.Float8{Float64: numFloat(cur.SunsetLat), Valid: true}
	}
	if cur.SunsetLng.Valid {
		upd.SunsetLng = pgtype.Float8{Float64: numFloat(cur.SunsetLng), Valid: true}
	}

	optText := func(v string, max int, field string) (pgtype.Text, error) {
		v = strings.TrimSpace(v)
		if len([]rune(v)) > max {
			return pgtype.Text{}, fmt.Errorf("%s is too long (max %d)", field, max)
		}
		return pgtype.Text{String: v, Valid: v != ""}, nil
	}
	if b.DisplayName != nil {
		if upd.DisplayName, err = optText(*b.DisplayName, 80, "display_name"); err != nil {
			return bad(err.Error())
		}
	}
	if b.Church != nil {
		if upd.Church, err = optText(*b.Church, 120, "church"); err != nil {
			return bad(err.Error())
		}
	}
	if b.SunsetCity != nil {
		if upd.SunsetCity, err = optText(*b.SunsetCity, 120, "sunset_city"); err != nil {
			return bad(err.Error())
		}
	}
	if b.UiLang != nil {
		ok, err := s.q.LanguageExists(ctx, *b.UiLang)
		if err != nil {
			return nil, err
		}
		if !ok {
			return bad("unknown ui_lang")
		}
		upd.UiLang = *b.UiLang
	}
	if b.ParallelLang != nil {
		if *b.ParallelLang == "" {
			upd.ParallelLang = pgtype.Text{}
		} else {
			ok, err := s.q.LanguageExists(ctx, *b.ParallelLang)
			if err != nil {
				return nil, err
			}
			if !ok {
				return bad("unknown parallel_lang")
			}
			upd.ParallelLang = pgtype.Text{String: *b.ParallelLang, Valid: true}
		}
	}
	if b.BibleTranslation != nil {
		code := strings.ToUpper(*b.BibleTranslation)
		ok, err := s.q.TranslationExists(ctx, code)
		if err != nil {
			return nil, err
		}
		if !ok {
			return bad("unknown bible_translation")
		}
		upd.BibleTranslation = code
	}
	if b.TextScale != nil {
		if *b.TextScale < 1 || *b.TextScale > 2 {
			return bad("text_scale must be between 1 and 2")
		}
		upd.TextScale = float64(*b.TextScale)
	}
	if b.DataSaver != nil {
		upd.DataSaver = *b.DataSaver
	}
	if b.SunsetLat != nil {
		if *b.SunsetLat < -90 || *b.SunsetLat > 90 {
			return bad("sunset_lat out of range")
		}
		upd.SunsetLat = pgtype.Float8{Float64: float64(*b.SunsetLat), Valid: true}
	}
	if b.SunsetLng != nil {
		if *b.SunsetLng < -180 || *b.SunsetLng > 180 {
			return bad("sunset_lng out of range")
		}
		upd.SunsetLng = pgtype.Float8{Float64: float64(*b.SunsetLng), Valid: true}
	}

	if err := s.q.UpdateUserSettings(ctx, upd); err != nil {
		return nil, err
	}
	me, err := s.loadMe(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	return api.UpdateMe200JSONResponse(me), nil
}

// ── Sync ────────────────────────────────────────────────

const (
	maxSyncItems = 500
	// syncOverlap re-sends rows written by transactions that started before the previous cursor but committed
	// after it. Duplicates are harmless: clients merge by id with last-write-wins.
	syncOverlap = 30 * time.Second
	// maxClockSkew clamps client timestamps so a device with a future clock can't win every conflict forever.
	maxClockSkew = 5 * time.Minute
)

var (
	markKinds   = map[api.UserMarkKind]bool{api.Highlight: true, api.Note: true, api.Save: true, api.Like: true}
	targetKinds = map[api.TargetKind]bool{
		api.TargetKindVerse: true, api.TargetKindEgwParagraph: true, api.TargetKindBelief: true, api.TargetKindHymn: true,
		api.TargetKindFeedItem: true, api.TargetKindSsDay: true, api.TargetKindCourseLesson: true,
	}
)

func encodeSyncCursor(t time.Time) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano)))
}

func decodeSyncCursor(c string) (time.Time, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, string(b))
}

func tsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func (s *Server) Sync(ctx context.Context, req api.SyncRequestObject) (api.SyncResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	bad := func(msg string) (api.SyncResponseObject, error) {
		return api.Sync400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}
	if req.Body == nil {
		return bad("body required")
	}
	b := req.Body
	var marks []api.UserMark
	var answers []api.UserAnswer
	var progress []api.UserProgress
	if b.Marks != nil {
		marks = *b.Marks
	}
	if b.Answers != nil {
		answers = *b.Answers
	}
	if b.Progress != nil {
		progress = *b.Progress
	}
	if len(marks)+len(answers)+len(progress) > maxSyncItems {
		return bad(fmt.Sprintf("too many items (max %d per request)", maxSyncItems))
	}
	since := pgtype.Timestamptz{}
	if b.Cursor != nil && *b.Cursor != "" {
		t, err := decodeSyncCursor(*b.Cursor)
		if err != nil {
			return bad("invalid cursor")
		}
		since = pgtype.Timestamptz{Time: t.Add(-syncOverlap), Valid: true}
	}

	// Validate everything before writing anything.
	for _, m := range marks {
		if m.Id == uuid.Nil || !markKinds[m.Kind] || !targetKinds[m.Target] || m.TargetRef == "" || len(m.TargetRef) > 200 {
			return bad("invalid mark " + m.Id.String())
		}
		if m.Note != nil && len(*m.Note) > 10_000 {
			return bad("note too long on mark " + m.Id.String())
		}
	}
	for _, a := range answers {
		if a.Id == uuid.Nil || !targetKinds[a.Target] || a.TargetRef == "" || len(a.TargetRef) > 200 {
			return bad("invalid answer " + a.Id.String())
		}
		if a.Answer != nil && len(*a.Answer) > 10_000 {
			return bad("answer too long on " + a.Id.String())
		}
	}
	for _, pr := range progress {
		if !targetKinds[pr.Target] || pr.TargetRef == "" || len(pr.TargetRef) > 200 {
			return bad("invalid progress row")
		}
		if pr.Percent != nil && (*pr.Percent < 0 || *pr.Percent > 100) {
			return bad("percent must be 0–100")
		}
	}

	var out api.Sync200JSONResponse
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now, err := q.SyncNow(ctx) // transaction start time: the next cursor
		if err != nil {
			return err
		}
		clamp := func(t time.Time) time.Time {
			if t.After(now.Add(maxClockSkew)) {
				return now
			}
			return t
		}
		for _, m := range marks {
			created := m.CreatedAt
			if created.IsZero() {
				created = m.UpdatedAt
			}
			err := q.UpsertMark(ctx, store.UpsertMarkParams{
				ID: m.Id, UserID: p.UserID, Kind: string(m.Kind), Target: store.TargetKind(m.Target), TargetRef: m.TargetRef,
				Color: textFromPtr(m.Color), Note: textFromPtr(m.Note), CreatedAt: clamp(created), UpdatedAt: clamp(m.UpdatedAt),
				DeletedAt: tsPtr(m.DeletedAt),
			})
			if err != nil {
				return err
			}
		}
		for _, a := range answers {
			err := q.UpsertAnswer(ctx, store.UpsertAnswerParams{
				ID: a.Id, UserID: p.UserID, Target: store.TargetKind(a.Target), TargetRef: a.TargetRef,
				Answer: textFromPtr(a.Answer), OptionID: int4FromPtr(a.OptionId), UpdatedAt: clamp(a.UpdatedAt), DeletedAt: tsPtr(a.DeletedAt),
			})
			if err != nil {
				return err
			}
		}
		for _, pr := range progress {
			err := q.UpsertProgress(ctx, store.UpsertProgressParams{
				UserID: p.UserID, Target: store.TargetKind(pr.Target), TargetRef: pr.TargetRef,
				Position: textFromPtr(pr.Position), Percent: int4FromPtr(pr.Percent), UpdatedAt: clamp(pr.UpdatedAt),
			})
			if err != nil {
				return err
			}
		}

		pm, err := q.PullMarks(ctx, store.PullMarksParams{UserID: p.UserID, Since: since})
		if err != nil {
			return err
		}
		pa, err := q.PullAnswers(ctx, store.PullAnswersParams{UserID: p.UserID, Since: since})
		if err != nil {
			return err
		}
		pp, err := q.PullProgress(ctx, store.PullProgressParams{UserID: p.UserID, Since: since})
		if err != nil {
			return err
		}
		out = api.Sync200JSONResponse{
			Cursor: encodeSyncCursor(now), Marks: make([]api.UserMark, len(pm)),
			Answers: make([]api.UserAnswer, len(pa)), Progress: make([]api.UserProgress, len(pp)),
		}
		for i, m := range pm {
			out.Marks[i] = api.UserMark{
				Id: m.ID, Kind: api.UserMarkKind(m.Kind), Target: api.TargetKind(m.Target), TargetRef: m.TargetRef,
				Color: textPtr(m.Color), Note: textPtr(m.Note), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DeletedAt: timePtr(m.DeletedAt),
			}
		}
		for i, a := range pa {
			out.Answers[i] = api.UserAnswer{
				Id: a.ID, Target: api.TargetKind(a.Target), TargetRef: a.TargetRef, Answer: textPtr(a.Answer),
				OptionId: int4Ptr(a.OptionID), UpdatedAt: a.UpdatedAt, DeletedAt: timePtr(a.DeletedAt),
			}
		}
		for i, r := range pp {
			out.Progress[i] = api.UserProgress{
				Target: api.TargetKind(r.Target), TargetRef: r.TargetRef, Position: textPtr(r.Position), Percent: int4Ptr(r.Percent), UpdatedAt: r.UpdatedAt,
			}
		}
		return nil
	})
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23503" { // FK violation, e.g. unknown enum-ish ref
			return bad("invalid reference in sync payload")
		}
		return nil, err
	}
	return out, nil
}

func textFromPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func int4FromPtr(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}
