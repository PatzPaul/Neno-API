package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PatzPaul/Neno-API/internal/api"
)

// Runs against a migrated + seeded database (`make migrate seed`); read-only.
//
//	NENO_TEST_DATABASE_URL=$DATABASE_URL go test ./internal/server/
func newTestServer(t *testing.T) *httptest.Server {
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
	h := api.HandlerWithOptions(api.NewStrictHandler(New(pool), nil), api.StdHTTPServerOptions{BaseRouter: http.NewServeMux()})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func getJSON(t *testing.T, ts *httptest.Server, path string, wantStatus int, out any) {
	t.Helper()
	res, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != wantStatus {
		t.Fatalf("GET %s: status %d, want %d", path, res.StatusCode, wantStatus)
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("GET %s: decode: %v", path, err)
		}
	}
}

func TestFeedPaginationOnlyPublished(t *testing.T) {
	ts := newTestServer(t)
	seen := map[string]bool{}
	path := "/v1/feed?lang=sw&limit=2"
	pages := 0
	for {
		var page api.FeedPage
		getJSON(t, ts, path, 200, &page)
		pages++
		for i, it := range page.Items {
			if seen[it.Id.String()] {
				t.Fatalf("item %s served twice", it.Id)
			}
			seen[it.Id.String()] = true
			if it.Lang != "sw" {
				t.Fatalf("item %s lang %s", it.Id, it.Lang)
			}
			if i > 0 && it.PublishAt.After(page.Items[i-1].PublishAt) {
				t.Fatal("items not newest-first")
			}
		}
		if page.NextCursor == nil {
			break
		}
		path = "/v1/feed?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if len(seen) != 5 || pages != 3 {
		t.Fatalf("got %d items over %d pages, want 5 over 3 (seed has 5 published sw items)", len(seen), pages)
	}
	for _, hidden := range []string{"00000000-0000-4000-8000-000000000007", "00000000-0000-4000-8000-000000000008"} {
		if seen[hidden] {
			t.Fatalf("draft/future item %s was served", hidden)
		}
	}
}

func TestFeedKindsAndFallback(t *testing.T) {
	ts := newTestServer(t)
	var page api.FeedPage
	getJSON(t, ts, "/v1/feed?kinds=prophecy", 200, &page)
	if len(page.Items) != 1 || page.Items[0].Media == nil || page.Items[0].Media.Bytes == nil {
		t.Fatalf("want 1 prophecy item with media bytes, got %+v", page.Items)
	}
	getJSON(t, ts, "/v1/feed?lang=fr", 200, &page)
	if page.Lang != "en" || len(page.Items) == 0 {
		t.Fatalf("fr should fall back to en, got lang=%s n=%d", page.Lang, len(page.Items))
	}
	getJSON(t, ts, "/v1/feed?kinds=nope", 400, nil)
	getJSON(t, ts, "/v1/feed?limit=500", 400, nil)
	getJSON(t, ts, "/v1/feed?cursor=garbage", 400, nil)
}

func TestFeedItemLinks(t *testing.T) {
	ts := newTestServer(t)
	var it api.FeedItemDetail
	getJSON(t, ts, "/v1/feed/00000000-0000-4000-8000-000000000001", 200, &it)
	if len(it.Links) != 3 || it.Links[0].TargetRef != "JHN.3.17" {
		t.Fatalf("links = %+v", it.Links)
	}
	getJSON(t, ts, "/v1/feed/00000000-0000-4000-8000-000000000007", 404, nil) // draft
	getJSON(t, ts, "/v1/feed/not-a-uuid", 400, nil)
}

func TestBibleChapterParallel(t *testing.T) {
	ts := newTestServer(t)
	var ch api.BibleChapter
	getJSON(t, ts, "/v1/bible/suv/jhn/3?parallel=KJV", 200, &ch)
	if ch.Translation != "SUV" || ch.BookName == nil || *ch.BookName != "Yohana" || len(ch.Verses) != 6 {
		t.Fatalf("chapter = %+v", ch)
	}
	if ch.Verses[0].ParallelText == nil || ch.Verses[5].ParallelText != nil {
		t.Fatal("parallel alignment wrong: 3:14 should have KJV, 3:19 should not")
	}
	getJSON(t, ts, "/v1/bible/SUV/JHN/99", 404, nil)
	getJSON(t, ts, "/v1/bible/XXX/JHN/3", 404, nil)
	getJSON(t, ts, "/v1/bible/SUV/JHN/3?parallel=XXX", 404, nil)
}

func TestPacksLatestOnly(t *testing.T) {
	ts := newTestServer(t)
	var res struct{ Packs []api.Pack }
	getJSON(t, ts, "/v1/packs?lang=sw", 200, &res)
	if len(res.Packs) != 2 || res.Packs[0].Slug != "bible-SUV" || res.Packs[0].Version != 2 {
		t.Fatalf("packs = %+v", res.Packs)
	}
}
