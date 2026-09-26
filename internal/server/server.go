// Package server implements the OpenAPI StrictServerInterface on top of the sqlc store.
package server

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/auth"
	"github.com/PatzPaul/Neno-API/internal/store"
)

const (
	defaultLang      = "sw"
	fallbackLang     = "en"
	defaultFeedLimit = 20
	maxFeedLimit     = 50
)

var langRe = regexp.MustCompile(`^[a-z]{2,3}$`)

type Server struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	verifier *auth.Verifier
	users    userCache
	now      func() time.Time
}

var _ api.StrictServerInterface = (*Server)(nil)

// New wires the handlers. A nil verifier rejects every protected operation (401).
func New(pool *pgxpool.Pool, verifier *auth.Verifier) *Server {
	return &Server{pool: pool, q: store.New(pool), verifier: verifier, users: userCache{seen: map[uuid.UUID]cachedUser{}}, now: time.Now}
}

func badRequest(msg string) api.BadRequestJSONResponse { return api.BadRequestJSONResponse{Error: msg} }
func notFound(msg string) api.NotFoundJSONResponse     { return api.NotFoundJSONResponse{Error: msg} }

// normLang reduces a BCP 47 tag to its primary subtag ("sw-TZ" → "sw"); content is stored per base language.
func normLang(p *string) (string, bool) {
	if p == nil || *p == "" {
		return defaultLang, true
	}
	l := strings.ToLower(strings.SplitN(*p, "-", 2)[0])
	return l, langRe.MatchString(l)
}

func (s *Server) GetHealth(ctx context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.q.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "health: db ping failed", "err", err)
		return api.GetHealth503JSONResponse{Status: "degraded", Db: "down"}, nil
	}
	return api.GetHealth200JSONResponse{Status: "ok", Db: "ok"}, nil
}

func (s *Server) ListPacks(ctx context.Context, req api.ListPacksRequestObject) (api.ListPacksResponseObject, error) {
	lang, ok := normLang(req.Params.Lang)
	if !ok {
		return api.ListPacks400JSONResponse{BadRequestJSONResponse: badRequest("invalid lang")}, nil
	}
	rows, err := s.q.ListLatestPacks(ctx, lang)
	if err != nil {
		return nil, err
	}
	packs := make([]api.Pack, len(rows))
	for i, r := range rows {
		packs[i] = api.Pack{Slug: r.Slug, Lang: r.Lang, Version: int(r.Version), Url: r.Url, Bytes: r.Bytes, Sha256: r.Sha256}
	}
	return api.ListPacks200JSONResponse{Packs: packs}, nil
}

func (s *Server) ListFeed(ctx context.Context, req api.ListFeedRequestObject) (api.ListFeedResponseObject, error) {
	p := req.Params
	bad := func(msg string) (api.ListFeedResponseObject, error) {
		return api.ListFeed400JSONResponse{BadRequestJSONResponse: badRequest(msg)}, nil
	}

	limit := defaultFeedLimit
	if p.Limit != nil {
		if *p.Limit < 1 || *p.Limit > maxFeedLimit {
			return bad("limit must be between 1 and 50")
		}
		limit = *p.Limit
	}
	kinds, err := parseKinds(p.Kinds)
	if err != nil {
		return bad(err.Error())
	}

	params := store.ListFeedParams{Kinds: kinds, Lim: int32(limit) + 1} // +1 to detect a next page
	if p.Cursor != nil && *p.Cursor != "" {
		c, err := decodeCursor(*p.Cursor)
		if err != nil {
			return bad("invalid cursor")
		}
		// The cursor pins the language so a fallback feed keeps paging in the fallback language.
		params.Lang = c.Lang
		params.CursorAt = c.pgAt()
		params.CursorID = c.nullID()
	} else {
		lang, ok := normLang(p.Lang)
		if !ok {
			return bad("invalid lang")
		}
		params.Lang = lang
	}

	rows, err := s.q.ListFeed(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 && p.Cursor == nil && params.Lang != fallbackLang {
		params.Lang = fallbackLang
		if rows, err = s.q.ListFeed(ctx, params); err != nil {
			return nil, err
		}
	}

	page := api.ListFeed200JSONResponse{Lang: params.Lang, Items: make([]api.FeedItem, 0, limit)}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next := encodeCursor(feedCursor{Lang: params.Lang, At: last.PublishAt.Time, ID: last.ID})
		page.NextCursor = &next
	}
	for _, r := range rows {
		page.Items = append(page.Items, toFeedItem(r))
	}
	page.Items = arrangeFeed(page.Items, p.Cursor == nil || *p.Cursor == "", s.now())
	return page, nil
}

func (s *Server) GetFeedItem(ctx context.Context, req api.GetFeedItemRequestObject) (api.GetFeedItemResponseObject, error) {
	row, err := s.q.GetFeedItem(ctx, req.Id)
	if errors.Is(err, pgx.ErrNoRows) {
		return api.GetFeedItem404JSONResponse{NotFoundJSONResponse: notFound("feed item not found")}, nil
	}
	if err != nil {
		return nil, err
	}
	links, err := s.q.ListFeedLinks(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	it := toFeedItem(store.ListFeedRow(row))
	out := api.GetFeedItem200JSONResponse{
		Id: it.Id, Kind: it.Kind, Lang: it.Lang, Kicker: it.Kicker, Source: it.Source, Body: it.Body,
		RefLabel: it.RefLabel, AltLang: it.AltLang, AltBody: it.AltBody, Media: it.Media,
		CtaLabel: it.CtaLabel, CtaTarget: it.CtaTarget, Audience: it.Audience, PublishAt: it.PublishAt,
		LikeCount: it.LikeCount,
		Links:     make([]api.FeedLink, len(links)),
	}
	for i, l := range links {
		out.Links[i] = api.FeedLink{Target: api.FeedLinkTarget(l.Target), TargetRef: l.TargetRef, Label: l.Label}
	}
	return out, nil
}

func (s *Server) GetBibleChapter(ctx context.Context, req api.GetBibleChapterRequestObject) (api.GetBibleChapterResponseObject, error) {
	nf := func(msg string) (api.GetBibleChapterResponseObject, error) {
		return api.GetBibleChapter404JSONResponse{NotFoundJSONResponse: notFound(msg)}, nil
	}
	tr, err := s.q.GetTranslation(ctx, strings.ToUpper(req.Translation))
	if errors.Is(err, pgx.ErrNoRows) {
		return nf("unknown translation")
	}
	if err != nil {
		return nil, err
	}
	book := strings.ToUpper(req.Book)
	verses, err := s.q.ListChapterVerses(ctx, store.ListChapterVersesParams{TranslationID: tr.ID, Book: book, Chapter: int32(req.Chapter)})
	if err != nil {
		return nil, err
	}
	if len(verses) == 0 {
		return nf("chapter not found")
	}

	out := api.GetBibleChapter200JSONResponse{Translation: tr.Code, Book: book, Chapter: req.Chapter, Verses: make([]api.Verse, len(verses))}
	if name, err := s.q.GetBookName(ctx, store.GetBookNameParams{Book: book, Lang: tr.Lang}); err == nil {
		out.BookName = &name
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	for i, v := range verses {
		out.Verses[i] = api.Verse{OsisRef: v.OsisRef, Verse: int(v.Verse), Text: v.Text}
	}

	if req.Params.Parallel != nil && *req.Params.Parallel != "" {
		ptr, err := s.q.GetTranslation(ctx, strings.ToUpper(*req.Params.Parallel))
		if errors.Is(err, pgx.ErrNoRows) {
			return nf("unknown parallel translation")
		}
		if err != nil {
			return nil, err
		}
		pverses, err := s.q.ListChapterVerses(ctx, store.ListChapterVersesParams{TranslationID: ptr.ID, Book: book, Chapter: int32(req.Chapter)})
		if err != nil {
			return nil, err
		}
		// Align by verse number; verses missing from the parallel translation stay without parallel_text.
		byVerse := make(map[int32]string, len(pverses))
		for _, v := range pverses {
			byVerse[v.Verse] = v.Text
		}
		for i := range out.Verses {
			if t, ok := byVerse[int32(out.Verses[i].Verse)]; ok {
				out.Verses[i].ParallelText = &t
			}
		}
		out.Parallel = &ptr.Code
	}
	return out, nil
}

func (s *Server) ListBibleBooks(ctx context.Context, req api.ListBibleBooksRequestObject) (api.ListBibleBooksResponseObject, error) {
	tr, err := s.q.GetTranslation(ctx, strings.ToUpper(req.Translation))
	if errors.Is(err, pgx.ErrNoRows) {
		return api.ListBibleBooks404JSONResponse{NotFoundJSONResponse: notFound("unknown translation")}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListBibleBooks(ctx, store.ListBibleBooksParams{TranslationID: tr.ID, Lang: tr.Lang})
	if err != nil {
		return nil, err
	}
	books := make([]api.BibleBook, len(rows))
	for i, r := range rows {
		books[i] = api.BibleBook{
			Osis: r.Osis, Ord: int(r.Ord), Testament: api.BibleBookTestament(r.Testament), Name: r.Name, Chapters: int(r.Chapters),
		}
		if r.Abbr != "" {
			abbr := r.Abbr
			books[i].Abbr = &abbr
		}
	}
	return api.ListBibleBooks200JSONResponse{Translation: tr.Code, Books: books}, nil
}
