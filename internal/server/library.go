package server

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/store"
)

func int4Ptr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}

// ── EGW ─────────────────────────────────────────────────

func (s *Server) ListEgwBooks(ctx context.Context, req api.ListEgwBooksRequestObject) (api.ListEgwBooksResponseObject, error) {
	lang, ok := normLang(req.Params.Lang)
	if !ok {
		return api.ListEgwBooks400JSONResponse{BadRequestJSONResponse: badRequest("invalid lang")}, nil
	}
	rows, err := s.q.ListEgwBooks(ctx, lang)
	if err != nil {
		return nil, err
	}
	books := make([]api.EgwBook, len(rows))
	for i, r := range rows {
		books[i] = api.EgwBook{EditionId: int(r.ID), BookCode: r.BookCode, Lang: r.Lang, Title: r.Title, OriginalTitle: r.OriginalTitle, Chapters: int(r.Chapters)}
	}
	return api.ListEgwBooks200JSONResponse{Books: books}, nil
}

func (s *Server) GetEgwChapter(ctx context.Context, req api.GetEgwChapterRequestObject) (api.GetEgwChapterResponseObject, error) {
	nf := func(msg string) (api.GetEgwChapterResponseObject, error) {
		return api.GetEgwChapter404JSONResponse{NotFoundJSONResponse: notFound(msg)}, nil
	}
	ed, err := s.q.GetEgwEdition(ctx, int32(req.Edition))
	if errors.Is(err, pgx.ErrNoRows) {
		return nf("edition not found")
	}
	if err != nil {
		return nil, err
	}
	paras, err := s.q.ListEgwChapterParagraphs(ctx, store.ListEgwChapterParagraphsParams{EditionID: ed.ID, Chapter: int32(req.N)})
	if err != nil {
		return nil, err
	}
	if len(paras) == 0 {
		return nf("chapter not found")
	}

	out := api.GetEgwChapter200JSONResponse{
		EditionId: int(ed.ID), BookCode: ed.BookCode, Title: ed.Title, Chapter: req.N, Chapters: int(ed.Chapters),
		ChapterTitle: textPtr(paras[0].ChapterTitle), Paragraphs: make([]api.EgwParagraph, len(paras)),
	}
	for i, p := range paras {
		out.Paragraphs[i] = api.EgwParagraph{Refcode: p.Refcode, Ord: int(p.Ord), Page: int4Ptr(p.Page), Text: p.Text}
	}

	// Parallel text: the same book in another language, aligned by EGW refcode. A language without that
	// edition simply returns no parallel text.
	if pl, ok := normLang(req.Params.Parallel); ok && req.Params.Parallel != nil && *req.Params.Parallel != "" && pl != ed.Lang {
		pid, err := s.q.GetEgwEditionByBookLang(ctx, store.GetEgwEditionByBookLangParams{BookCode: ed.BookCode, Lang: pl})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return nil, err
		default:
			pparas, err := s.q.ListEgwChapterParagraphs(ctx, store.ListEgwChapterParagraphsParams{EditionID: pid, Chapter: int32(req.N)})
			if err != nil {
				return nil, err
			}
			byRef := make(map[string]string, len(pparas))
			for _, p := range pparas {
				byRef[p.Refcode] = p.Text
			}
			for i := range out.Paragraphs {
				if t, ok := byRef[out.Paragraphs[i].Refcode]; ok {
					out.Paragraphs[i].ParallelText = &t
				}
			}
			out.ParallelLang = &pl
		}
	}
	return out, nil
}

// ── Fundamental Beliefs ─────────────────────────────────

func (s *Server) ListBeliefs(ctx context.Context, req api.ListBeliefsRequestObject) (api.ListBeliefsResponseObject, error) {
	lang, ok := normLang(req.Params.Lang)
	if !ok {
		return api.ListBeliefs400JSONResponse{BadRequestJSONResponse: badRequest("invalid lang")}, nil
	}
	rows, err := s.q.ListBeliefs(ctx, lang)
	if err != nil {
		return nil, err
	}
	out := make([]api.Belief, len(rows))
	for i, r := range rows {
		out[i] = api.Belief{N: int(r.N), GroupKey: api.BeliefGroupKey(r.GroupKey), Title: r.Title}
	}
	return api.ListBeliefs200JSONResponse{Beliefs: out}, nil
}

func (s *Server) GetBelief(ctx context.Context, req api.GetBeliefRequestObject) (api.GetBeliefResponseObject, error) {
	lang, ok := normLang(req.Params.Lang)
	if !ok || req.N < 1 || req.N > 28 {
		return api.GetBelief404JSONResponse{NotFoundJSONResponse: notFound("belief not found")}, nil
	}
	r, err := s.q.GetBelief(ctx, store.GetBeliefParams{N: int32(req.N), Lang: lang})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetBelief404JSONResponse{NotFoundJSONResponse: notFound("belief not found")}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetBelief200JSONResponse{N: int(r.N), GroupKey: api.BeliefGroupKey(r.GroupKey), Title: r.Title, Body: &r.Body}, nil
}

// ── Hymnals ─────────────────────────────────────────────

func (s *Server) ListHymnals(ctx context.Context, _ api.ListHymnalsRequestObject) (api.ListHymnalsResponseObject, error) {
	rows, err := s.q.ListHymnals(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Hymnal, len(rows))
	for i, r := range rows {
		out[i] = api.Hymnal{Code: r.Code, Lang: r.Lang, Name: r.Name}
	}
	return api.ListHymnals200JSONResponse{Hymnals: out}, nil
}

func (s *Server) ListHymns(ctx context.Context, req api.ListHymnsRequestObject) (api.ListHymnsResponseObject, error) {
	h, err := s.q.GetHymnal(ctx, strings.ToUpper(req.Code))
	if errors.Is(err, pgx.ErrNoRows) {
		return api.ListHymns404JSONResponse{NotFoundJSONResponse: notFound("hymnal not found")}, nil
	}
	if err != nil {
		return nil, err
	}
	params := store.ListHymnsParams{HymnalID: h.ID}
	if req.Params.Q != nil {
		if q := strings.TrimSpace(*req.Params.Q); q != "" {
			if n, err := strconv.Atoi(q); err == nil {
				params.QNumber = pgtype.Int4{Int32: int32(n), Valid: true}
			} else {
				params.QTitle = pgtype.Text{String: q, Valid: true}
			}
		}
	}
	rows, err := s.q.ListHymns(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]api.HymnSummary, len(rows))
	for i, r := range rows {
		out[i] = api.HymnSummary{Number: int(r.Number), Title: r.Title, OriginalTitle: textPtr(r.OriginalTitle), Category: textPtr(r.Category), HasAudio: r.HasAudio}
	}
	return api.ListHymns200JSONResponse{Hymns: out}, nil
}

func (s *Server) GetHymn(ctx context.Context, req api.GetHymnRequestObject) (api.GetHymnResponseObject, error) {
	nf := api.GetHymn404JSONResponse{NotFoundJSONResponse: notFound("hymn not found")}
	h, err := s.q.GetHymnal(ctx, strings.ToUpper(req.Code))
	if errors.Is(err, pgx.ErrNoRows) {
		return nf, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := s.q.GetHymn(ctx, store.GetHymnParams{HymnalID: h.ID, Number: int32(req.Number)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nf, nil
	}
	if err != nil {
		return nil, err
	}
	stanzas, err := s.q.ListHymnStanzas(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	out := api.GetHymn200JSONResponse{
		Hymnal: h.Code, Number: int(r.Number), Title: r.Title, OriginalTitle: textPtr(r.OriginalTitle), Category: textPtr(r.Category),
		Stanzas:    make([]api.HymnStanza, len(stanzas)),
		AudioChoir: toMedia(r.ChoirID, r.ChoirKind, r.ChoirUrl, r.ChoirHlsUrl, r.ChoirBytes, r.ChoirDurationS),
		AudioPiano: toMedia(r.PianoID, r.PianoKind, r.PianoUrl, r.PianoHlsUrl, r.PianoBytes, r.PianoDurationS),
	}
	for i, st := range stanzas {
		out.Stanzas[i] = api.HymnStanza{Idx: int(st.Idx), Kind: api.HymnStanzaKind(st.Kind), Text: st.Text}
	}
	return out, nil
}

// ── Sabbath School ──────────────────────────────────────

func (s *Server) GetSabbathSchoolCurrent(ctx context.Context, req api.GetSabbathSchoolCurrentRequestObject) (api.GetSabbathSchoolCurrentResponseObject, error) {
	nf := api.GetSabbathSchoolCurrent404JSONResponse{NotFoundJSONResponse: notFound("no lesson for this week")}
	lang, ok := normLang(req.Params.Lang)
	if !ok {
		return nf, nil
	}
	day := s.now().In(feedTZ)
	if req.Params.Date != nil {
		day = req.Params.Date.Time
	}
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)

	l, err := s.q.GetSabbathSchoolLesson(ctx, store.GetSabbathSchoolLessonParams{Lang: lang, Day: pgtype.Date{Time: day, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return nf, nil
	}
	if err != nil {
		return nil, err
	}
	days, err := s.q.ListSabbathSchoolDays(ctx, l.LessonID)
	if err != nil {
		return nil, err
	}
	out := api.GetSabbathSchoolCurrent200JSONResponse{
		Year: int(l.Year), Quarter: int(l.Quarter), QuarterTitle: l.QuarterTitle, LessonN: int(l.N), LessonTitle: l.Title,
		WeekStart: openapi_types.Date{Time: l.WeekStart.Time}, MemoryRef: textPtr(l.MemoryRef), MemoryText: textPtr(l.MemoryText),
		Days: make([]api.SabbathSchoolDay, len(days)),
	}
	for i, d := range days {
		date := openapi_types.Date{Time: l.WeekStart.Time.AddDate(0, 0, int(d.DayIdx))}
		out.Days[i] = api.SabbathSchoolDay{
			DayIdx: int(d.DayIdx), Date: &date, Title: d.Title, Body: d.Body, Question: textPtr(d.Question),
			Audio: toMedia(d.MediaID, d.MediaKind, d.MediaUrl, d.MediaHlsUrl, d.MediaBytes, d.MediaDurationS),
		}
	}
	return out, nil
}

// ── Courses ─────────────────────────────────────────────

func (s *Server) ListCourses(ctx context.Context, req api.ListCoursesRequestObject) (api.ListCoursesResponseObject, error) {
	lang, ok := normLang(req.Params.Lang)
	if !ok {
		return api.ListCourses400JSONResponse{BadRequestJSONResponse: badRequest("invalid lang")}, nil
	}
	rows, err := s.q.ListCourses(ctx, lang)
	if err != nil {
		return nil, err
	}
	out := make([]api.Course, len(rows))
	for i, r := range rows {
		out[i] = api.Course{Id: int(r.ID), Slug: r.Slug, Title: r.Title, Audience: r.Audience, Lessons: int(r.Lessons)}
	}
	return api.ListCourses200JSONResponse{Courses: out}, nil
}

func (s *Server) GetCourseLesson(ctx context.Context, req api.GetCourseLessonRequestObject) (api.GetCourseLessonResponseObject, error) {
	l, err := s.q.GetCourseLesson(ctx, store.GetCourseLessonParams{CourseID: int32(req.Id), N: int32(req.N)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetCourseLesson404JSONResponse{NotFoundJSONResponse: notFound("lesson not found")}, nil
	}
	if err != nil {
		return nil, err
	}
	qs, err := s.q.ListLessonQuestions(ctx, l.LessonID)
	if err != nil {
		return nil, err
	}
	opts, err := s.q.ListLessonOptions(ctx, l.LessonID)
	if err != nil {
		return nil, err
	}
	byQ := map[int32][]api.QuizOption{}
	for _, o := range opts {
		byQ[o.QuestionID] = append(byQ[o.QuestionID], api.QuizOption{Id: int(o.ID), Label: o.Label, IsCorrect: o.IsCorrect})
	}
	out := api.GetCourseLesson200JSONResponse{
		CourseId: int(l.CourseID), CourseTitle: l.CourseTitle, N: int(l.N), Total: int(l.Total), Title: l.Title, Body: l.Body,
		Image:     toMedia(l.MediaID, l.MediaKind, l.MediaUrl, l.MediaHlsUrl, l.MediaBytes, l.MediaDurationS),
		Questions: make([]api.QuizQuestion, len(qs)),
	}
	for i, q := range qs {
		o := byQ[q.ID]
		if o == nil {
			o = []api.QuizOption{}
		}
		out.Questions[i] = api.QuizQuestion{Id: int(q.ID), Prompt: q.Prompt, ExplainRef: textPtr(q.ExplainRef), ExplainText: textPtr(q.ExplainText), Options: o}
	}
	return out, nil
}

func (s *Server) AnswerQuiz(ctx context.Context, req api.AnswerQuizRequestObject) (api.AnswerQuizResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return api.AnswerQuiz400JSONResponse{BadRequestJSONResponse: badRequest("body required")}, nil
	}
	r, err := s.q.GetQuizOptionForLesson(ctx, store.GetQuizOptionForLessonParams{
		OptionID: int32(req.Body.OptionId), QuestionID: int32(req.Body.QuestionId), CourseID: int32(req.Id), N: int32(req.N),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.AnswerQuiz404JSONResponse{NotFoundJSONResponse: notFound("question or option not found in this lesson")}, nil
	}
	if err != nil {
		return nil, err
	}

	// One answer per (user, question): update in place, insert on first answer.
	ref := strconv.Itoa(req.Body.QuestionId)
	opt := pgtype.Int4{Int32: int32(req.Body.OptionId), Valid: true}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		n, err := q.UpdateQuizAnswer(ctx, store.UpdateQuizAnswerParams{OptionID: opt, UserID: p.UserID, TargetRef: ref})
		if err != nil || n > 0 {
			return err
		}
		return q.InsertQuizAnswer(ctx, store.InsertQuizAnswerParams{UserID: p.UserID, TargetRef: ref, OptionID: opt})
	})
	if err != nil {
		return nil, err
	}
	return api.AnswerQuiz200JSONResponse{Correct: r.IsCorrect, CorrectOptionId: int(r.CorrectOptionID)}, nil
}
