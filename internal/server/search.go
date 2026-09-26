package server

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/store"
)

const (
	searchPerGroup = 5
	snippetRunes   = 140
)

// refRe matches a typed Bible reference such as "Yohana 3:16", "1 Yohana 2" or "Jn 3:16".
var refRe = regexp.MustCompile(`^\s*((?:[1-3]\s*)?\p{L}[\p{L}.]*)\s+(\d{1,3})(?::(\d{1,3}))?\s*$`)

type searchGroup = struct {
	Hits  []api.SearchHit `json:"hits"`
	Scope api.SearchScope `json:"scope"`
}

func (s *Server) Search(ctx context.Context, req api.SearchRequestObject) (api.SearchResponseObject, error) {
	q := strings.TrimSpace(req.Params.Q)
	if utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 100 {
		return api.Search400JSONResponse{BadRequestJSONResponse: badRequest("q must be 2–100 characters")}, nil
	}
	lang, ok := normLang(req.Params.Lang)
	if !ok {
		return api.Search400JSONResponse{BadRequestJSONResponse: badRequest("invalid lang")}, nil
	}
	want := func(sc api.SearchScope) bool { return req.Params.Scope == nil || *req.Params.Scope == sc }
	lim := int32(searchPerGroup)

	out := api.Search200JSONResponse{Groups: []searchGroup{}}
	add := func(sc api.SearchScope, hits []api.SearchHit) {
		if len(hits) > 0 {
			out.Groups = append(out.Groups, searchGroup{Scope: sc, Hits: hits})
		}
	}

	if want(api.SearchScopeBible) {
		var hits []api.SearchHit
		if hit, ok, err := s.searchRef(ctx, q, lang); err != nil {
			return nil, err
		} else if ok {
			hits = append(hits, hit)
		}
		rows, err := s.q.SearchVerses(ctx, store.SearchVersesParams{Lang: lang, Q: q, Lim: lim})
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if len(hits) > 0 && hits[0].TargetRef == r.OsisRef {
				continue
			}
			name := r.Book
			if r.BookName.Valid {
				name = r.BookName.String
			}
			hits = append(hits, api.SearchHit{
				Title: fmt.Sprintf("%s %d:%d", name, r.Chapter, r.Verse), Snippet: snippet(r.Text, q),
				Target: api.SearchHitTargetVerse, TargetRef: r.OsisRef,
			})
		}
		if len(hits) > searchPerGroup {
			hits = hits[:searchPerGroup]
		}
		add(api.SearchScopeBible, hits)
	}
	if want(api.SearchScopeEgw) {
		rows, err := s.q.SearchEgw(ctx, store.SearchEgwParams{Lang: lang, Q: q, Lim: lim})
		if err != nil {
			return nil, err
		}
		hits := make([]api.SearchHit, 0, len(rows))
		for _, r := range rows {
			ed, ch := int(r.EditionID), int(r.Chapter)
			hits = append(hits, api.SearchHit{
				Title: r.Title + " · " + r.Refcode, Snippet: snippet(r.Text, q), Target: api.SearchHitTargetEgwParagraph,
				TargetRef: r.Refcode, EditionId: &ed, Chapter: &ch,
			})
		}
		add(api.SearchScopeEgw, hits)
	}
	if want(api.SearchScopeBeliefs) {
		rows, err := s.q.SearchBeliefs(ctx, store.SearchBeliefsParams{Lang: lang, Q: q, Lim: lim})
		if err != nil {
			return nil, err
		}
		hits := make([]api.SearchHit, 0, len(rows))
		for _, r := range rows {
			hits = append(hits, api.SearchHit{Title: fmt.Sprintf("%d · %s", r.N, r.Title), Snippet: snippet(r.Body, q), Target: api.SearchHitTargetBelief, TargetRef: strconv.Itoa(int(r.N))})
		}
		add(api.SearchScopeBeliefs, hits)
	}
	if want(api.SearchScopeHymns) {
		rows, err := s.q.SearchHymns(ctx, store.SearchHymnsParams{Lang: lang, Q: q, Lim: lim})
		if err != nil {
			return nil, err
		}
		hits := make([]api.SearchHit, 0, len(rows))
		for _, r := range rows {
			hits = append(hits, api.SearchHit{
				Title: fmt.Sprintf("%d · %s", r.Number, r.Title), Snippet: r.Category.String,
				Target: api.SearchHitTargetHymn, TargetRef: fmt.Sprintf("%s/%d", r.Code, r.Number),
			})
		}
		add(api.SearchScopeHymns, hits)
	}
	if want(api.SearchScopeVideo) {
		rows, err := s.q.SearchVideos(ctx, store.SearchVideosParams{Lang: lang, Q: q, Lim: lim})
		if err != nil {
			return nil, err
		}
		hits := make([]api.SearchHit, 0, len(rows))
		for _, r := range rows {
			title := r.Kicker
			if r.RefLabel.Valid {
				title += " · " + r.RefLabel.String
			}
			hits = append(hits, api.SearchHit{Title: title, Snippet: snippet(r.Body, q), Target: api.SearchHitTargetFeedItem, TargetRef: r.ID.String()})
		}
		add(api.SearchScopeVideo, hits)
	}
	return out, nil
}

// searchRef resolves a typed reference ("Yohana 3:16") to a verse hit in the user's language.
func (s *Server) searchRef(ctx context.Context, q, lang string) (api.SearchHit, bool, error) {
	m := refRe.FindStringSubmatch(q)
	if m == nil {
		return api.SearchHit{}, false, nil
	}
	book, err := s.q.FindBookByName(ctx, store.FindBookByNameParams{Name: strings.Join(strings.Fields(m[1]), " "), Lang: lang})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.SearchHit{}, false, nil
	}
	if err != nil {
		return api.SearchHit{}, false, err
	}
	ch, _ := strconv.Atoi(m[2])
	verse := 1
	if m[3] != "" {
		verse, _ = strconv.Atoi(m[3])
	}
	v, err := s.q.GetVerseInLang(ctx, store.GetVerseInLangParams{Lang: lang, Book: book, Chapter: int32(ch), Verse: int32(verse)})
	if errors.Is(err, pgx.ErrNoRows) {
		return api.SearchHit{}, false, nil
	}
	if err != nil {
		return api.SearchHit{}, false, err
	}
	name := book
	if v.BookName.Valid {
		name = v.BookName.String
	}
	return api.SearchHit{Title: fmt.Sprintf("%s %d:%d", name, ch, verse), Snippet: snippet(v.Text, ""), Target: api.SearchHitTargetVerse, TargetRef: v.OsisRef}, true, nil
}

// snippet returns ~140 runes of text centred on the first case-insensitive match of q.
func snippet(text, q string) string {
	r := []rune(strings.Join(strings.Fields(text), " "))
	if len(r) <= snippetRunes {
		return string(r)
	}
	start := 0
	if q != "" {
		if i := strings.Index(strings.ToLower(string(r)), strings.ToLower(q)); i >= 0 {
			start = utf8.RuneCountInString(strings.ToLower(string(r))[:i]) - snippetRunes/3
		}
	}
	start = max(0, min(start, len(r)-snippetRunes))
	out := string(r[start : start+snippetRunes])
	if start > 0 {
		out = "…" + out
	}
	if start+snippetRunes < len(r) {
		out += "…"
	}
	return out
}
