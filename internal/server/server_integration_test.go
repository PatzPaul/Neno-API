package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatzPaul/Neno-API/internal/api"
	"github.com/PatzPaul/Neno-API/internal/auth"
)

// Integration tests run against a migrated + seeded database (`make migrate seed`). They create throwaway
// users (random Keycloak subs) and delete them afterwards.
//
//	NENO_TEST_DATABASE_URL=$DATABASE_URL go test ./internal/server/

const (
	testIssuer   = "https://test.invalid/realms/neno"
	testAudience = "neno-api"
)

type testEnv struct {
	ts   *httptest.Server
	pool *pgxpool.Pool
	key  *rsa.PrivateKey
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dbURL := os.Getenv("NENO_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("NENO_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	b64 := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	jwks, _ := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256",
		"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
	}}})
	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwks)
	}))
	t.Cleanup(jwksSrv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	v, err := auth.New(ctx, auth.Config{Issuer: testIssuer, Audience: testAudience, JWKSURL: jwksSrv.URL})
	if err != nil {
		t.Fatal(err)
	}
	srv := New(pool, v)
	h := api.HandlerWithOptions(
		api.NewStrictHandler(srv, []api.StrictMiddlewareFunc{srv.AuthMiddleware()}),
		api.StdHTTPServerOptions{BaseRouter: http.NewServeMux()},
	)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return &testEnv{ts: ts, pool: pool, key: key}
}

type tokenOpts struct {
	aud   string
	roles []string
	key   *rsa.PrivateKey
}

// newUser returns a fresh Keycloak-style subject and a signed access token for it; the user row is removed
// when the test ends.
func (e *testEnv) newUser(t *testing.T, o tokenOpts) (uuid.UUID, string) {
	t.Helper()
	sub := uuid.New()
	t.Cleanup(func() { _, _ = e.pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", sub) })
	return sub, e.token(t, sub, o)
}

func (e *testEnv) token(t *testing.T, sub uuid.UUID, o tokenOpts) string {
	t.Helper()
	if o.aud == "" {
		o.aud = testAudience
	}
	if o.key == nil {
		o.key = e.key
	}
	claims := jwt.MapClaims{
		"iss": testIssuer, "aud": []string{o.aud, "account"}, "sub": sub.String(),
		"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(),
		"email": "test-" + sub.String()[:8] + "@example.invalid", "name": "Test Mtumiaji",
		"realm_access": map[string]any{"roles": o.roles},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "test"
	s, err := tok.SignedString(o.key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *testEnv) do(t *testing.T, method, path, token string, body any, wantStatus int, out any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != wantStatus {
		var e map[string]any
		_ = json.NewDecoder(res.Body).Decode(&e)
		t.Fatalf("%s %s: status %d, want %d (%v)", method, path, res.StatusCode, wantStatus, e)
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
}

func (e *testEnv) get(t *testing.T, path string, wantStatus int, out any) {
	t.Helper()
	e.do(t, http.MethodGet, path, "", nil, wantStatus, out)
}

// ── Feed ────────────────────────────────────────────────

const seedSwFeedItems = 6 // published, past, sw items in db/seed/sample.sql

func TestFeedPaginationNoDupOrSkip(t *testing.T) {
	e := newTestEnv(t)
	var all api.FeedPage
	e.get(t, "/v1/feed?lang=sw&limit=50", 200, &all)
	if len(all.Items) != seedSwFeedItems {
		t.Fatalf("single page: %d items, want %d", len(all.Items), seedSwFeedItems)
	}
	if all.Items[0].Kind != api.FeedKindVerse {
		t.Fatalf("first item should be Aya ya Siku (verse), got %s", all.Items[0].Kind)
	}
	if all.Items[1].Kind != api.FeedKindSabbathSchool {
		t.Fatalf("second item should be today's Sabbath School, got %s", all.Items[1].Kind)
	}

	seen := map[uuid.UUID]bool{}
	path := "/v1/feed?lang=sw&limit=2"
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
		var page api.FeedPage
		e.get(t, path, 200, &page)
		for _, it := range page.Items {
			if seen[it.Id] {
				t.Fatalf("item %s served twice", it.Id)
			}
			seen[it.Id] = true
			if it.LikeCount == nil {
				t.Fatalf("item %s has no like_count", it.Id)
			}
		}
		if page.NextCursor == nil {
			break
		}
		path = "/v1/feed?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	for _, it := range all.Items {
		if !seen[it.Id] {
			t.Fatalf("item %s skipped by pagination", it.Id)
		}
	}
	for _, hidden := range []string{"00000000-0000-4000-8000-000000000007", "00000000-0000-4000-8000-000000000008"} {
		if seen[uuid.MustParse(hidden)] {
			t.Fatalf("draft/future item %s was served", hidden)
		}
	}
}

func TestFeedKindsAndFallback(t *testing.T) {
	e := newTestEnv(t)
	var page api.FeedPage
	e.get(t, "/v1/feed?kinds=prophecy", 200, &page)
	if len(page.Items) != 1 || page.Items[0].Media == nil || page.Items[0].Media.Bytes == nil {
		t.Fatalf("want 1 prophecy item with media bytes, got %+v", page.Items)
	}
	e.get(t, "/v1/feed?lang=fr", 200, &page)
	if page.Lang != "en" || len(page.Items) == 0 {
		t.Fatalf("fr should fall back to en, got lang=%s n=%d", page.Lang, len(page.Items))
	}
	e.get(t, "/v1/feed?kinds=nope", 400, nil)
	e.get(t, "/v1/feed?limit=500", 400, nil)
	e.get(t, "/v1/feed?cursor=garbage", 400, nil)
}

func TestFeedItemLinks(t *testing.T) {
	e := newTestEnv(t)
	var it api.FeedItemDetail
	e.get(t, "/v1/feed/00000000-0000-4000-8000-000000000001", 200, &it)
	if len(it.Links) != 3 || it.Links[0].TargetRef != "JHN.3.17" {
		t.Fatalf("links = %+v", it.Links)
	}
	e.get(t, "/v1/feed/00000000-0000-4000-8000-000000000007", 404, nil) // draft
	e.get(t, "/v1/feed/not-a-uuid", 400, nil)
}

// ── Auth ────────────────────────────────────────────────

func TestAuthRequired(t *testing.T) {
	e := newTestEnv(t)
	item := "/v1/feed/00000000-0000-4000-8000-000000000002/like"
	e.do(t, "GET", "/v1/me", "", nil, 401, nil)
	e.do(t, "POST", "/v1/sync", "", map[string]any{}, 401, nil)
	e.do(t, "POST", item, "", nil, 401, nil)
	e.do(t, "POST", "/v1/courses/1/lessons/3/answers", "", map[string]int{"question_id": 1, "option_id": 1}, 401, nil)

	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	sub := uuid.New()
	e.do(t, "GET", "/v1/me", e.token(t, sub, tokenOpts{key: otherKey}), nil, 401, nil)      // wrong signature
	e.do(t, "GET", "/v1/me", e.token(t, sub, tokenOpts{aud: "other"}), nil, 401, nil)       // wrong audience
	e.do(t, "GET", "/v1/me", "not.a.jwt", nil, 401, nil)                                    // garbage
	e.get(t, "/v1/beliefs", 200, nil)                                                       // public stays public
	e.do(t, "GET", "/v1/beliefs", e.token(t, sub, tokenOpts{key: otherKey}), nil, 200, nil) // bad token ignored on public ops
}

func TestMeCreateAndPatch(t *testing.T) {
	e := newTestEnv(t)
	sub, tok := e.newUser(t, tokenOpts{roles: []string{"offline_access", "editor", "reviewer"}})

	var me api.Me
	e.do(t, "GET", "/v1/me", tok, nil, 200, &me)
	if me.Id != sub || me.Role != api.Reviewer || me.Email == nil || me.DisplayName == nil || *me.DisplayName != "Test Mtumiaji" {
		t.Fatalf("me = %+v", me)
	}
	if me.UiLang != "sw" || me.BibleTranslation != "SUV" || me.TextScale != 1 {
		t.Fatalf("defaults = %+v", me)
	}

	for _, bad := range []map[string]any{
		{"text_scale": 3}, {"ui_lang": "xx"}, {"parallel_lang": "zz"}, {"bible_translation": "NOPE"},
		{"sunset_lat": 91}, {"sunset_lng": -181}, {"display_name": string(make([]byte, 81))},
	} {
		e.do(t, "PATCH", "/v1/me", tok, bad, 400, nil)
	}

	e.do(t, "PATCH", "/v1/me", tok, map[string]any{
		"display_name": "Neema", "church": "Kanisa la Mfano", "ui_lang": "en", "parallel_lang": "sw", "text_scale": 1.5,
		"data_saver": true, "sunset_city": "Nairobi", "sunset_lat": -1.2921, "sunset_lng": 36.8219,
	}, 200, &me)
	if *me.DisplayName != "Neema" || me.UiLang != "en" || *me.ParallelLang != "sw" || me.TextScale != 1.5 || !me.DataSaver ||
		*me.SunsetCity != "Nairobi" || me.SunsetLat == nil || *me.SunsetLat > -1.29 {
		t.Fatalf("patched = %+v", me)
	}
	var cleared api.Me // fresh value: json.Decode keeps stale fields that the response omits
	e.do(t, "PATCH", "/v1/me", tok, map[string]any{"parallel_lang": "", "sunset_city": ""}, 200, &cleared)
	if cleared.ParallelLang != nil || cleared.SunsetCity != nil || *cleared.DisplayName != "Neema" {
		t.Fatalf("clear = %+v", cleared)
	}
	// Keycloak name changes never overwrite a name the user chose.
	e.do(t, "GET", "/v1/me", tok, nil, 200, &me)
	if *me.DisplayName != "Neema" {
		t.Fatalf("display name overwritten: %v", *me.DisplayName)
	}
}

func TestLikesIdempotentAndCounted(t *testing.T) {
	e := newTestEnv(t)
	_, a := e.newUser(t, tokenOpts{})
	_, b := e.newUser(t, tokenOpts{})
	item := "00000000-0000-4000-8000-000000000004"

	var base api.FeedItemDetail
	e.get(t, "/v1/feed/"+item, 200, &base)
	n0 := *base.LikeCount

	var st api.LikeState
	e.do(t, "POST", "/v1/feed/"+item+"/like", a, nil, 200, &st)
	e.do(t, "POST", "/v1/feed/"+item+"/like", a, nil, 200, &st)
	if !st.Liked || st.LikeCount != n0+1 {
		t.Fatalf("after double like by a: %+v (base %d)", st, n0)
	}
	e.do(t, "POST", "/v1/feed/"+item+"/like", b, nil, 200, &st)
	if st.LikeCount != n0+2 {
		t.Fatalf("after like by b: %+v", st)
	}
	e.do(t, "DELETE", "/v1/feed/"+item+"/like", a, nil, 200, &st)
	e.do(t, "DELETE", "/v1/feed/"+item+"/like", a, nil, 200, &st)
	if st.Liked || st.LikeCount != n0+1 {
		t.Fatalf("after unlike by a: %+v", st)
	}
	var it api.FeedItemDetail
	e.get(t, "/v1/feed/"+item, 200, &it)
	if *it.LikeCount != n0+1 {
		t.Fatalf("feed like_count = %d, want %d", *it.LikeCount, n0+1)
	}
	e.do(t, "POST", "/v1/feed/00000000-0000-4000-8000-000000000007/like", a, nil, 404, nil) // draft
}

func TestSyncLastWriteWins(t *testing.T) {
	e := newTestEnv(t)
	_, tok := e.newUser(t, tokenOpts{})
	id := uuid.New()
	t2 := time.Now().Add(-1 * time.Minute).UTC().Truncate(time.Millisecond)
	t1 := t2.Add(-1 * time.Hour)
	mark := func(note string, at time.Time) map[string]any {
		return map[string]any{"id": id, "kind": "note", "target": "verse", "target_ref": "JHN.3.16", "note": note,
			"created_at": t1, "updated_at": at}
	}

	var res api.SyncResponse
	e.do(t, "POST", "/v1/sync", tok, map[string]any{"marks": []any{mark("newer", t2)}}, 200, &res)
	if len(res.Marks) != 1 || *res.Marks[0].Note != "newer" || res.Cursor == "" {
		t.Fatalf("first sync = %+v", res)
	}
	// An older edit from another device loses.
	e.do(t, "POST", "/v1/sync", tok, map[string]any{"cursor": res.Cursor, "marks": []any{mark("older", t1)}}, 200, &res)
	var full api.SyncResponse
	e.do(t, "POST", "/v1/sync", tok, map[string]any{}, 200, &full)
	if len(full.Marks) != 1 || *full.Marks[0].Note != "newer" {
		t.Fatalf("LWW broken: %+v", full.Marks)
	}
	// A newer edit wins; a soft delete propagates.
	del := mark("deleted", t2.Add(time.Second))
	del["deleted_at"] = t2.Add(time.Second)
	e.do(t, "POST", "/v1/sync", tok, map[string]any{"marks": []any{del}, "progress": []any{
		map[string]any{"target": "egw_paragraph", "target_ref": "1", "position": "SC 9.1", "percent": 12, "updated_at": t2},
	}}, 200, &full)
	if len(full.Marks) != 1 || full.Marks[0].DeletedAt == nil || len(full.Progress) != 1 || *full.Progress[0].Percent != 12 {
		t.Fatalf("after delete: %+v", full)
	}

	// Another user can't read or overwrite this mark.
	_, other := e.newUser(t, tokenOpts{})
	e.do(t, "POST", "/v1/sync", other, map[string]any{"marks": []any{mark("hijack", t2.Add(time.Hour))}}, 200, &res)
	if len(res.Marks) != 0 {
		t.Fatalf("other user sees marks: %+v", res.Marks)
	}
	e.do(t, "POST", "/v1/sync", tok, map[string]any{}, 200, &full)
	if *full.Marks[0].Note != "deleted" {
		t.Fatalf("other user overwrote mark: %+v", full.Marks[0])
	}

	many := make([]any, 501)
	for i := range many {
		many[i] = map[string]any{"id": uuid.New(), "kind": "save", "target": "verse", "target_ref": "JHN.3.16", "created_at": t2, "updated_at": t2}
	}
	e.do(t, "POST", "/v1/sync", tok, map[string]any{"marks": many}, 400, nil)
	e.do(t, "POST", "/v1/sync", tok, map[string]any{"marks": []any{map[string]any{"id": uuid.New(), "kind": "bogus", "target": "verse", "target_ref": "x", "created_at": t2, "updated_at": t2}}}, 400, nil)
	e.do(t, "POST", "/v1/sync", tok, map[string]any{"cursor": "!!"}, 400, nil)
}

// ── Library ─────────────────────────────────────────────

func TestBibleBooksAndChapter(t *testing.T) {
	e := newTestEnv(t)
	var res struct {
		Translation string
		Books       []api.BibleBook
	}
	e.get(t, "/v1/bible/suv/books", 200, &res)
	if res.Translation != "SUV" || len(res.Books) != 66 {
		t.Fatalf("books: %s %d", res.Translation, len(res.Books))
	}
	jhn, gen := res.Books[42], res.Books[0]
	if jhn.Osis != "JHN" || jhn.Name != "Yohana" || jhn.Chapters != 3 || jhn.Testament != api.NT || gen.Chapters != 0 || gen.Name != "Mwanzo" {
		t.Fatalf("JHN=%+v GEN=%+v", jhn, gen)
	}
	e.get(t, "/v1/bible/XXX/books", 404, nil)

	var ch api.BibleChapter
	e.get(t, "/v1/bible/suv/jhn/3?parallel=KJV", 200, &ch)
	if ch.Translation != "SUV" || ch.BookName == nil || *ch.BookName != "Yohana" || len(ch.Verses) != 36 {
		t.Fatalf("chapter = %+v", ch)
	}
	if ch.Verses[17].ParallelText == nil || ch.Verses[18].ParallelText != nil {
		t.Fatal("parallel alignment wrong: 3:18 should have KJV, 3:19 should not")
	}
	e.get(t, "/v1/bible/SUV/JHN/99", 404, nil)
	e.get(t, "/v1/bible/XXX/JHN/3", 404, nil)
	e.get(t, "/v1/bible/SUV/JHN/3?parallel=XXX", 404, nil)
}

func TestEgw(t *testing.T) {
	e := newTestEnv(t)
	var books struct{ Books []api.EgwBook }
	e.get(t, "/v1/egw/books?lang=sw", 200, &books)
	if len(books.Books) != 1 || books.Books[0].BookCode != "SC" || books.Books[0].Chapters != 2 {
		t.Fatalf("books = %+v", books.Books)
	}
	ed := books.Books[0].EditionId
	var ch api.EgwChapter
	e.get(t, fmt.Sprintf("/v1/egw/%d/chapters/1?parallel=en", ed), 200, &ch)
	if len(ch.Paragraphs) != 3 || ch.Paragraphs[0].Refcode != "SC 9.1" || ch.Paragraphs[0].ParallelText == nil || ch.ParallelLang == nil || ch.Chapters != 2 {
		t.Fatalf("chapter = %+v", ch)
	}
	var fr api.EgwChapter
	e.get(t, fmt.Sprintf("/v1/egw/%d/chapters/1?parallel=fr", ed), 200, &fr)
	if fr.ParallelLang != nil || fr.Paragraphs[0].ParallelText != nil {
		t.Fatal("fr has no edition; expected no parallel text")
	}
	e.get(t, fmt.Sprintf("/v1/egw/%d/chapters/9", ed), 404, nil)
	e.get(t, "/v1/egw/999999/chapters/1", 404, nil)
}

func TestBeliefs(t *testing.T) {
	e := newTestEnv(t)
	var list struct{ Beliefs []api.Belief }
	e.get(t, "/v1/beliefs?lang=sw", 200, &list)
	if len(list.Beliefs) != 5 || list.Beliefs[0].N != 1 || list.Beliefs[0].GroupKey != api.God || list.Beliefs[2].GroupKey != api.ChristianLife {
		t.Fatalf("beliefs = %+v", list.Beliefs)
	}
	var b api.Belief
	e.get(t, "/v1/beliefs/10?lang=en", 200, &b)
	if b.Body == nil || b.GroupKey != api.Salvation {
		t.Fatalf("belief 10 = %+v", b)
	}
	e.get(t, "/v1/beliefs/2", 404, nil) // draft
}

func TestHymns(t *testing.T) {
	e := newTestEnv(t)
	var hymnals struct{ Hymnals []api.Hymnal }
	e.get(t, "/v1/hymnals", 200, &hymnals)
	if len(hymnals.Hymnals) < 2 {
		t.Fatalf("hymnals = %+v", hymnals.Hymnals)
	}
	var list struct{ Hymns []api.HymnSummary }
	e.get(t, "/v1/hymnals/nzk/hymns", 200, &list)
	if len(list.Hymns) != 6 || !list.Hymns[0].HasAudio || list.Hymns[1].HasAudio {
		t.Fatalf("hymns = %+v", list.Hymns)
	}
	e.get(t, "/v1/hymnals/NZK/hymns?q=12", 200, &list)
	if len(list.Hymns) != 1 || list.Hymns[0].Number != 12 {
		t.Fatalf("q=12 → %+v", list.Hymns)
	}
	e.get(t, "/v1/hymnals/NZK/hymns?q=wimbo", 200, &list)
	if len(list.Hymns) != 6 {
		t.Fatalf("q=wimbo → %d", len(list.Hymns))
	}
	var h api.Hymn
	e.get(t, "/v1/hymnals/NZK/hymns/1", 200, &h)
	if len(h.Stanzas) != 4 || h.Stanzas[1].Kind != api.HymnStanzaKindRefrain || h.AudioChoir == nil || h.AudioPiano == nil {
		t.Fatalf("hymn = %+v", h)
	}
	e.get(t, "/v1/hymnals/NZK/hymns/999", 404, nil)
	e.get(t, "/v1/hymnals/NOPE/hymns", 404, nil)
}

func TestSabbathSchoolCurrent(t *testing.T) {
	e := newTestEnv(t)
	var w api.SabbathSchoolWeek
	e.get(t, "/v1/sabbath-school/current?lang=sw", 200, &w)
	today := time.Now().In(feedTZ)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	ws := w.WeekStart.Time
	if ws.Weekday() != time.Saturday || today.Before(ws) || !today.Before(ws.AddDate(0, 0, 7)) {
		t.Fatalf("week_start %s does not contain %s", ws.Format(time.DateOnly), today.Format(time.DateOnly))
	}
	if len(w.Days) != 7 || w.Days[6].Date == nil || !w.Days[6].Date.Time.Equal(ws.AddDate(0, 0, 6)) || w.MemoryText == nil {
		t.Fatalf("week = %+v", w)
	}
	e.get(t, "/v1/sabbath-school/current?lang=sw&date=2020-01-01", 404, nil)
}

func TestCourseQuiz(t *testing.T) {
	e := newTestEnv(t)
	var courses struct{ Courses []api.Course }
	e.get(t, "/v1/courses?lang=sw", 200, &courses)
	if len(courses.Courses) != 1 || courses.Courses[0].Lessons != 12 {
		t.Fatalf("courses = %+v", courses.Courses)
	}
	cid := courses.Courses[0].Id
	var l api.CourseLesson
	e.get(t, fmt.Sprintf("/v1/courses/%d/lessons/3", cid), 200, &l)
	if l.Total != 12 || len(l.Questions) != 1 || len(l.Questions[0].Options) != 3 || *l.Questions[0].ExplainRef != "DAN.7.17" {
		t.Fatalf("lesson = %+v", l)
	}
	q := l.Questions[0]
	var right, wrong int
	for _, o := range q.Options {
		if o.IsCorrect {
			right = o.Id
		} else {
			wrong = o.Id
		}
	}
	_, tok := e.newUser(t, tokenOpts{})
	path := fmt.Sprintf("/v1/courses/%d/lessons/3/answers", cid)
	var res struct {
		Correct         bool `json:"correct"`
		CorrectOptionID int  `json:"correct_option_id"`
	}
	e.do(t, "POST", path, tok, map[string]int{"question_id": q.Id, "option_id": wrong}, 200, &res)
	if res.Correct || res.CorrectOptionID != right {
		t.Fatalf("wrong answer → %+v", res)
	}
	e.do(t, "POST", path, tok, map[string]int{"question_id": q.Id, "option_id": right}, 200, &res)
	if !res.Correct {
		t.Fatalf("right answer → %+v", res)
	}
	// Stored once per question (updated in place), visible through sync.
	var s api.SyncResponse
	e.do(t, "POST", "/v1/sync", tok, map[string]any{}, 200, &s)
	if len(s.Answers) != 1 || *s.Answers[0].OptionId != right {
		t.Fatalf("answers = %+v", s.Answers)
	}
	e.do(t, "POST", fmt.Sprintf("/v1/courses/%d/lessons/4/answers", cid), tok, map[string]int{"question_id": q.Id, "option_id": right}, 404, nil)
	e.do(t, "POST", path, tok, map[string]int{"question_id": q.Id, "option_id": 999999}, 404, nil)
	e.get(t, fmt.Sprintf("/v1/courses/%d/lessons/13", cid), 404, nil)
}

func TestSearchGroups(t *testing.T) {
	e := newTestEnv(t)
	groups := func(q string, scope string) map[api.SearchScope][]api.SearchHit {
		t.Helper()
		var r api.SearchResults
		path := "/v1/search?lang=sw&q=" + url.QueryEscape(q)
		if scope != "" {
			path += "&scope=" + scope
		}
		e.get(t, path, 200, &r)
		m := map[api.SearchScope][]api.SearchHit{}
		for _, g := range r.Groups {
			m[g.Scope] = g.Hits
		}
		return m
	}
	if g := groups("yohana", ""); len(g[api.SearchScopeBible]) == 0 || len(g[api.SearchScopeBible]) > searchPerGroup {
		t.Fatalf("yohana → %+v", g)
	}
	if g := groups("Yohana 3:16", ""); len(g[api.SearchScopeBible]) == 0 || g[api.SearchScopeBible][0].TargetRef != "JHN.3.16" {
		t.Fatalf("reference search → %+v", g[api.SearchScopeBible])
	}
	if g := groups("imani", "beliefs"); len(g[api.SearchScopeBeliefs]) == 0 || len(g) != 1 {
		t.Fatalf("imani/beliefs → %+v", g)
	}
	if g := groups("wimbo", ""); len(g[api.SearchScopeHymns]) == 0 || g[api.SearchScopeHymns][0].Target != api.SearchHitTargetHymn {
		t.Fatalf("wimbo → %+v", g)
	}
	if g := groups("njia salama", ""); len(g[api.SearchScopeEgw]) == 0 || g[api.SearchScopeEgw][0].EditionId == nil || g[api.SearchScopeEgw][0].Chapter == nil {
		t.Fatalf("egw → %+v", g)
	}
	if g := groups("danieli", "video"); len(g[api.SearchScopeVideo]) != 1 {
		t.Fatalf("video → %+v", g)
	}
	e.get(t, "/v1/search?q=a", 400, nil)
}

func TestPacksLatestOnly(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	slug := "test-" + uuid.NewString()[:8]
	t.Cleanup(func() { _, _ = e.pool.Exec(ctx, `DELETE FROM packs WHERE slug = $1`, slug) })
	if _, err := e.pool.Exec(ctx, `INSERT INTO packs (slug, lang, version, url, bytes, sha256) VALUES
		($1, 'sw', 1, 'http://test.invalid/packs/v1', 10, repeat('0', 64)),
		($1, 'sw', 2, 'http://test.invalid/packs/v2', 20, repeat('0', 64))`, slug); err != nil {
		t.Fatal(err)
	}
	var res struct{ Packs []api.Pack }
	e.get(t, "/v1/packs?lang=sw", 200, &res)
	found := 0
	for _, p := range res.Packs {
		if p.Slug == slug {
			found++
			if p.Version != 2 || p.Bytes != 20 {
				t.Fatalf("latest = %+v, want v2", p)
			}
		}
	}
	if found != 1 {
		t.Fatalf("slug %s appears %d times in %+v", slug, found, res.Packs)
	}
}
